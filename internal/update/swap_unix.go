//go:build !windows

package update

import (
	"fmt"
	"os"
)

// executableMode is the permission set restored on the swapped-in binary, since
// a downloaded file lands without the execute bit.
const executableMode = 0o755

// swap replaces the running executable with newBin (already downloaded and
// verified). Unix lets a running binary be renamed over directly — the kernel
// keeps the open inode alive for the current process — so no park-and-restore
// dance is needed, unlike Windows.
func swap(newBin string) error {
	exe, err := targetExecutable()
	if err != nil {
		return err
	}
	if err := os.Chmod(newBin, executableMode); err != nil {
		return fmt.Errorf("chmod new binary: %w", err)
	}
	if err := os.Rename(newBin, exe); err != nil {
		return fmt.Errorf("install new exe: %w", err)
	}
	return nil
}
