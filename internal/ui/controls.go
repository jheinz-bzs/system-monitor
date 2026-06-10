package ui

// Small control chrome from design-system-05: the segmented unit toggle the
// CPU page header shows (% / GHz / load) and the toggleChip on/off control.
// The segmented control is rendered as static, non-interactive chrome: only
// the "%" series is collected today, so it exists to match the wireframe and
// gains behavior when the other unit series do.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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

// toggleSwatchOffAlpha dims a toggleChip's series swatch while the chip is off
// (~35% opacity): the hue stays identifiable but clearly reads as inactive.
const toggleSwatchOffAlpha = 0x59

// toggleChip is a tappable on/off control chip: a series swatch beside a mono
// label inside pill chrome, the interactive sibling of a legend entry
// (panel.go). On shows the surface-3 chip fill of an active segment; off
// empties the fill and mutes the swatch and label. Tapping flips the state,
// repaints, and reports the new state through onChange.
type toggleChip struct {
	widget.BaseWidget

	label    string
	swatch   color.NRGBA
	on       bool
	hovered  bool
	onChange func(on bool)
}

// newToggleChip builds a chip labeled label with the given swatch hue,
// starting in the on state given. onChange fires after every tap with the new
// state.
func newToggleChip(label string, swatch color.NRGBA, on bool, onChange func(on bool)) *toggleChip {
	t := &toggleChip{label: label, swatch: swatch, on: on, onChange: onChange}
	t.ExtendBaseWidget(t)
	return t
}

// Tapped implements fyne.Tappable.
func (t *toggleChip) Tapped(_ *fyne.PointEvent) {
	t.on = !t.on
	t.Refresh()
	if t.onChange != nil {
		t.onChange(t.on)
	}
}

// MouseIn / MouseMoved / MouseOut implement desktop.Hoverable.
func (t *toggleChip) MouseIn(_ *desktop.MouseEvent)    { t.hovered = true; t.Refresh() }
func (t *toggleChip) MouseMoved(_ *desktop.MouseEvent) {}
func (t *toggleChip) MouseOut()                        { t.hovered = false; t.Refresh() }

// Cursor implements desktop.Cursorable — a pointer, as over nav items.
func (t *toggleChip) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (t *toggleChip) CreateRenderer() fyne.WidgetRenderer {
	frame := canvas.NewRectangle(color.Transparent)
	frame.StrokeWidth = theme.Size(theme.SizeNameInputBorder)
	frame.CornerRadius = theme.Size(theme.SizeNameInputRadius)

	swatch := canvas.NewRectangle(t.swatch)
	swatch.CornerRadius = theme.Size(theme.SizeNameInputRadius)

	r := &toggleChipRenderer{
		chip:   t,
		frame:  frame,
		swatch: swatch,
		label:  newStatusText(t.label, status.Neutral),
	}
	r.apply()
	return r
}

type toggleChipRenderer struct {
	chip   *toggleChip
	frame  *canvas.Rectangle
	swatch *canvas.Rectangle
	label  *canvas.Text
}

// apply sets the per-state colors: chip fill and full-strength colors when on,
// hollow frame with muted swatch/label when off, border-strong outline on
// hover in either state.
func (r *toggleChipRenderer) apply() {
	r.frame.StrokeColor = palette.Border
	if r.chip.hovered {
		r.frame.StrokeColor = palette.BorderStrong
	}
	if r.chip.on {
		r.frame.FillColor = palette.Surface3
		r.swatch.FillColor = r.chip.swatch
		r.label.Color = palette.Text2
		return
	}
	r.frame.FillColor = color.Transparent
	dimmed := r.chip.swatch
	dimmed.A = toggleSwatchOffAlpha
	r.swatch.FillColor = dimmed
	r.label.Color = palette.Text3
}

// Layout places the swatch and label like a legend entry, inset by the pill
// pads, both vertically centered in the chip.
func (r *toggleChipRenderer) Layout(size fyne.Size) {
	r.frame.Resize(size)
	r.frame.Move(fyne.NewPos(0, 0))

	r.swatch.Resize(fyne.NewSize(legendSwatchSize, legendSwatchSize))
	r.swatch.Move(fyne.NewPos(pillHPad, (size.Height-legendSwatchSize)/2))

	lbl := r.label.MinSize()
	r.label.Resize(lbl)
	r.label.Move(fyne.NewPos(
		pillHPad+legendSwatchSize+legendSwatchGap, (size.Height-lbl.Height)/2))
}

func (r *toggleChipRenderer) MinSize() fyne.Size {
	lbl := r.label.MinSize()
	w := pillHPad + legendSwatchSize + legendSwatchGap + lbl.Width + pillHPad
	return fyne.NewSize(w, lbl.Height+2*pillVPad)
}

func (r *toggleChipRenderer) Refresh() {
	r.apply()
	r.frame.Refresh()
	r.swatch.Refresh()
	r.label.Refresh()
}

func (r *toggleChipRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.frame, r.swatch, r.label}
}

func (r *toggleChipRenderer) Destroy() {}
