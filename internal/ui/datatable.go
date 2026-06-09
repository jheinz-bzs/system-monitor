package ui

// dataTable is a generic, design-system-styled data table widget. It is the
// table analog of lineChart: deliberately decoupled from any domain type.
//
// Data flows through TableSource (Snapshot() [][]string): the caller
// pre-formats each cell value; the table only lays them out. This mirrors how
// lineChart consumes series.Source (Values() []float64) and applies its own
// formatters — the widget is blind to what the numbers mean.
//
// Columns are declared at construction via tableColumns(). Each column carries
// a header label, a pixel width, and a text alignment. rowHeight() overrides
// the default 29px row height when needed.
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

// TableSource is the data seam for dataTable. Snapshot returns the current
// row set as pre-formatted cell strings — one inner slice per row, one string
// per column. The table calls Snapshot on every Refresh; the implementation
// should return a live snapshot, not a cached one.
//
// Defined here at the consumer (ui package) per idiomatic Go, so the
// adapters in processtable.go (and future tables) stay free of any cross-layer
// import.
type TableSource interface {
	Snapshot() [][]string
}

// tableColumn declares one column: its header label, pixel width, and cell
// text alignment.
type tableColumn struct {
	header string
	width  float32
	align  fyne.TextAlign
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

// dataTableRenderer draws the table: a header row followed by data rows from
// a pre-allocated pool. Canvas objects are allocated once in
// newDataTableRenderer and reused — the same strategy as lineChartRenderer's
// grid-line and label pools.
type dataTableRenderer struct {
	table         *dataTable
	headerBG      *canvas.Rectangle
	headerDivider *canvas.Line
	headerCells   []*canvas.Text     // one per column
	rowBGs        []*canvas.Rectangle // tableRowPoolSize
	rowDividers   []*canvas.Line      // tableRowPoolSize
	rowCells      [][]*canvas.Text    // [row][col], tableRowPoolSize rows
	snapshot      [][]string
	size          fyne.Size
}

func newDataTableRenderer(t *dataTable) *dataTableRenderer {
	r := &dataTableRenderer{table: t}

	r.headerBG = canvas.NewRectangle(palette.Surface2)
	r.headerDivider = newTableDivider()

	for _, c := range t.cols {
		r.headerCells = append(r.headerCells, newColumnLabel(c.header))
	}

	nCols := len(t.cols)
	r.rowBGs = make([]*canvas.Rectangle, tableRowPoolSize)
	r.rowDividers = make([]*canvas.Line, tableRowPoolSize)
	r.rowCells = make([][]*canvas.Text, tableRowPoolSize)
	for i := range tableRowPoolSize {
		r.rowBGs[i] = canvas.NewRectangle(color.Transparent)
		r.rowDividers[i] = newTableDivider()
		cells := make([]*canvas.Text, nCols)
		for j := range nCols {
			cells[j] = newTableText("")
		}
		r.rowCells[i] = cells
	}

	return r
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
	r.layoutHeader()
	r.layoutRows()
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
		lbl.Move(fyne.NewPos(x+spaceSM, (tableHeaderHeight-sz.Height)/2))
		x += col.width
	}

	r.headerDivider.Position1 = fyne.NewPos(0, tableHeaderHeight)
	r.headerDivider.Position2 = fyne.NewPos(r.size.Width, tableHeaderHeight)
	r.headerDivider.Refresh()
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
		if j >= len(r.rowCells[i]) {
			break
		}
		cell := r.rowCells[i][j]
		if i < len(r.snapshot) && j < len(r.snapshot[i]) {
			cell.Text = r.snapshot[i][j]
		} else {
			cell.Text = ""
		}
		cell.Alignment = col.align
		sz := cell.MinSize()
		cell.Resize(sz)
		var cellX float32
		if col.align == fyne.TextAlignTrailing {
			cellX = x + col.width - sz.Width - spaceSM
		} else {
			cellX = x + spaceSM
		}
		cell.Move(fyne.NewPos(cellX, y+(rowH-sz.Height)/2))
		cell.Refresh()
		cell.Show()
		x += col.width
	}
}

func (r *dataTableRenderer) hideRow(i int) {
	r.rowBGs[i].Hide()
	r.rowDividers[i].Hide()
	for _, c := range r.rowCells[i] {
		c.Hide()
	}
}

// Objects returns all canvas objects in back-to-front draw order: row
// backgrounds, row dividers, row cells, then the header layer on top.
func (r *dataTableRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0)
	for _, bg := range r.rowBGs {
		objs = append(objs, bg)
	}
	for _, div := range r.rowDividers {
		objs = append(objs, div)
	}
	for _, cells := range r.rowCells {
		for _, c := range cells {
			objs = append(objs, c)
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
