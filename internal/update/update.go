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
// affordance. A package-manager-owned install (the .deb under /usr/bin) never
// self-updates — InstallMode() classifies it ModePackageManager, Start is a
// no-op, and the UI defers the upgrade to apt instead (issue #68).
package update

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// GitHub coordinates of the published releases. The module path is a vanity
// import (github.com/josephheinz/...); releases are actually cut from this repo,
// so the API queries target it directly.
const (
	owner = "josephheinz"
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
	// appImageExt is the asset suffix for an AppImage launch: a single
	// self-contained file, so it swaps like a bare binary. A bare Linux binary
	// resolves the extensionless asset instead and self-updates when its install
	// dir is writable; the .deb resolves the same extensionless asset for
	// availability detection but never installs it (Mode / ADR-011).
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

// Mode is how a newer release is delivered for this running instance: in-place
// self-update, or deferred to the distro package manager.
type Mode int

const (
	// ModeSelf self-updates: download the verified asset and swap the binary.
	ModeSelf Mode = iota
	// ModePackageManager defers upgrades to apt: the install is dpkg-owned, so
	// the UI surfaces the apt upgrade command instead of self-updating (#68).
	ModePackageManager
	// ModeUnsupported has no in-app update path: self-swap would hit a
	// permission wall (a root-owned, non-distro dir) and apt doesn't manage it.
	ModeUnsupported
)

// available is a resolved, installable release: where to get the platform binary
// and the expected hash to verify it against.
type available struct {
	version  string
	assetURL string
	checksum string // hex-encoded SHA-256
}

// assetName is this platform's self-update asset name — the file the downloader
// resolves in the release. It must match the names the release workflow produces:
// Windows .exe, macOS bare; Linux picks the .AppImage when launched as one and
// the bare binary otherwise (a .deb launch also resolves the bare binary, so
// availability detection works even though it can never install it — Mode).
func assetName() string {
	name := assetPrefix + runtime.GOOS + "-" + runtime.GOARCH
	switch {
	case runtime.GOOS == goosWindows:
		name += windowsExt
	case isAppImage():
		name += appImageExt
	}
	return name
}

// isAppImage reports whether this process was launched from an AppImage.
func isAppImage() bool {
	return os.Getenv(envAppImage) != ""
}

// debInstallRoot is the prefix of dpkg-owned install paths. The .deb installs to
// /usr/bin/system-monitor (packaging/nfpm.yaml), and the apt repo serves the same
// package, so any launch from under /usr/bin is apt-managed. Path-based rather
// than `dpkg -S <exe>` because the probe needs no subprocess, works on any distro
// (a .deb only exists on Debian-family), and matches exactly where the .deb
// lands; a binary hand-placed in another root-owned dir (/usr/local/bin, /opt)
// keeps today's no-affordance behavior (ADR-011).
const debInstallRoot = "/usr/bin/"

// isPackageManagerOwned reports whether exe lives under the distro package
// manager's install root — the /usr/bin tree the .deb owns.
func isPackageManagerOwned(exe string) bool {
	return strings.HasPrefix(exe, debInstallRoot)
}

// InstallMode reports how a newer release is delivered for this running instance:
// Windows, macOS, and AppImage launches always self-update; a bare Linux binary
// self-updates when its install dir is writable, defers to apt when it is
// dpkg-owned (the .deb's /usr/bin), and has no in-app update path otherwise (a
// root-owned non-distro dir). The classification is path/OS/env based — no
// network — and is read once at wiring time.
func InstallMode() Mode {
	exe, err := os.Executable()
	if err != nil {
		return ModeUnsupported
	}
	return modeFor(exe)
}

// modeFor classifies the delivery mode for a process running exe (which
// InstallMode resolves from os.Executable). The path is a parameter so the
// branches are unit-testable without a real install.
func modeFor(exe string) Mode {
	if runtime.GOOS != goosLinux || isAppImage() {
		return ModeSelf
	}
	switch {
	// The dpkg-owned check runs before the writability probe so a .deb — even
	// run as root, where the probe would succeed — is never eligible to have its
	// file swapped out from under dpkg's database (issue #68).
	case isPackageManagerOwned(exe):
		return ModePackageManager
	case canReplace(exe):
		return ModeSelf
	}
	return ModeUnsupported
}

// Supported reports whether an in-app update affordance applies to this running
// instance. It is true for both self-updating installs (Windows, macOS,
// AppImage, writable bare Linux binary) and dpkg-owned installs that defer to
// apt; only an install with no update path at all (a root-owned non-distro dir)
// is unsupported. The UI wires the controller — and thus the banner/status-bar
// affordance — whenever this is true (ADR-011).
func Supported() bool {
	return InstallMode() != ModeUnsupported
}

// canReplace reports whether a temp file can be created beside exe — the write
// permission download+rename need. It performs the exact operation the installer
// does, so a false answer predicts a permission-wall install failure.
func canReplace(exe string) bool {
	f, err := os.CreateTemp(filepath.Dir(exe), assetPrefix+"probe-*")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
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
