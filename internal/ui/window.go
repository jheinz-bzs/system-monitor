package ui

import "fyne.io/fyne/v2"

// defaultWindowSize returns the initial size of the main window. The app is
// designed for a dense, desktop-class layout, so it opens reasonably large.
// Dimensions are expressed on the spacing scale (both already on-grid).
func defaultWindowSize() fyne.Size {
	return fyne.NewSize(320*spaceSM, 200*spaceSM) // 1280 x 800
}
