//go:build unix

package singleinstance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockSuffix names the lockfile inside the cache/temp dir; lockFilePerm keeps
// it private to the owning user.
const (
	lockSuffix   = ".lock"
	lockFilePerm = 0o600
)

// lockfilePath resolves where the lockfile lives: the user cache dir
// (e.g. $XDG_CACHE_HOME or ~/.cache), or the system temp dir when that is
// unavailable. The file is never deleted — the kernel releases the flock when
// the process dies (even a crash), and deleting the file would race a second
// instance locking a freshly created one while the first still runs.
func lockfilePath(name string) string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, name+lockSuffix)
}

// acquire takes the single-instance flock(2) lock on a lockfile keyed by name.
// flock is advisory and scoped to the open file description, so a second open
// of the same file — in another process or this one — conflicts and fails
// immediately with ErrAlreadyRunning. The lock is released by closing the fd
// (release) or by the kernel on process death, which is why the lockfile
// itself must never be removed.
func acquire(name string) (func(), error) {
	path := lockfilePath(name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, lockFilePerm)
	if err != nil {
		return nil, fmt.Errorf("open lockfile %q: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("flock %q: %w", path, err)
	}
	return func() {
		// Closing the fd drops the flock; the kernel also releases it on
		// process death, so a stale lockfile is harmless — never delete it
		// (deleting would race a second instance locking a fresh file).
		_ = f.Close()
	}, nil
}
