package ui

import "fyne.io/fyne/v2"

// defaultWindowSize returns the initial size of the main window. The app is
// designed for a dense, desktop-class layout, so it opens reasonably large.
func defaultWindowSize() fyne.Size {
	return fyne.NewSize(1280, 800)
}
