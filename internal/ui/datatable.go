package ui

// dataTable is a generic, design-system-styled data table widget. It is the
// table analog of lineChart: deliberately decoupled from any domain type.
//
// Data flows through TableSource (Snapshot() [][]tableCell): the caller
// pre-formats each cell; the table only lays them out. This mirrors how
// lineChart consumes series.Source (Values() []float64) and applies its own
// formatters — the widget is blind to what the values mean.
//
// Columns are declared at construction via tableColumns(). Each column carries
// a header label, a pixel width, a text alignment, an optional cell color, and
// a kind: text columns render their cell's text, bar columns render the cell's
// frac as a mini bar gauge (the wireframes' inline CPU bars). rowHeight()
// overrides the default 29px row height when needed.
//
// The renderer pre-allocates canvas objects for tableRowPoolSize rows. On each
// Refresh it calls src.Snapshot() to get the current rows (the same pull-on-
// render pattern as lineChart calling src.Values() inside arrange()), then
// shows or hides pool rows based on the live count and available height.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// Table geometry. Heights come from the design-system-03 spec; pool size sets
// how many rows the renderer pre-allocates — not a visible row limit.
const (
	tableHeaderHeight = 34 // px; design-system-03 panel/column header height
	tableDefaultRowH  = 29 // px; design-system-03 table row height
	tableRowPoolSize  = 20 // renderer pre-allocation; not a display cap
	tableMinVisRows   = 3  // minimum visible data rows contributing to MinSize
)

// Mini-bar geometry for bar columns (wireframe table bar-cell).
const (
	tableBarWidth  = 66 // px; bar track width
	tableBarHeight = 5  // px; bar track/fill height
)

// TableSource is the data seam for dataTable. Snapshot returns the current
// row set as pre-formatted cells — one inner slice per row, one tableCell per
// column. The table calls Snapshot on every Refresh; the implementation
// should return a live snapshot, not a cached one.
//
// Defined here at the consumer (ui package) per idiomatic Go, so the
// adapters in processtable.go (and future tables) stay free of any cross-layer
// import.
type TableSource interface {
	Snapshot() [][]tableCell
}

// tableCell is one cell's display data: text for text columns; frac (0..1)
// drives the fill of bar columns.
type tableCell struct {
	text string
	frac float64
}

// columnKind selects how a column renders its cells.
type columnKind int

const (
	columnText columnKind = iota // render tableCell.text
	columnBar                    // render tableCell.frac as a mini bar
)

// tableColumn declares one column: its header label, pixel width, cell text
// alignment, rendering kind, and an optional cell text color (nil falls back
// to the table-data default, text-2). width is a minimum for the flex column:
// the flex column absorbs any widget width beyond the declared sum, so the
// table fills its panel and the trailing column hugs the right edge.
type tableColumn struct {
	header string
	width  float32
	align  fyne.TextAlign
	kind   columnKind
	color  color.Color
	flex   bool
}

// tableOption configures a dataTable at construction, mirroring lineChartOption.
type tableOption func(*dataTable)

// tableColumns sets the column definitions. Pass one tableColumn per column
// in left-to-right order.
func tableColumns(cols ...tableColumn) tableOption {
	return func(t *dataTable) { t.cols = cols }
}

// rowHeight overrides the per-row pixel height (default: tableDefaultRowH).
func rowHeight(h float32) tableOption {
	return func(t *dataTable) {
		if h > 0 {
			t.rowH = h
		}
	}
}

// dataTable is a design-system-styled data table widget. Build with
// newDataTable; drive live updates with Refresh().
type dataTable struct {
	widget.BaseWidget

	src  TableSource
	cols []tableColumn
	rowH float32
}

// newDataTable builds a dataTable fed by src and configured by opts.
func newDataTable(src TableSource, opts ...tableOption) *dataTable {
	t := &dataTable{src: src, rowH: tableDefaultRowH}
	for _, opt := range opts {
		opt(t)
	}
	t.ExtendBaseWidget(t)
	return t
}

func (t *dataTable) MinSize() fyne.Size {
	var totalW float32
	for _, c := range t.cols {
		totalW += c.width
	}
	minH := tableHeaderHeight + t.rowH*tableMinVisRows
	return fyne.NewSize(totalW, minH)
}

func (t *dataTable) CreateRenderer() fyne.WidgetRenderer {
	return newDataTableRenderer(t)
}

// tableBar is the pooled canvas pair for one bar cell: a fixed-width track
// with an accent fill scaled by the cell's frac.
type tableBar struct {
	track *canvas.Rectangle
	fill  *canvas.Rectangle
}

// dataTableRenderer draws the table: a header row followed by data rows from
// a pre-allocated pool. Canvas objects are allocated once in
// newDataTableRenderer and reused — the same strategy as lineChartRenderer's
// grid-line and label pools.
type dataTableRenderer struct {
	table         *dataTable
	headerBG      *canvas.Rectangle
	headerDivider *canvas.Line
	headerCells   []*canvas.Text      // one per column
	rowBGs        []*canvas.Rectangle // tableRowPoolSize
	rowDividers   []*canvas.Line      // tableRowPoolSize
	rowCells      [][]*canvas.Text    // [row][col]; nil entries at bar columns
	rowBars       [][]*tableBar       // [row][col]; nil entries at text columns
	snapshot      [][]tableCell
	colW          []float32 // effective column widths for the current size
	size          fyne.Size
}

func newDataTableRenderer(t *dataTable) *dataTableRenderer {
	r := &dataTableRenderer{table: t}

	r.headerBG = canvas.NewRectangle(palette.Surface2)
	r.headerDivider = newTableDivider()

	for _, c := range t.cols {
		r.headerCells = append(r.headerCells, newTableHeaderLabel(c.header))
	}

	r.rowBGs = make([]*canvas.Rectangle, tableRowPoolSize)
	r.rowDividers = make([]*canvas.Line, tableRowPoolSize)
	r.rowCells = make([][]*canvas.Text, tableRowPoolSize)
	r.rowBars = make([][]*tableBar, tableRowPoolSize)
	for i := range tableRowPoolSize {
		r.rowBGs[i] = canvas.NewRectangle(color.Transparent)
		r.rowDividers[i] = newTableDivider()
		r.rowCells[i], r.rowBars[i] = newCellPools(t.cols)
	}

	return r
}

// newCellPools allocates one row's cell objects: a text per text column and a
// bar pair per bar column, each at its column's slot (nil in the other pool).
func newCellPools(cols []tableColumn) ([]*canvas.Text, []*tableBar) {
	cells := make([]*canvas.Text, len(cols))
	bars := make([]*tableBar, len(cols))
	for j, col := range cols {
		if col.kind == columnBar {
			bars[j] = newTableBar()
			continue
		}
		cell := newTableText("")
		if col.color != nil {
			cell.Color = col.color
		}
		cells[j] = cell
	}
	return cells, bars
}

// newTableBar builds a pooled mini bar: surface-3 track, accent fill.
func newTableBar() *tableBar {
	return &tableBar{
		track: canvas.NewRectangle(palette.Surface3),
		fill:  canvas.NewRectangle(palette.Accent),
	}
}

// newTableDivider builds a 1px border-colored horizontal line.
func newTableDivider() *canvas.Line {
	l := canvas.NewLine(palette.Border)
	l.StrokeWidth = 1
	return l
}

func (r *dataTableRenderer) Layout(size fyne.Size) {
	r.size = size
	r.arrange()
}

func (r *dataTableRenderer) Refresh() {
	r.arrange()
	canvas.Refresh(r.table)
}

// arrange pulls a fresh snapshot and repositions all canvas objects for the
// current size. Mirrors lineChartRenderer.arrange().
func (r *dataTableRenderer) arrange() {
	if r.size.Width <= 0 || r.size.Height <= 0 {
		return
	}
	r.snapshot = r.table.src.Snapshot()
	r.colW = effectiveWidths(r.table.cols, r.size.Width)
	r.layoutHeader()
	r.layoutRows()
}

// effectiveWidths returns each column's render width for the given widget
// width: the declared widths, with any extra beyond their sum granted to the
// (first) flex column. Without a flex column, or when there is no extra, the
// declared widths stand.
func effectiveWidths(cols []tableColumn, total float32) []float32 {
	widths := make([]float32, len(cols))
	var sum float32
	flexIdx := -1
	for i, c := range cols {
		widths[i] = c.width
		sum += c.width
		if c.flex && flexIdx < 0 {
			flexIdx = i
		}
	}
	if extra := total - sum; flexIdx >= 0 && extra > 0 {
		widths[flexIdx] += extra
	}
	return widths
}

func (r *dataTableRenderer) layoutHeader() {
	r.headerBG.Resize(fyne.NewSize(r.size.Width, tableHeaderHeight))
	r.headerBG.Move(fyne.NewPos(0, 0))

	var x float32
	for i, col := range r.table.cols {
		if i >= len(r.headerCells) {
			break
		}
		lbl := r.headerCells[i]
		sz := lbl.MinSize()
		lbl.Resize(sz)
		lbl.Move(fyne.NewPos(
			alignedCellX(x, col.align, r.colW[i], sz.Width), (tableHeaderHeight-sz.Height)/2))
		x += r.colW[i]
	}

	r.headerDivider.Position1 = fyne.NewPos(0, tableHeaderHeight)
	r.headerDivider.Position2 = fyne.NewPos(r.size.Width, tableHeaderHeight)
	r.headerDivider.Refresh()
}

// alignedCellX is the single home of the column-alignment rule, shared by the
// header labels and the data cells: trailing columns right-align against the
// column edge, everything else leads, both inset by spaceSM.
func alignedCellX(colX float32, align fyne.TextAlign, colW, contentW float32) float32 {
	if align == fyne.TextAlignTrailing {
		return colX + colW - contentW - spaceSM
	}
	return colX + spaceSM
}

func (r *dataTableRenderer) layoutRows() {
	rowH := r.table.rowH
	availH := r.size.Height - tableHeaderHeight
	visibleRows := min(tableRowPoolSize, min(len(r.snapshot), int(availH/rowH)))

	for i := range tableRowPoolSize {
		if i >= visibleRows {
			r.hideRow(i)
			continue
		}
		rowY := tableHeaderHeight + float32(i)*rowH
		r.showRow(i, rowY)
	}
}

func (r *dataTableRenderer) showRow(i int, y float32) {
	rowH := r.table.rowH
	r.rowBGs[i].Resize(fyne.NewSize(r.size.Width, rowH))
	r.rowBGs[i].Move(fyne.NewPos(0, y))
	r.rowBGs[i].Show()

	divY := y + rowH
	r.rowDividers[i].Position1 = fyne.NewPos(0, divY)
	r.rowDividers[i].Position2 = fyne.NewPos(r.size.Width, divY)
	r.rowDividers[i].Refresh()
	r.rowDividers[i].Show()

	var x float32
	for j, col := range r.table.cols {
		cell := r.cellAt(i, j)
		if col.kind == columnBar {
			r.showBar(r.rowBars[i][j], cell, x, y, r.colW[j])
		} else {
			r.showText(r.rowCells[i][j], cell, x, y, col.align, r.colW[j])
		}
		x += r.colW[j]
	}
}

// cellAt returns the snapshot cell for [row i, col j], or a zero cell when the
// snapshot is ragged or shorter than the column set.
func (r *dataTableRenderer) cellAt(i, j int) tableCell {
	if i < len(r.snapshot) && j < len(r.snapshot[i]) {
		return r.snapshot[i][j]
	}
	return tableCell{}
}

// showText positions one text cell within its column at row top y.
func (r *dataTableRenderer) showText(text *canvas.Text, cell tableCell, x, y float32, align fyne.TextAlign, colW float32) {
	text.Text = cell.text
	text.Alignment = align
	sz := text.MinSize()
	text.Resize(sz)
	text.Move(fyne.NewPos(alignedCellX(x, align, colW, sz.Width), y+(r.table.rowH-sz.Height)/2))
	text.Refresh()
	text.Show()
}

// showBar positions one bar cell: the track centered in the column, the fill
// scaled by the cell's frac (clamped to the track).
func (r *dataTableRenderer) showBar(bar *tableBar, cell tableCell, x, y, colW float32) {
	barX := x + (colW-tableBarWidth)/2
	barY := y + (r.table.rowH-tableBarHeight)/2

	bar.track.Resize(fyne.NewSize(tableBarWidth, tableBarHeight))
	bar.track.Move(fyne.NewPos(barX, barY))
	bar.track.Show()

	frac := clamp32(float32(cell.frac), 0, 1)
	bar.fill.Resize(fyne.NewSize(tableBarWidth*frac, tableBarHeight))
	bar.fill.Move(fyne.NewPos(barX, barY))
	bar.fill.Show()
}

func (r *dataTableRenderer) hideRow(i int) {
	r.rowBGs[i].Hide()
	r.rowDividers[i].Hide()
	for _, c := range r.rowCells[i] {
		if c != nil {
			c.Hide()
		}
	}
	for _, b := range r.rowBars[i] {
		if b != nil {
			b.track.Hide()
			b.fill.Hide()
		}
	}
}

// Objects returns all canvas objects in back-to-front draw order: row
// backgrounds, row dividers, row cells (text and bars), then the header layer
// on top.
func (r *dataTableRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0)
	for _, bg := range r.rowBGs {
		objs = append(objs, bg)
	}
	for _, div := range r.rowDividers {
		objs = append(objs, div)
	}
	for i := range tableRowPoolSize {
		for _, c := range r.rowCells[i] {
			if c != nil {
				objs = append(objs, c)
			}
		}
		for _, b := range r.rowBars[i] {
			if b != nil {
				objs = append(objs, b.track, b.fill)
			}
		}
	}
	objs = append(objs, r.headerBG, r.headerDivider)
	for _, lbl := range r.headerCells {
		objs = append(objs, lbl)
	}
	return objs
}

func (r *dataTableRenderer) MinSize() fyne.Size {
	return r.table.MinSize()
}

func (r *dataTableRenderer) Destroy() {}
