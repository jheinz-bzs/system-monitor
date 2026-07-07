// Package update implements opt-in self-update from GitHub Releases (BZS253-71).
//
// It is deliberately Fyne-free: the UI consumes it through the Controller seam
// (Snapshot/Check/Start), so this package depends on neither the UI nor the
// monitor layer — only the standard library. The flow is: query the repo's
// latest release, compare its semver tag against the build-time version, and —
// only on explicit user confirmation — download the platform asset, verify its
// SHA-256 against the release's checksums.txt, swap it in, and restart.
//
// No silent background replacement: Check only updates observable state; Start
// (the install) runs solely when the user acts on the "update available"
// affordance.
package update

import (
	"os"
	"runtime"
	"time"
)

// GitHub coordinates of the published releases. The module path is a vanity
// import (github.com/josephheinz/...); releases are actually cut from this repo,
// so the API queries target it directly.
const (
	owner = "jheinz-bzs"
	repo  = "system-monitor"

	latestReleaseURL = "https://api.github.com/repos/" + owner + "/" + repo + "/releases/latest"

	// GitHub's API rejects requests without a User-Agent.
	userAgent = "system-monitor-updater"
)

// Release asset naming — the contract with .github/workflows/release.yml. The
// downloader resolves its platform's binary by building this exact name from the
// running GOOS/GOARCH, and reads its hash from the release's checksums file.
const (
	assetPrefix    = "system-monitor-"
	checksumsAsset = "checksums.txt"
	windowsExt     = ".exe"
	// appImageExt is the Linux self-update asset suffix: the AppImage is a single
	// self-contained file, so it swaps like a bare binary. The .deb is install-only
	// (apt updates it), and a bare Linux binary doesn't self-update — only the
	// AppImage does (see Supported / ADR-011).
	appImageExt = ".AppImage"

	// oldSuffix parks the previous binary during a Windows swap (see swap_windows.go);
	// it is deleted on the next launch by CleanupOld.
	oldSuffix = ".old"
)

// GOOS values the updater special-cases (extracted so the asset/swap logic reads
// these identifiers, not bare runtime-string literals).
const (
	goosWindows = "windows"
	goosLinux   = "linux"
)

// envAppImage is the env var the AppImage runtime sets to the path of the running
// .AppImage file. Its presence both identifies an AppImage launch and gives the
// swap/restart their target (the inner os.Executable points into a read-only mount).
const envAppImage = "APPIMAGE"

// httpTimeout bounds both the release check and the asset download so a stalled
// network degrades to a logged no-op rather than hanging a goroutine forever.
const httpTimeout = 30 * time.Second

// Status is the self-update state surfaced to the UI status bar.
type Status int

const (
	StatusIdle        Status = iota // no newer release, or check not yet run / failed
	StatusChecking                  // querying the releases API
	StatusAvailable                 // a strictly-newer release is ready to install
	StatusDownloading               // fetching + hashing the asset
	StatusInstalling                // verified; swapping the binary
	StatusFailed                    // an install attempt failed (logged; no retry loop)
)

// Snapshot is an immutable view of the controller's state for the UI to render.
type Snapshot struct {
	State      Status
	NewVersion string // the available release tag, set once State reaches StatusAvailable
}

// available is a resolved, installable release: where to get the platform binary
// and the expected hash to verify it against.
type available struct {
	version  string
	assetURL string
	checksum string // hex-encoded SHA-256
}

// assetName is this platform's self-update asset name — the file the downloader
// resolves in the release. It must match the names the release workflow produces:
// Windows .exe, the Linux .AppImage (not the bare binary or .deb). Releases ship
// no macOS asset (build-from-source only), so on darwin the lookup misses and
// checkLatest treats it as a normal no-update.
func assetName() string {
	name := assetPrefix + runtime.GOOS + "-" + runtime.GOARCH
	switch runtime.GOOS {
	case goosWindows:
		name += windowsExt
	case goosLinux:
		name += appImageExt
	}
	return name
}

// Supported reports whether in-app self-update applies to this running instance.
// Windows always self-updates; on Linux only an AppImage launch does — a bare
// binary or a .deb install manages its own updates (apt / re-download), so the
// updater isn't wired and no "update failed" noise is shown there. macOS stays
// true but no-ops: releases carry no darwin asset, so the check finds nothing.
func Supported() bool {
	if runtime.GOOS == goosLinux {
		return os.Getenv(envAppImage) != ""
	}
	return true
}

// targetExecutable is the path self-update replaces and re-launches: the AppImage
// file when running as one ($APPIMAGE), otherwise the running executable. Inside
// an AppImage, os.Executable points into a read-only mount, so the env var is the
// only correct swap target.
func targetExecutable() (string, error) {
	if p := os.Getenv(envAppImage); p != "" {
		return p, nil
	}
	return os.Executable()
}

// CleanupOld removes the parked previous binary left by a Windows swap. It runs
// best-effort at startup; on other platforms (and a clean launch) there is no
// such file and it is a no-op.
func CleanupOld() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.Remove(exe + oldSuffix)
}
