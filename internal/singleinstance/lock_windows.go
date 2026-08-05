//go:build windows

package singleinstance

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// mutexPrefix scopes the named mutex to the machine-wide namespace so every
// instance of the app contends on the same lock regardless of the session it
// runs in (the default Local\ namespace is per-session on Windows).
const mutexPrefix = `Global\`

// acquire takes the single-instance lock via a machine-wide named mutex. A
// second CreateMutexW with the same name returns ERROR_ALREADY_EXISTS — the
// signal that a live instance holds the lock. The handle is kept for the
// process lifetime and the OS drops the mutex when the process dies, so
// nothing stale survives a crash.
func acquire(name string) (func(), error) {
	namePtr, err := windows.UTF16PtrFromString(mutexPrefix + name)
	if err != nil {
		return nil, fmt.Errorf("mutex name: %w", err)
	}
	m, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("create mutex: %w", err)
	}
	return func() {
		windows.CloseHandle(m)
	}, nil
}
