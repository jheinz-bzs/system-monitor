//go:build !darwin

package update

// resignAfterSwap is a no-op off macOS: Windows and Linux builds aren't
// distributed as signed bundles, so there's no seal to restore after a swap.
func resignAfterSwap(exe string) error {
	return nil
}
