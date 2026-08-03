package update

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestIsPackageManagerOwned locks the dpkg-owned detection to the /usr/bin tree
// the .deb installs into (packaging/nfpm.yaml), and nothing else.
func TestIsPackageManagerOwned(t *testing.T) {
	for _, tc := range []struct {
		name string
		exe  string
		want bool
	}{
		{name: "deb install path", exe: "/usr/bin/system-monitor", want: true},
		{name: "path under usr-bin", exe: "/usr/bin/sub/system-monitor", want: true},
		{name: "usr-local-bin not apt-owned", exe: "/usr/local/bin/system-monitor", want: false},
		{name: "user bin", exe: filepath.Join("/home/me/bin/system-monitor"), want: false},
		{name: "opt install", exe: "/opt/system-monitor/bin/system-monitor", want: false},
		{name: "empty", exe: "", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPackageManagerOwned(tc.exe); got != tc.want {
				t.Errorf("isPackageManagerOwned(%q) = %v, want %v", tc.exe, got, tc.want)
			}
		})
	}
}

// TestModeFor exercises the mode classification over synthetic install layouts
// (not the real executable), covering the issue #68 dpkg-owned case.
func TestModeFor(t *testing.T) {
	t.Setenv(envAppImage, "") // each case below decides its own AppImage-ness

	t.Run("deb install defers to apt", func(t *testing.T) {
		if got := modeFor("/usr/bin/system-monitor"); got != ModePackageManager {
			t.Errorf("modeFor(/usr/bin) = %v, want %v", got, ModePackageManager)
		}
	})

	t.Run("writable standalone dir self-updates", func(t *testing.T) {
		exe := filepath.Join(t.TempDir(), "system-monitor")
		if got := modeFor(exe); got != ModeSelf {
			t.Errorf("modeFor(writable) = %v, want %v", got, ModeSelf)
		}
	})

	t.Run("unwritable non-distro dir is unsupported", func(t *testing.T) {
		// A directory that cannot exist (so CreateTemp beside it must fail)
		// models a root-owned, non-apt path like /opt.
		exe := filepath.Join(t.TempDir(), "no-such-dir", "system-monitor")
		if got := modeFor(exe); got != ModeUnsupported {
			t.Errorf("modeFor(unwritable) = %v, want %v", got, ModeUnsupported)
		}
	})

	t.Run("appimage launch always self-updates", func(t *testing.T) {
		t.Setenv(envAppImage, "/home/me/system-monitor.AppImage")
		if got := modeFor("/usr/bin/system-monitor"); got != ModeSelf {
			t.Errorf("modeFor(appimage) = %v, want %v", got, ModeSelf)
		}
	})
}

// TestStartNoOpsForPackageManagerMode: for a dpkg-owned install Start must be a
// no-op even when an update is available — the guard that stops an auto-install
// preference from swapping the file out from under dpkg's database (issue #68).
func TestStartNoOpsForPackageManagerMode(t *testing.T) {
	c := NewController("v1.0.0", ModePackageManager, nil)
	c.set(StatusAvailable, available{version: "v9.9.9", assetURL: "https://example.test/bin"})

	c.Start(context.Background())

	// The guard returns before spawning any work, so the state must not advance.
	if got := c.Snapshot(); got.State != StatusAvailable || got.NewVersion != "v9.9.9" {
		t.Fatalf("Start advanced package-manager state to %+v", got)
	}
}

// TestStartProceedsForSelfMode is the positive control: the same Start that
// no-ops above really does run the install for a ModeSelf controller. The
// injected assetURL is malformed, so the download fails immediately (no
// network) and the controller lands on StatusFailed.
func TestStartProceedsForSelfMode(t *testing.T) {
	c := NewController("v1.0.0", ModeSelf, nil)
	c.set(StatusAvailable, available{version: "v9.9.9", assetURL: "://bad"})

	c.Start(context.Background())
	deadline := time.Now().Add(time.Second)
	for c.Snapshot().State != StatusFailed {
		if time.Now().After(deadline) {
			t.Fatal("self-update Start never left StatusAvailable")
		}
		time.Sleep(time.Millisecond)
	}
}
