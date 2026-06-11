package ui

// Small control chrome from design-system-05. Currently only the segmented
// unit toggle the CPU page header shows (% / GHz / load). It is rendered as
// static, non-interactive chrome: only the "%" series is collected today, so
// the control exists to match the wireframe and gains behavior when the other
// unit series do.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
)

// Segmented-control geometry. The item pad is off-grid: the wireframe's 26px
// item height minus the 16px label line leaves 5px above and below.
const (
	segItemHPad = spaceLG // 12; label inset within a segment
	segItemVPad = 5       // px; vertical pad producing the 26px item height
)

// newSegmented renders a segmented control with the active segment
// highlighted (surface-3 chip, primary text) and the rest muted.
func newSegmented(active int, labels ...string) fyne.CanvasObject {
	frame := canvas.NewRectangle(palette.Surface2)
	frame.StrokeColor = palette.Border
	frame.StrokeWidth = theme.Size(theme.SizeNameInputBorder)
	frame.CornerRadius = theme.Size(sizeName.PanelRadius)

	row := container.New(layout.NewCustomPaddedHBoxLayout(0))
	for i, l := range labels {
		row.Add(newSegment(l, i == active))
	}
	return container.NewStack(frame, row)
}

// newSegment builds one segment: padded mono caption text, with a surface-3
// chip behind the active one.
func newSegment(label string, isActive bool) fyne.CanvasObject {
	col := palette.Text3
	if isActive {
		col = palette.Text
	}
	text := styledText(label, font.MonoRegular, theme.SizeNameCaptionText, col)
	padded := container.New(
		layout.NewCustomPaddedLayout(segItemVPad, segItemVPad, segItemHPad, segItemHPad), text)
	if !isActive {
		return padded
	}
	chip := canvas.NewRectangle(palette.Surface3)
	chip.CornerRadius = theme.Size(theme.SizeNameInputRadius)
	return container.NewStack(chip, padded)
}
