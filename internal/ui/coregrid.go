package ui

// Per-core utilisation grid (the CPU tab wireframe's "Per-core" panel body):
// one cell per logical core, laid out in a fixed-column grid. Each cell is a
// rounded surface-2 card showing the core label ("C0") on the left, its
// current utilisation percent on the right, and a 5px horizontal bar
// underneath — surface-3 track, filled in the core's categorical hue (c1–c8,
// wrapping) by the live fraction.
//
// Like lineChart, the widget is fed by series.Source values and re-reads each
// source on Refresh, showing the newest sample. It is a custom widget with a
// pooled renderer (the lineChart / dataTable strategy): every canvas object is
// allocated once at construction — the core count is fixed for the process
// lifetime — and arrange() only moves/resizes them.

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/josephheinz/system-monitor/internal/series"
)

// Core-grid geometry, from the CPU tab wireframe's per-core pane. The gaps are
// off-grid wireframe values, so they keep their own literal-px consts rather
// than being snapped to the spacing scale.
const (
	coreGridCols     = 4       // cells per row (wireframe grid-template-columns)
	coreGridGap      = 6       // px; gap between cells, both axes
	coreHeaderBarGap = 3       // px; gap between a cell's header row and its bar
	coreBarHeight    = 5       // px; bar track/fill height
	coreCellPad      = spaceMD // 8; content inset from the cell card's edges
	coreCellMinWidth = 96      // px; per-cell floor so bars stay readable
)

// coreLabelPrefix forms the cell labels: "C" + core index ("C0", "C1", …).
const coreLabelPrefix = "C"

// coreGrid is the per-core bar grid widget. The zero value is not usable;
// build one with newCoreGrid.
type coreGrid struct {
	widget.BaseWidget

	cores []series.Source
}

// newCoreGrid returns a grid with one cell per source, in core order. Sources
// are re-read on every Refresh; the newest sample drives each cell.
func newCoreGrid(cores []series.Source) *coreGrid {
	g := &coreGrid{cores: cores}
	g.ExtendBaseWidget(g)
	return g
}

// coreCell is one core's pooled canvas objects: the rounded card behind the
// cell, the muted index label, the live percent readout, and the bar
// track/fill pair.
type coreCell struct {
	card  *canvas.Rectangle
	label *canvas.Text
	value *canvas.Text
	track *canvas.Rectangle
	fill  *canvas.Rectangle
}

// newCoreCell builds the cell for core i. The fill takes the core's
// categorical hue — c1 for core 0, wrapping after c8 — matching the wireframe
// (the chart's secondary lines start at c2 only because the overall line owns
// the accent; the bars have no such headline to stay distinct from).
func newCoreCell(i int) *coreCell {
	card := canvas.NewRectangle(palette.Surface2)
	card.StrokeColor = palette.Border
	card.StrokeWidth = panelBorderWidth
	card.CornerRadius = theme.Size(sizeName.PanelRadius)

	hue := palette.Series[i%len(palette.Series)]
	return &coreCell{
		card:  card,
		label: newMeta(coreLabelPrefix + strconv.Itoa(i)),
		value: styledText("", font.MonoRegular, theme.SizeNameCaptionText, palette.Text),
		track: newBarRect(palette.Surface3),
		fill:  newBarRect(hue),
	}
}

func (g *coreGrid) CreateRenderer() fyne.WidgetRenderer {
	r := &coreGridRenderer{grid: g}
	r.cells = make([]*coreCell, len(g.cores))
	for i := range r.cells {
		r.cells[i] = newCoreCell(i)
	}
	return r
}

type coreGridRenderer struct {
	grid  *coreGrid
	cells []*coreCell
	size  fyne.Size
}

func (r *coreGridRenderer) Layout(size fyne.Size) {
	r.size = size
	r.arrange()
}

func (r *coreGridRenderer) Refresh() {
	r.arrange()
	canvas.Refresh(r.grid)
}

// arrange recomputes every cell for the current size and data: the grid splits
// the widget into equal cells (cols across, rows down, core order row-major),
// and each cell centers its fixed-height content in the slot.
func (r *coreGridRenderer) arrange() {
	if len(r.cells) == 0 || r.size.Width <= 0 || r.size.Height <= 0 {
		return
	}
	cols, rows := r.dims()
	cellW := (r.size.Width - float32((cols-1)*coreGridGap)) / float32(cols)
	cellH := (r.size.Height - float32((rows-1)*coreGridGap)) / float32(rows)
	for i, cell := range r.cells {
		x := float32(i%cols) * (cellW + coreGridGap)
		y := float32(i/cols) * (cellH + coreGridGap)
		r.arrangeCell(cell, r.latest(i), x, y, cellW, cellH)
	}
}

// dims returns the grid's column and row counts for the current core count.
func (r *coreGridRenderer) dims() (cols, rows int) {
	cols = min(coreGridCols, len(r.cells))
	rows = (len(r.cells) + cols - 1) / cols
	return cols, rows
}

// latest returns core i's newest sample, or 0 while its buffer is empty.
func (r *coreGridRenderer) latest(i int) float64 {
	vals := r.grid.cores[i].Values()
	if len(vals) == 0 {
		return 0
	}
	return vals[len(vals)-1]
}

// arrangeCell lays out one cell in the slot at (x, y): the card filling the
// slot, then — inset by the card padding — label left and percent right on
// the header row with the bar underneath, the content block vertically
// centered in the card.
func (r *coreGridRenderer) arrangeCell(c *coreCell, v float64, x, y, w, h float32) {
	c.card.Resize(fyne.NewSize(w, h))
	c.card.Move(fyne.NewPos(x, y))

	c.value.Text = formatPercent(v)
	c.value.Refresh()

	innerX := x + coreCellPad
	innerW := w - 2*coreCellPad
	labelSize := c.label.MinSize()
	valueSize := c.value.MinSize()
	headerH := max(labelSize.Height, valueSize.Height)
	contentH := headerH + coreHeaderBarGap + coreBarHeight
	top := y + (h-contentH)/2

	c.label.Resize(labelSize)
	c.label.Move(fyne.NewPos(innerX, top+(headerH-labelSize.Height)/2))
	c.value.Resize(valueSize)
	c.value.Move(fyne.NewPos(innerX+innerW-valueSize.Width, top+(headerH-valueSize.Height)/2))

	barY := top + headerH + coreHeaderBarGap
	c.track.Resize(fyne.NewSize(innerW, coreBarHeight))
	c.track.Move(fyne.NewPos(innerX, barY))
	frac := clamp32(float32(v/percentMax), 0, 1)
	c.fill.Resize(fyne.NewSize(innerW*frac, coreBarHeight))
	c.fill.Move(fyne.NewPos(innerX, barY))
}

func (r *coreGridRenderer) MinSize() fyne.Size {
	if len(r.cells) == 0 {
		return fyne.Size{}
	}
	cols, rows := r.dims()
	headerH := max(r.cells[0].label.MinSize().Height, r.cells[0].value.MinSize().Height)
	cellH := headerH + coreHeaderBarGap + coreBarHeight + 2*coreCellPad
	return fyne.NewSize(
		float32(cols*coreCellMinWidth+(cols-1)*coreGridGap),
		float32(rows)*cellH+float32((rows-1)*coreGridGap),
	)
}

// Objects lists each cell's card first, then its track under its fill, with
// the texts on top. The objects are reused across frames, so listing them
// here doesn't flicker.
func (r *coreGridRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, len(r.cells)*5)
	for _, c := range r.cells {
		objs = append(objs, c.card, c.track, c.fill, c.label, c.value)
	}
	return objs
}

func (r *coreGridRenderer) Destroy() {}
