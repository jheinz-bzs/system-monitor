package singleinstance

import (
	"errors"
	"testing"
)

// setup points the user cache dir at a temp dir so the tests exercise the lock
// mechanism without dropping lockfiles into the developer's real cache. XDG
// covers Linux; HOME the macOS fallback (the Unix UserCacheDir variants).
func setup(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
}

// TestAcquireSucceeds checks the first Acquire for a name succeeds and returns
// a non-nil release.
func TestAcquireSucceeds(t *testing.T) {
	setup(t)
	release, err := Acquire("test-first")
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	if release == nil {
		t.Fatal("first Acquire returned nil release")
	}
	release()
}

// TestSecondAcquireFailsWhileHeld simulates the "second instance": a second
// Acquire on the same name, via a separate open of the lockfile (or mutex),
// conflicts and reports ErrAlreadyRunning even from the same process.
func TestSecondAcquireFailsWhileHeld(t *testing.T) {
	setup(t)
	release, err := Acquire("test-held")
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	defer release()

	if _, err := Acquire("test-held"); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Acquire = %v, want ErrAlreadyRunning", err)
	}
}

// TestAcquireAgainAfterRelease checks the lock is reusable once released, so a
// fresh launch after a clean exit always starts.
func TestAcquireAgainAfterRelease(t *testing.T) {
	setup(t)
	const name = "test-reacquire"
	release, err := Acquire(name)
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	release()

	release, err = Acquire(name)
	if err != nil {
		t.Fatalf("Acquire after release failed: %v", err)
	}
	release()
}
