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
// The renderer pre-allocates canvas objects for a fixed pool of rows (default
// tableRowPoolSize; rowPool overrides). On each Refresh it calls
// src.Snapshot() to get the current rows (the same pull-on-render pattern as
// lineChart calling src.Values() inside arrange()), then shows or hides pool
// rows based on the live count and available height.
//
// A table hosted in a scroll container combines sizeToRows() (so its MinSize —
// and therefore the scrollbar — spans every data row) with setViewport (so
// only the rows actually on screen are laid out and refreshed). That keeps a
// full process list of hundreds of rows cheap at a 1s refresh: per tick the
// pool's ~viewport-worth of objects update, never one object per data row.

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Table geometry. Heights come from the design-system-03 spec; pool size sets
// how many rows the renderer pre-allocates — not a visible row limit.
const (
	tableHeaderHeight = 34 // px; design-system-03 panel/column header height
	tableDefaultRowH  = 29 // px; design-system-03 table row height
	tableRowPoolSize  = 20 // renderer pre-allocation; not a display cap
	tableMinVisRows   = 3  // default data rows contributing to MinSize (minVisibleRows overrides)
	tableOverdrawRows = 2  // viewport-windowing slack rows beyond the visible slice
)

// noTableRow / noTableColumn are the "none" sentinels for row indices (hover,
// selection) and column hit tests.
const (
	noTableRow    = -1
	noTableColumn = -1
)

// Mini-bar geometry for bar columns (wireframe table bar-cell).
const (
	tableBarWidth  = 66 // px; bar track width
	tableBarHeight = 5  // px; bar track/fill height
)

// tableCellHPad insets cell content from its column edge — the wireframe
// table's 12px th/td padding. With a flush table panel this is also what
// separates the first/last columns from the panel border.
const tableCellHPad = spaceLG // 12

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

// tableRowHighlighter is an optional TableSource extension: sources that track
// a selected row report its index in the current snapshot (noTableRow for
// none). The renderer asserts for it right after Snapshot(), so the index and
// the rows it points into always come from the same snapshot.
type tableRowHighlighter interface {
	highlightedRow() int
}

// tableCell is one cell's display data: text for text and pill columns; frac
// (0..1) drives the fill of bar columns; pill selects a pill cell's semantic
// color role (only read by pill columns, which always set it).
type tableCell struct {
	text string
	frac float64
	pill statusKind
}

// columnKind selects how a column renders its cells.
type columnKind int

const (
	columnText columnKind = iota // render tableCell.text
	columnBar                    // render tableCell.frac as a mini bar
	columnPill                   // render tableCell.text in status-pill chrome
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

// minVisibleRows overrides how many data rows contribute to MinSize (default:
// tableMinVisRows). A table wrapped in a scroll container raises this to its
// row limit so a short pane scrolls to reach every row instead of clipping the
// list at the pane's edge.
func minVisibleRows(n int) tableOption {
	return func(t *dataTable) {
		if n > 0 {
			t.minRows = n
		}
	}
}

// rowPool overrides how many rows the renderer pre-allocates (default:
// tableRowPoolSize). A viewport-windowed table raises this to cover the
// tallest viewport it expects to fill.
func rowPool(n int) tableOption {
	return func(t *dataTable) {
		if n > 0 {
			t.poolSize = n
		}
	}
}

// sizeToRows makes MinSize track the snapshot's row count instead of the
// minVisibleRows floor, so a hosting scroll container's content height — and
// scrollbar — always spans exactly the current rows.
func sizeToRows() tableOption {
	return func(t *dataTable) { t.sizesToRows = true }
}

// onRowTapped registers a callback fired with the data-row index when a row is
// tapped. Registering it also enables the hover highlight and pointer cursor.
func onRowTapped(fn func(row int)) tableOption {
	return func(t *dataTable) { t.onRowTap = fn }
}

// onHeaderTapped registers a callback fired with the column index when a
// header cell is tapped (the seam sortable tables hang sorting on).
func onHeaderTapped(fn func(col int)) tableOption {
	return func(t *dataTable) { t.onHeaderTap = fn }
}

// dataTable is a design-system-styled data table widget. Build with
// newDataTable; drive live updates with Refresh().
type dataTable struct {
	widget.BaseWidget

	src         TableSource
	cols        []tableColumn
	rowH        float32
	minRows     int
	poolSize    int
	sizesToRows bool

	// viewportY/H is the vertical slice a hosting scroll container shows
	// (setViewport); zero height means "not scroll-hosted". rowCount is the
	// last snapshot's row count, cached by arrange() for MinSize and hit
	// testing. hoverRow is the pointer-hovered data row, noTableRow when none.
	viewportY float32
	viewportH float32
	rowCount  int
	hoverRow  int

	onRowTap    func(row int)
	onHeaderTap func(col int)
}

// Interactivity contracts. Tables without tap callbacks stay inert: taps
// no-op, the hover row never sets, and the cursor stays default.
var (
	_ fyne.Tappable      = (*dataTable)(nil)
	_ desktop.Hoverable  = (*dataTable)(nil)
	_ desktop.Cursorable = (*dataTable)(nil)
)

// newDataTable builds a dataTable fed by src and configured by opts.
func newDataTable(src TableSource, opts ...tableOption) *dataTable {
	t := &dataTable{
		src:      src,
		rowH:     tableDefaultRowH,
		minRows:  tableMinVisRows,
		poolSize: tableRowPoolSize,
		hoverRow: noTableRow,
	}
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
	rows := t.minRows
	if t.sizesToRows {
		rows = max(t.rowCount, t.minRows)
	}
	return fyne.NewSize(totalW, tableHeaderHeight+t.rowH*float32(rows))
}

func (t *dataTable) CreateRenderer() fyne.WidgetRenderer {
	return newDataTableRenderer(t)
}

// setViewport tells a scroll-hosted table which vertical slice of itself is
// visible, so arrange() lays out only the rows intersecting it and pins the
// header to the slice's top. Call it from the scroll container's OnScrolled
// (and each refresh tick), then Refresh. A zero height restores the
// render-from-the-top behavior of unhosted tables.
func (t *dataTable) setViewport(y, h float32) {
	t.viewportY = y
	t.viewportH = h
}

// setColumnHeader rewrites one column's header label (sort markers); the next
// arrange picks it up.
func (t *dataTable) setColumnHeader(i int, header string) {
	if i >= 0 && i < len(t.cols) {
		t.cols[i].header = header
	}
}

// Tapped implements fyne.Tappable: a tap on the header band reports the
// column, a tap on a data row reports the row.
func (t *dataTable) Tapped(e *fyne.PointEvent) {
	if t.inHeaderBand(e.Position.Y) {
		if t.onHeaderTap == nil {
			return
		}
		if col := columnAtX(effectiveWidths(t.cols, t.Size().Width), e.Position.X); col != noTableColumn {
			t.onHeaderTap(col)
		}
		return
	}
	if t.onRowTap == nil {
		return
	}
	if row := t.rowAt(e.Position.Y); row != noTableRow {
		t.onRowTap(row)
	}
}

// MouseIn / MouseMoved / MouseOut implement desktop.Hoverable, tracking which
// data row the pointer is over. Repaint happens only when the hovered row
// changes — never per pixel of motion.
func (t *dataTable) MouseIn(e *desktop.MouseEvent)    { t.setHoverRow(t.hoverTarget(e.Position)) }
func (t *dataTable) MouseMoved(e *desktop.MouseEvent) { t.setHoverRow(t.hoverTarget(e.Position)) }
func (t *dataTable) MouseOut()                        { t.setHoverRow(noTableRow) }

// Cursor implements desktop.Cursorable: a pointer over interactive tables,
// the default arrow otherwise.
func (t *dataTable) Cursor() desktop.Cursor {
	if t.onRowTap == nil && t.onHeaderTap == nil {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

// hoverTarget maps a pointer position to the data row it should highlight:
// noTableRow for inert tables and the header band.
func (t *dataTable) hoverTarget(p fyne.Position) int {
	if t.onRowTap == nil || t.inHeaderBand(p.Y) {
		return noTableRow
	}
	return t.rowAt(p.Y)
}

func (t *dataTable) setHoverRow(row int) {
	if row == t.hoverRow {
		return
	}
	t.hoverRow = row
	t.Refresh()
}

// inHeaderBand reports whether a widget-space y falls on the header, which
// arrange pins to the top of the current viewport slice.
func (t *dataTable) inHeaderBand(y float32) bool {
	rel := y - t.viewportY
	return rel >= 0 && rel < tableHeaderHeight
}

// rowAt maps a widget-space y to its data-row index, noTableRow outside the
// rows. Rows sit at absolute positions below the (unpinned) header origin, so
// no viewport adjustment applies.
func (t *dataTable) rowAt(y float32) int {
	row := int((y - tableHeaderHeight) / t.rowH)
	if row < 0 || row >= t.rowCount {
		return noTableRow
	}
	return row
}

// columnAtX maps a widget-space x to its column index, noTableColumn past the
// last column.
func columnAtX(colW []float32, x float32) int {
	var edge float32
	for i, w := range colW {
		edge += w
		if x < edge {
			return i
		}
	}
	return noTableColumn
}

// tableBar is the pooled canvas pair for one bar cell: a fixed-width track
// with an accent fill scaled by the cell's frac.
type tableBar struct {
	track *canvas.Rectangle
	fill  *canvas.Rectangle
}

// tablePill is the pooled canvas pair for one pill cell: status-pill chrome
// (tinted fill, border-strong outline) hugging a mono status label. It is the
// raw-canvas analog of newStatusPill, which builds the same chrome from
// containers — too heavy to rebuild per row per tick.
type tablePill struct {
	bg   *canvas.Rectangle
	text *canvas.Text
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
	rowBGs        []*canvas.Rectangle // poolSize
	rowDividers   []*canvas.Line      // poolSize
	rowCells      [][]*canvas.Text    // [row][col]; nil entries at bar/pill columns
	rowBars       [][]*tableBar       // [row][col]; nil entries at other kinds
	rowPills      [][]*tablePill      // [row][col]; nil entries at other kinds
	snapshot      [][]tableCell
	highlight     int       // selected data row from tableRowHighlighter; noTableRow when none
	colW          []float32 // effective column widths for the current size
	size          fyne.Size
}

func newDataTableRenderer(t *dataTable) *dataTableRenderer {
	r := &dataTableRenderer{table: t, highlight: noTableRow}

	r.headerBG = canvas.NewRectangle(palette.Surface2)
	r.headerDivider = newTableDivider()

	for _, c := range t.cols {
		r.headerCells = append(r.headerCells, newTableHeaderLabel(c.header))
	}

	r.rowBGs = make([]*canvas.Rectangle, t.poolSize)
	r.rowDividers = make([]*canvas.Line, t.poolSize)
	r.rowCells = make([][]*canvas.Text, t.poolSize)
	r.rowBars = make([][]*tableBar, t.poolSize)
	r.rowPills = make([][]*tablePill, t.poolSize)
	for i := range t.poolSize {
		r.rowBGs[i] = canvas.NewRectangle(color.Transparent)
		r.rowDividers[i] = newTableDivider()
		r.rowCells[i], r.rowBars[i], r.rowPills[i] = newCellPools(t.cols)
	}

	return r
}

// newCellPools allocates one row's cell objects: a text per text column, a bar
// pair per bar column, and pill chrome per pill column, each at its column's
// slot (nil in the other pools).
func newCellPools(cols []tableColumn) ([]*canvas.Text, []*tableBar, []*tablePill) {
	cells := make([]*canvas.Text, len(cols))
	bars := make([]*tableBar, len(cols))
	pills := make([]*tablePill, len(cols))
	for j, col := range cols {
		switch col.kind {
		case columnBar:
			bars[j] = newTableBar()
		case columnPill:
			pills[j] = newTablePill()
		default:
			cell := newTableText("")
			if col.color != nil {
				cell.Color = col.color
			}
			cells[j] = cell
		}
	}
	return cells, bars, pills
}

// newTablePill builds a pooled status pill: border-strong outline, chip
// radius, and the status-pill text role. Fill and text color are set per cell
// in showPill.
func newTablePill() *tablePill {
	bg := canvas.NewRectangle(color.Transparent)
	bg.StrokeColor = palette.BorderStrong
	bg.StrokeWidth = 1
	bg.CornerRadius = pillRadius
	return &tablePill{
		bg:   bg,
		text: styledText("", font.MonoRegular, sizeName.StatusPill, palette.Text2),
	}
}

// newTableBar builds a pooled mini bar: surface-3 track, accent fill.
func newTableBar() *tableBar {
	return &tableBar{
		track: newBarRect(palette.Surface3),
		fill:  newBarRect(palette.Accent),
	}
}

// newBarRect builds a bar track/fill rectangle in the wireframes' bar chrome:
// a flat color with rounded ends (the 2px chip radius every wireframe bar
// carries). Shared by the table mini-bars and the CPU per-core grid so the
// bar language stays uniform.
func newBarRect(col color.Color) *canvas.Rectangle {
	r := canvas.NewRectangle(col)
	r.CornerRadius = theme.Size(theme.SizeNameInputRadius)
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
	r.table.rowCount = len(r.snapshot)
	r.highlight = noTableRow
	if h, ok := r.table.src.(tableRowHighlighter); ok {
		r.highlight = h.highlightedRow()
	}
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

// layoutHeader places the header band. It pins to the top of the viewport
// slice (top is 0 for unhosted tables), so a scroll-hosted table keeps a
// sticky header — the band is painted last (Objects), covering scrolled rows.
// Label text is re-applied from the column defs each pass so setColumnHeader
// rewrites (sort markers) take effect.
func (r *dataTableRenderer) layoutHeader() {
	top := r.table.viewportY
	r.headerBG.Resize(fyne.NewSize(r.size.Width, tableHeaderHeight))
	r.headerBG.Move(fyne.NewPos(0, top))

	var x float32
	for i, col := range r.table.cols {
		if i >= len(r.headerCells) {
			break
		}
		lbl := r.headerCells[i]
		lbl.Text = strings.ToUpper(col.header)
		sz := lbl.MinSize()
		lbl.Resize(sz)
		lbl.Move(fyne.NewPos(
			alignedCellX(x, col.align, r.colW[i], sz.Width), top+(tableHeaderHeight-sz.Height)/2))
		lbl.Refresh()
		x += r.colW[i]
	}

	r.headerDivider.Position1 = fyne.NewPos(0, top+tableHeaderHeight)
	r.headerDivider.Position2 = fyne.NewPos(r.size.Width, top+tableHeaderHeight)
	r.headerDivider.Refresh()
}

// alignedCellX is the single home of the column-alignment rule, shared by the
// header labels and the data cells: trailing columns right-align against the
// column edge, everything else leads, both inset by tableCellHPad.
func alignedCellX(colX float32, align fyne.TextAlign, colW, contentW float32) float32 {
	if align == fyne.TextAlignTrailing {
		return colX + colW - contentW - tableCellHPad
	}
	return colX + tableCellHPad
}

// layoutRows shows pool rows for the visible data rows and hides the rest.
// Pool slot i displays data row first+i at that row's absolute y position.
func (r *dataTableRenderer) layoutRows() {
	rowH := r.table.rowH
	first, count := r.visibleRowRange(rowH)

	for i := range r.table.poolSize {
		if i >= count {
			r.hideRow(i)
			continue
		}
		row := first + i
		r.showRow(i, row, tableHeaderHeight+float32(row)*rowH)
	}
}

// visibleRowRange resolves which data rows to draw: with a viewport set, the
// rows intersecting it (plus overdraw slack); otherwise rows from the top
// until the widget height is filled. Both are capped by the pool.
func (r *dataTableRenderer) visibleRowRange(rowH float32) (first, count int) {
	if r.table.viewportH > 0 {
		first = max(0, int((r.table.viewportY-tableHeaderHeight)/rowH))
		count = int(r.table.viewportH/rowH) + tableOverdrawRows
	} else {
		count = int((r.size.Height - tableHeaderHeight) / rowH)
	}
	count = min(count, len(r.snapshot)-first, r.table.poolSize)
	return first, max(count, 0)
}

// showRow paints data row `row` into pool slot `pool` at row top y.
func (r *dataTableRenderer) showRow(pool, row int, y float32) {
	rowH := r.table.rowH
	r.rowBGs[pool].FillColor = r.rowFill(row)
	r.rowBGs[pool].Resize(fyne.NewSize(r.size.Width, rowH))
	r.rowBGs[pool].Move(fyne.NewPos(0, y))
	r.rowBGs[pool].Refresh()
	r.rowBGs[pool].Show()

	divY := y + rowH
	r.rowDividers[pool].Position1 = fyne.NewPos(0, divY)
	r.rowDividers[pool].Position2 = fyne.NewPos(r.size.Width, divY)
	r.rowDividers[pool].Refresh()
	r.rowDividers[pool].Show()

	var x float32
	for j, col := range r.table.cols {
		cell := r.cellAt(row, j)
		switch col.kind {
		case columnBar:
			r.showBar(r.rowBars[pool][j], cell, x, y, r.colW[j])
		case columnPill:
			r.showPill(r.rowPills[pool][j], cell, x, y)
		default:
			r.showText(r.rowCells[pool][j], cell, x, y, col.align, r.colW[j])
		}
		x += r.colW[j]
	}
}

// rowFill is a data row's background: selected wins over hovered, everything
// else stays transparent (the design-system row states).
func (r *dataTableRenderer) rowFill(row int) color.Color {
	switch row {
	case r.highlight:
		return palette.Surface3
	case r.table.hoverRow:
		return palette.Surface2
	default:
		return color.Transparent
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

// showPill paints one status pill at its column's leading edge: chrome hugging
// the label, tinted by the cell's semantic role. An empty status hides the
// pill entirely rather than drawing hollow chrome around nothing.
func (r *dataTableRenderer) showPill(pill *tablePill, cell tableCell, x, y float32) {
	if cell.text == "" {
		pill.bg.Hide()
		pill.text.Hide()
		return
	}
	pill.text.Text = cell.text
	pill.text.Color = statusColor(cell.pill)
	sz := pill.text.MinSize()

	w := sz.Width + 2*pillHPad
	h := sz.Height + 2*pillVPad
	pillX := x + tableCellHPad
	pillY := y + (r.table.rowH-h)/2

	pill.bg.FillColor = tablePillFill(cell.pill)
	pill.bg.Resize(fyne.NewSize(w, h))
	pill.bg.Move(fyne.NewPos(pillX, pillY))
	pill.bg.Refresh()
	pill.bg.Show()

	pill.text.Resize(sz)
	pill.text.Move(fyne.NewPos(pillX+pillHPad, pillY+pillVPad))
	pill.text.Refresh()
	pill.text.Show()
}

// tablePillFill is a table pill's background for its semantic role. Unlike
// newStatusPill's neutral (surface-3), the wireframe's table draws neutral
// pills hollow — outline only — so quiet states don't shout in a dense list.
func tablePillFill(kind statusKind) color.Color {
	if kind == status.Neutral {
		return color.Transparent
	}
	return pillFill(kind)
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
	for _, p := range r.rowPills[i] {
		if p != nil {
			p.bg.Hide()
			p.text.Hide()
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
	for i := range r.table.poolSize {
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
		for _, p := range r.rowPills[i] {
			if p != nil {
				objs = append(objs, p.bg, p.text)
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
