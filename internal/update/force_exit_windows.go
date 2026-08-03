//go:build windows

package update

import (
	"os"
	"time"
)

// forceExitDelay bounds the old process's teardown after the new binary has
// been spawned. The clean Fyne quit (tray removal, session flush) normally
// finishes in well under a second; if that teardown hangs, a stale instance
// would keep its tray icon alive next to the fresh one (issue #64). The
// fallback below makes sure the process cannot outlive its replacement.
const forceExitDelay = 5 * time.Second

// armForceExit schedules an unconditional os.Exit so a hung update restart
// cannot leave a zombie instance behind in the system tray. It is a no-op when
// the clean shutdown completes first — the process exits and takes the timer
// goroutine down with it.
func armForceExit() {
	go func() {
		time.Sleep(forceExitDelay)
		os.Exit(0)
	}()
}
