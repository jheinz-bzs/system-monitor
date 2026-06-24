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

	// oldSuffix parks the previous binary during a Windows swap (see swap_windows.go);
	// it is deleted on the next launch by CleanupOld.
	oldSuffix = ".old"
)

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

// assetName is this platform's release binary name, e.g. system-monitor-linux-amd64
// or system-monitor-windows-amd64.exe. It must match the names produced by the
// release workflow's build matrix.
func assetName() string {
	name := assetPrefix + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += windowsExt
	}
	return name
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
