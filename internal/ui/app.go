// Package ui builds and runs the System Monitor application window.
//
// The window hosts a persistent shell — a title bar, a vertical tab navigation,
// and a status bar — with one tab per metric area (Overview, CPU, Memory, Disk,
// Network, Processes, Ports, Connections). At this scaffolding stage the tab
// contents are placeholders; real panes are added in later work.
package ui

import "fyne.io/fyne/v2/app"

// Run creates the application, shows the main window, and blocks until it is
// closed.
func Run() {
	a := app.NewWithID("com.josephheinz.systemmonitor")
	a.Settings().SetTheme(newTheme())
	w := a.NewWindow("System Monitor")

	// The shell draws its own chrome flush to the window edges, so suppress
	// Fyne's default padding around window content.
	w.SetPadded(false)
	w.SetContent(buildContent())

	w.Resize(defaultWindowSize())
	w.CenterOnScreen()
	w.ShowAndRun()
}
