//go:build !windows

package update

// armForceExit is a no-op off Windows, where the Fyne quit path reliably
// terminates the process after an update restart.
func armForceExit() {}
