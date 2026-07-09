package ui

// Small control chrome from design-system-05: the interactive segmentedSelect
// (the Processes treemap's CPU/memory sizing toggle) and the toggleChip on/off
// control.

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

// segmentedSelect is an interactive row of labeled segments with the active one
// chipped, reporting the
// chosen index through onChange whenever a *different* segment is tapped. The
// Processes treemap uses it to toggle its sizing metric between CPU and memory.
// Hit-testing maps the tap's X onto a segment via the per-segment right edges
// the renderer records each Layout.
type segmentedSelect struct {
	widget.BaseWidget

	labels   []string
	active   int
	onChange func(index int)
	edges    []float32 // right-edge X of each segment, in widget coords
}

var (
	_ fyne.Tappable      = (*segmentedSelect)(nil)
	_ desktop.Cursorable = (*segmentedSelect)(nil)
)

// newSegmentedSelect builds an interactive segmented control over labels with
// the given segment active. onChange fires with the new index after a tap that
// changes the selection.
func newSegmentedSelect(active int, onChange func(index int), labels ...string) *segmentedSelect {
	s := &segmentedSelect{labels: labels, active: active, onChange: onChange}
	s.ExtendBaseWidget(s)
	return s
}

// Tapped implements fyne.Tappable: select the segment the tap fell in (the last
// one if the tap lands past every recorded edge, e.g. on the right border).
func (s *segmentedSelect) Tapped(ev *fyne.PointEvent) {
	n := len(s.labels)
	if n == 0 {
		return
	}
	for i, edge := range s.edges {
		if ev.Position.X >= edge {
			continue
		}
		s.choose(i)
		return
	}
	s.choose(n - 1)
}

// choose activates segment i, repainting and notifying only on a real change.
func (s *segmentedSelect) choose(i int) {
	if i == s.active {
		return
	}
	s.active = i
	s.Refresh()
	if s.onChange != nil {
		s.onChange(i)
	}
}

// Cursor implements desktop.Cursorable — a pointer, as over the toggle chip.
func (s *segmentedSelect) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (s *segmentedSelect) CreateRenderer() fyne.WidgetRenderer {
	frame := canvas.NewRectangle(palette.Surface2)
	frame.StrokeColor = palette.Border
	frame.StrokeWidth = theme.Size(theme.SizeNameInputBorder)
	frame.CornerRadius = theme.Size(sizeName.PanelRadius)

	chip := canvas.NewRectangle(palette.Surface3)
	chip.CornerRadius = theme.Size(theme.SizeNameInputRadius)

	texts := make([]*canvas.Text, len(s.labels))
	for i, l := range s.labels {
		texts[i] = styledText(l, font.MonoRegular, theme.SizeNameCaptionText, palette.Text3)
	}
	return &segmentedSelectRenderer{ctrl: s, frame: frame, chip: chip, texts: texts}
}

type segmentedSelectRenderer struct {
	ctrl  *segmentedSelect
	frame *canvas.Rectangle
	chip  *canvas.Rectangle
	texts []*canvas.Text
}

// Layout sizes the frame, lays the segments left→right (each as wide as its
// label plus the segment pad), parks the chip behind the active one, and
// records each segment's right edge for hit-testing.
func (r *segmentedSelectRenderer) Layout(size fyne.Size) {
	r.frame.Resize(size)
	r.frame.Move(fyne.NewPos(0, 0))

	r.ctrl.edges = r.ctrl.edges[:0]
	x := float32(0)
	for i, t := range r.texts {
		ts := t.MinSize()
		segW := ts.Width + 2*segItemHPad
		if i == r.ctrl.active {
			r.chip.Resize(fyne.NewSize(segW, size.Height))
			r.chip.Move(fyne.NewPos(x, 0))
		}
		t.Resize(ts)
		t.Move(fyne.NewPos(x+segItemHPad, (size.Height-ts.Height)/2))
		x += segW
		r.ctrl.edges = append(r.ctrl.edges, x)
	}
	r.apply()
}

// apply colors the active label primary, the rest muted.
func (r *segmentedSelectRenderer) apply() {
	for i, t := range r.texts {
		col := palette.Text3
		if i == r.ctrl.active {
			col = palette.Text
		}
		t.Color = col
		t.Refresh()
	}
}

func (r *segmentedSelectRenderer) MinSize() fyne.Size {
	var w, h float32
	for _, t := range r.texts {
		ts := t.MinSize()
		w += ts.Width + 2*segItemHPad
		h = max(h, ts.Height)
	}
	return fyne.NewSize(w, h+2*segItemVPad)
}

func (r *segmentedSelectRenderer) Refresh() {
	r.Layout(r.ctrl.Size())
	canvas.Refresh(r.ctrl)
}

func (r *segmentedSelectRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.frame, r.chip}
	for _, t := range r.texts {
		objs = append(objs, t)
	}
	return objs
}

func (r *segmentedSelectRenderer) Destroy() {}

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

// Session tracking control labels (BZS253-77): the toggle reads "Record" idle
// and "Stop" while a session is active.
const (
	labelRecordStart = "Record"
	labelRecordStop  = "Stop"
)

// recordDotGap is the gap between the record control's state dot and its label,
// matching the status bar's dot-to-label spacing.
const recordDotGap = statusDotGap // 4

// recordControl is the single start/stop toggle for session tracking mode
// (BZS253-77), sitting in the status bar. Idle it shows a muted dot beside
// "Record"; while a session is active the dot turns red and the label reads
// "Stop" — the clear "recording" affordance the card calls for. Tapping invokes
// onToggle; the composition root opens a save dialog and starts the recorder, or
// stops the active session. Its painted state is read from recording() so it
// tracks the session even when start is deferred behind an async save dialog;
// refresh re-syncs it once per poll tick.
type recordControl struct {
	widget.BaseWidget

	onToggle  func()
	recording func() bool

	dot    *canvas.Circle
	dotObj fyne.CanvasObject
	label  *canvas.Text
}

var (
	_ fyne.Tappable      = (*recordControl)(nil)
	_ desktop.Cursorable = (*recordControl)(nil)
)

// newRecordControl builds the toggle. recording reports whether a session is
// active; onToggle is invoked on tap. Both canvas objects are built here (once)
// so refresh can mutate them directly, the same pattern the status bar uses.
func newRecordControl(recording func() bool, onToggle func()) *recordControl {
	c := &recordControl{
		onToggle:  onToggle,
		recording: recording,
		label:     newMeta(labelRecordStart),
	}
	// Reuse the status bar's dot helper: same geometry, and it returns the circle
	// so refresh can recolor it (dry).
	c.dot, c.dotObj = coloredDot(palette.Text3)
	c.ExtendBaseWidget(c)
	c.apply()
	return c
}

// Tapped implements fyne.Tappable.
func (c *recordControl) Tapped(_ *fyne.PointEvent) {
	if c.onToggle != nil {
		c.onToggle()
	}
}

// Cursor implements desktop.Cursorable — a pointer, as over other controls.
func (c *recordControl) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (c *recordControl) CreateRenderer() fyne.WidgetRenderer {
	row := container.New(layout.NewCustomPaddedHBoxLayout(recordDotGap),
		vCenter(c.dotObj), vCenter(c.label))
	return widget.NewSimpleRenderer(row)
}

// apply paints the current session state onto the dot and label.
func (c *recordControl) apply() {
	if c.recording != nil && c.recording() {
		c.dot.FillColor = palette.Red
		c.label.Text = labelRecordStop
		return
	}
	c.dot.FillColor = palette.Text3
	c.label.Text = labelRecordStart
}

// refresh re-reads the session state and repaints, so the affordance tracks a
// start/stop within one poll tick. Touches the canvas — the caller (status bar)
// runs on the UI goroutine.
func (c *recordControl) refresh() {
	c.apply()
	c.dot.Refresh()
	c.label.Refresh()
}
