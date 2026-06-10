package ui

// Design-system panel chrome (design-system-04 "panel" component): a rounded
// surface card with a 34px surface-2 header — uppercase mono title on the
// left, an optional trailing accessory (legend, jump link) on the right —
// above a padded body. Tabs compose their panes from this one component so
// every panel reads identically.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
)

// Panel geometry (design-system-03/-04).
const (
	panelHeaderHeight = 34      // px; panel header band height
	panelHeaderHPad   = spaceLG // 12; header text inset from the panel edges
	panelBodyPad      = spaceLG // 12; body inset from the panel edges
	panelBorderWidth  = 1       // px; hairline card outline
)

// newPanel assembles a titled panel around body. trailing is an optional
// right-aligned header accessory (nil for a plain header). The body is inset
// on all sides; pass the bare content, not pre-padded wrappers.
func newPanel(title string, trailing, body fyne.CanvasObject) fyne.CanvasObject {
	card := canvas.NewRectangle(palette.Surface)
	card.StrokeColor = palette.Border
	card.StrokeWidth = panelBorderWidth
	card.CornerRadius = theme.Size(sizeName.PanelRadius)

	paddedBody := container.New(
		layout.NewCustomPaddedLayout(panelBodyPad, panelBodyPad, panelBodyPad, panelBodyPad), body)
	content := newTightBorder(newPanelHeader(title, trailing), nil, nil, nil, paddedBody)
	// Inset the content by the stroke so the header band doesn't paint over
	// the card's 1px outline.
	inset := container.New(layout.NewCustomPaddedLayout(
		panelBorderWidth, panelBorderWidth, panelBorderWidth, panelBorderWidth), content)
	return container.NewStack(card, inset)
}

// newPanelHeader builds the 34px surface-2 header band: title left, optional
// trailing accessory right, 1px divider underneath.
func newPanelHeader(title string, trailing fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(palette.Surface2)
	bg.SetMinSize(fyne.NewSize(0, panelHeaderHeight))

	row := container.NewHBox(vCenter(newColumnLabel(title)), layout.NewSpacer())
	if trailing != nil {
		row.Add(vCenter(trailing))
	}
	inset := container.New(
		layout.NewCustomPaddedLayout(0, 0, panelHeaderHPad, panelHeaderHPad), row)
	return vStackTight(container.NewStack(bg, inset), hLine())
}

// vCenter centers o vertically within whatever height its parent stretches it
// to (HBox children are stretched to the row height).
func vCenter(o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedVBoxLayout(0),
		layout.NewSpacer(), o, layout.NewSpacer())
}

// Legend geometry (design-system-04 chart legend).
const (
	legendSwatchSize = 9       // px; square series swatch
	legendSwatchGap  = 6       // px; swatch-to-label gap (off-grid)
	legendItemGap    = spaceXL // 16; gap between legend entries
)

// legendEntry is one legend item: a series label and its swatch color.
type legendEntry struct {
	label string
	col   color.Color
}

// newLegend renders chart legend entries for a panel header: a colored swatch
// beside a muted mono label per entry.
func newLegend(entries ...legendEntry) fyne.CanvasObject {
	items := make([]fyne.CanvasObject, 0, len(entries))
	for _, e := range entries {
		swatch := canvas.NewRectangle(e.col)
		swatch.CornerRadius = theme.Size(theme.SizeNameInputRadius)
		sized := container.NewGridWrap(
			fyne.NewSize(legendSwatchSize, legendSwatchSize), swatch)
		items = append(items, container.New(
			layout.NewCustomPaddedHBoxLayout(legendSwatchGap),
			vCenter(sized), vCenter(newStatusText(e.label, status.Neutral))))
	}
	return container.New(layout.NewCustomPaddedHBoxLayout(legendItemGap), items...)
}

// newJumpLink renders the cross-nav link chrome (design-system-05) as static
// accent text. Navigation wiring lands with the target tabs; the chrome exists
// now so panel headers match the wireframes.
func newJumpLink(text string) fyne.CanvasObject {
	return styledText(text, font.MonoRegular, theme.SizeNameCaptionText, palette.Accent)
}
