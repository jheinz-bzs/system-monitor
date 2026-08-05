//go:build !unix && !windows

package singleinstance

// acquire is a no-op on platforms without a native single-instance primitive:
// the lock is always acquired so the app runs, and release does nothing. This
// keeps the package building on every GOOS; only the Unix and Windows builds
// actually enforce single-instance.
func acquire(_ string) (func(), error) {
	return func() {}, nil
}
