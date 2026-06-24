//go:build windows

package update

import (
	"fmt"
	"os"
)

// swap replaces the running executable with newBin (already downloaded and
// verified) using the Windows-safe rename dance: a locked, running .exe cannot be
// deleted or overwritten, but it CAN be renamed. So the current binary is parked
// as <exe>.old and the new one takes its place; the parked copy is removed on the
// next launch by CleanupOld. On failure the park is rolled back so the install
// directory is never left without a runnable binary.
func swap(newBin string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	old := exe + oldSuffix
	_ = os.Remove(old) // clear any stale park slot from a prior update

	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("park current exe: %w", err)
	}
	if err := os.Rename(newBin, exe); err != nil {
		_ = os.Rename(old, exe) // best-effort rollback
		return fmt.Errorf("install new exe: %w", err)
	}
	return nil
}
