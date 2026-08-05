//go:build unix

package singleinstance

import (
	"os"
	"testing"
)

// TestReleaseDoesNotDeleteLockfile guards the design decision that the lockfile
// is left in place after release: the kernel releases the flock when the
// process dies, so the file carries no stale state — and deleting it would race
// a second instance locking a freshly created file while the first still runs.
func TestReleaseDoesNotDeleteLockfile(t *testing.T) {
	setup(t)
	name := "test-no-delete"
	release, err := Acquire(name)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	release()

	if _, err := os.Stat(lockfilePath(name)); err != nil {
		t.Fatalf("lockfile removed on release: %v", err)
	}
}
