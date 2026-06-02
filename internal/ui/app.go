// Package ui builds and runs the System Monitor application window.
//
// At this scaffolding stage it opens a single empty window. Future work will
// add the eight tabs (Overview, CPU, Memory, Disk, Network, Processes, Ports,
// Connections) described in the design doc.
package ui

import (
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

// Run creates the application, shows the main window, and blocks until it is
// closed.
func Run() {
	a := app.NewWithID("com.josephheinz.systemmonitor")
	a.Settings().SetTheme(newTheme())
	w := a.NewWindow("System Monitor")

	// Placeholder content until the tabbed layout is built.
	title := newHeading("System Monitor — scaffold")

	w.SetContent(container.NewCenter(
		title,
	))

	w.Resize(defaultWindowSize())
	w.CenterOnScreen()
	w.ShowAndRun()
}
