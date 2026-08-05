package ui

import (
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
)

// alreadyRunningTitle and alreadyRunningMessage are the native error popup a
// second launch shows before refusing to start (issue #96). The message
// reuses appName so the app's name stays single-sourced.
const (
	alreadyRunningTitle   = "Already running"
	alreadyRunningMessage = appName + " is already running."
)

// showAlreadyRunningDialog is the losing instance's whole lifecycle (issue
// #96): a minimal Fyne app exists only to render the native error popup, then
// the process exits. It must not look like a no-op — the user needs to know why
// the app "didn't open". The dialog's OK dismisses the dialog, whose SetOnClosed
// closes the window; the window close ends ShowAndRun's run loop, returning
// control to Run's os.Exit(1). The normal window, collectors, and tray are
// never started in this process.
func showAlreadyRunningDialog() {
	a := app.NewWithID(appID)
	w := a.NewWindow(appName)
	d := dialog.NewInformation(alreadyRunningTitle, alreadyRunningMessage, w)
	d.SetOnClosed(func() { w.Close() })
	d.Show()
	w.ShowAndRun()
}
