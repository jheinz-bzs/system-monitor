// Package singleinstance enforces one running instance per app ID (issue #96).
//
// It is deliberately Fyne-free so it unit-tests natively and cross-compiles
// cleanly: the UI calls Acquire at the top of Run and treats a non-nil
// ErrAlreadyRunning as "another instance is running — refuse to start". The
// lock is held for the process lifetime (the caller defers release) and is
// auto-released by the OS if the process dies, even a crash, so a stale lock
// can never wedge a later launch.
//
// The mechanism is per-platform via build tags: flock(2) on a lockfile on Unix
// (lock_unix.go), a named mutex on Windows (lock_windows.go), and a no-op
// fallback elsewhere (lock_other.go) so the package still builds on every GOOS.
package singleinstance

import "errors"

// ErrAlreadyRunning is returned by Acquire when a live instance already holds
// the lock. Callers distinguish it from a mechanism failure with errors.Is: a
// mechanism failure means only that single-instance protection is unavailable,
// not that an instance is running.
var ErrAlreadyRunning = errors.New("single instance already running")

// Acquire attempts to take the exclusive single-instance lock keyed by name
// (the app ID). On success it returns a release func that drops the lock; the
// caller must defer it so the lock is held for the whole session. A non-nil
// error means the lock could not be taken: errors.Is(err, ErrAlreadyRunning)
// reports a live instance, any other error a mechanism failure (e.g. no usable
// lockfile location).
func Acquire(name string) (release func(), err error) {
	return acquire(name)
}
