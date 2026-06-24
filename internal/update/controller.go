package update

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Controller is the UI-facing seam for self-update. It holds the update state
// machine and runs the network/disk work off the UI goroutine. The UI reads
// Snapshot each poll tick and calls Start when the user confirms an update; it
// never touches the lower-level check/download/swap functions directly.
//
// Concurrency: state is guarded by mu. Check and Start each run their slow work
// on their own goroutine, so neither blocks the caller (window show / a tap).
type Controller struct {
	current string       // build-time version, e.g. "v1.2.0"
	client  *http.Client // injectable for tests
	quit    func()       // ask the app to exit so the freshly-spawned process takes over

	mu     sync.Mutex
	status Status
	avail  available
}

// NewController returns a controller comparing against current. quit is invoked
// after the new binary has been spawned, to exit this process (the UI wires it
// to a clean Fyne shutdown); a nil quit falls back to os.Exit.
func NewController(current string, quit func()) *Controller {
	return &Controller{
		current: current,
		client:  &http.Client{Timeout: httpTimeout},
		quit:    quit,
		status:  StatusIdle,
	}
}

// IsRelease reports whether version is a real release the updater can compare
// against (false for the "dev" build), so the caller can skip wiring the
// controller entirely on a dev build.
func IsRelease(version string) bool { return isReleaseVersion(version) }

// Snapshot returns the current state for the UI to render.
func (c *Controller) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Snapshot{State: c.status, NewVersion: c.avail.version}
}

// Check queries the latest release and records whether a newer one is available.
// It is failure-tolerant: any error (offline, API hiccup, parse) is logged and
// leaves the state idle — never a crash, never a nag. Run it on a goroutine.
func (c *Controller) Check(ctx context.Context) {
	c.set(StatusChecking, available{})
	avail, ok, err := checkLatest(ctx, c.client, c.current)
	if err != nil {
		log.Printf("update: check failed: %v", err)
		c.set(StatusIdle, available{})
		return
	}
	if !ok {
		c.set(StatusIdle, available{})
		return
	}
	c.set(StatusAvailable, avail)
}

// Start installs the available update: download, verify, swap, then spawn the new
// binary and quit. It is a no-op unless an update is currently available, so a
// stray tap can't trigger an install mid-flight. The work runs on a goroutine so
// the calling UI tap returns immediately.
func (c *Controller) Start(ctx context.Context) {
	c.mu.Lock()
	if c.status != StatusAvailable {
		c.mu.Unlock()
		return
	}
	avail := c.avail
	c.mu.Unlock()

	go func() {
		if err := c.install(ctx, avail); err != nil {
			log.Printf("update: install failed: %v", err)
			c.set(StatusFailed, avail)
			return
		}
		if err := restart(); err != nil {
			log.Printf("update: restart failed: %v", err)
			c.set(StatusFailed, avail)
			return
		}
		c.shutdown()
	}()
}

// install downloads the verified asset next to the current binary and swaps it
// in, advancing the observable state through the download/install phases.
func (c *Controller) install(ctx context.Context, a available) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	c.set(StatusDownloading, a)
	tmp, err := downloadVerified(ctx, c.client, a.assetURL, a.checksum, filepath.Dir(exe))
	if err != nil {
		return err
	}
	c.set(StatusInstalling, a)
	return swap(tmp)
}

func (c *Controller) set(s Status, a available) {
	c.mu.Lock()
	c.status = s
	c.avail = a
	c.mu.Unlock()
}

func (c *Controller) shutdown() {
	if c.quit != nil {
		c.quit()
		return
	}
	os.Exit(0)
}

// restart spawns a fresh instance of the (now swapped-in) executable with the
// same arguments, so the user lands on the new version once this process exits.
// os.StartProcess is used rather than exec-in-place because Windows has no exec.
func restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	proc, err := os.StartProcess(exe, os.Args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return err
	}
	return proc.Release()
}
