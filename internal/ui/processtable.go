package ui

// Process table adapters — wire process data into the generic dataTable widget.
//
// This file is the process-domain analog of cpu.go's relationship to lineChart:
// it knows about processRow, formats cell values, and declares the wireframes'
// two top-process tables — the CPU tab's (PID / Process / User / CPU% / bar /
// Mem, newProcessTable) and the Memory tab's (PID / Process / User / RSS / bar
// / %Mem, newMemProcessTable). The generic dataTable widget (datatable.go)
// never sees processRow.
//
// processSource and memProcessSource are the seams between the monitor layer
// and these adapters; they are consumed here and implemented in app.go (the
// composition root), which is the only place that knows both
// monitor.ProcessInfo and processRow.

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

// Process data constants.
const (
	topCPUProcessLimit    = 10 // rows returned by topByCPU
	topMemProcessLimit    = 10 // rows returned by topByMemory
	processTableRowHeight = 29 // px; passed as rowHeight() option to dataTable
)

// Process table column widths (px), taken from the CPU tab wireframe's table
// and shared by the Memory tab's table (the same design-system component; the
// flex name column absorbs the wider pane). All are off-grid component
// dimensions, not spacing-scale steps.
const (
	procColPIDW  = 74  // px; PID
	procColNameW = 195 // px; process name
	procColUserW = 74  // px; owning user
	procColCPUW  = 87  // px; CPU% value — right-aligned numeric
	procColBarW  = 90  // px; inline mini-bar
	procColMemW  = 74  // px; resident memory — right-aligned numeric
	procColPctW  = 74  // px; %Mem value — right-aligned numeric
)

// Column header labels. Named constants so changes propagate from one place.
const (
	colHeaderPID     = "PID"
	colHeaderProcess = "Process"
	colHeaderUser    = "User"
	colHeaderCPU     = "CPU%"
	colHeaderMem     = "Mem"
	colHeaderRSS     = "RSS"
	colHeaderPctMem  = "Mem%"
)

// sortDescMarker tags the column a table is sorted by, descending. The
// wireframes draw ▾ (U+25BE), which the bundled IBM Plex fonts lack; ↓ is the
// closest covered glyph.
const sortDescMarker = " ↓"

// processSource is the data seam between the monitor layer and this adapter.
// Defined here at the consumer (ui package) per idiomatic Go; app.go adapts
// the concrete ProcessCollector to this interface without creating a cross-
// layer import.
type processSource interface {
	topByCPU(n int) []processRow
}

// processSourceFunc adapts any func(int)[]processRow to processSource.
type processSourceFunc func(n int) []processRow

func (f processSourceFunc) topByCPU(n int) []processRow { return f(n) }

// PID is a typed process identifier, carried as a first-class value so
// cross-tab navigation links resolve without string parsing or type assertions.
type PID int32

// processRow is the display-layer shape for one process. app.go selects and
// converts from monitor.ProcessInfo when building the adapter, so this type
// never appears in the monitor package.
type processRow struct {
	pid  PID
	name string
	user string
	cpu  float64 // 0..100, machine-wide scale
	mem  uint64  // resident set bytes
}

// processTableSource implements TableSource for a processSource. It formats
// each processRow into the six wireframe columns; the CPU value feeds both the
// numeric column and the bar column's fill fraction.
type processTableSource struct {
	src processSource
}

// Snapshot calls topByCPU on every Refresh to return a live snapshot, exactly
// as series.Source.Values() is called on every lineChart arrange().
func (s *processTableSource) Snapshot() [][]tableCell {
	rows := s.src.topByCPU(topCPUProcessLimit)
	cells := make([][]tableCell, len(rows))
	for i, r := range rows {
		cells[i] = []tableCell{
			{text: strconv.Itoa(int(r.pid))},
			{text: r.name},
			{text: shortUsername(r.user)},
			{text: formatPercent1(r.cpu)},
			{frac: r.cpu / percentMax},
			{text: formatBytesShort(r.mem)},
		}
	}
	return cells
}

// shortUsername strips the Windows "DOMAIN\" qualifier so the User column
// shows the bare account name the wireframe's narrow column expects.
func shortUsername(user string) string {
	if i := strings.LastIndexByte(user, '\\'); i >= 0 {
		return user[i+1:]
	}
	return user
}

// formatPercent1 renders a percentage for the tables' numeric columns (CPU%,
// %Mem): one decimal, no unit suffix ("42.0"), matching the wireframe cells.
func formatPercent1(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// newProcessTable builds a *dataTable configured for the process view: the six
// wireframe columns fed by src. The name column renders in primary text; the
// rest keep the table-data default (text-2).
func newProcessTable(src processSource) *dataTable {
	return newDataTable(
		&processTableSource{src: src},
		tableColumns(
			tableColumn{header: colHeaderPID, width: procColPIDW, align: fyne.TextAlignLeading},
			tableColumn{header: colHeaderProcess, width: procColNameW, align: fyne.TextAlignLeading, color: palette.Text, flex: true},
			tableColumn{header: colHeaderUser, width: procColUserW, align: fyne.TextAlignLeading},
			tableColumn{header: colHeaderCPU, width: procColCPUW, align: fyne.TextAlignTrailing},
			tableColumn{header: "", width: procColBarW, kind: columnBar},
			tableColumn{header: colHeaderMem, width: procColMemW, align: fyne.TextAlignTrailing},
		),
		rowHeight(processTableRowHeight),
	)
}

// memProcessSource is the Memory tab's data seam to the monitor layer: the
// top-by-memory analog of processSource. Defined here at the consumer per
// idiomatic Go; app.go adapts the concrete ProcessCollector to it.
type memProcessSource interface {
	topByMemory(n int) []processRow
}

// memProcessSourceFunc adapts any func(int)[]processRow to memProcessSource.
type memProcessSourceFunc func(n int) []processRow

func (f memProcessSourceFunc) topByMemory(n int) []processRow { return f(n) }

// memBarFullScalePct is the Mem% value at which a memory-table bar fills its
// whole track, measured from the wireframe's fills (its 5.6%-of-total row
// fills 56% of the track). Percentage points, not a fraction.
const memBarFullScalePct = 10

// memProcessTableSource implements TableSource for the Memory tab's table. It
// formats each processRow into the wireframe's columns; total (physical memory
// bytes) scales the Mem% column and the bars.
type memProcessTableSource struct {
	src   memProcessSource
	total uint64
}

// Snapshot calls topByMemory on every Refresh to return a live snapshot. The
// bars fill linearly with each row's share of physical memory, reaching a full
// track at memBarFullScalePct (the wireframe's scale — against the raw 0..100%
// domain every bar would sit near-empty).
func (s *memProcessTableSource) Snapshot() [][]tableCell {
	rows := s.src.topByMemory(topMemProcessLimit)
	cells := make([][]tableCell, len(rows))
	for i, r := range rows {
		pct := byteFraction(r.mem, s.total) * percentMax
		cells[i] = []tableCell{
			{text: strconv.Itoa(int(r.pid))},
			{text: r.name},
			{text: shortUsername(r.user)},
			{text: formatBytesShort(r.mem)},
			{frac: min(pct/memBarFullScalePct, 1)},
			{text: formatPercent1(pct)},
		}
	}
	return cells
}

// byteFraction returns part/whole as a 0..1 fraction, 0 when whole is zero
// (an unknown total must not divide by zero or fill a bar).
func byteFraction(part, whole uint64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}

// newMemProcessTable builds a *dataTable configured for the Memory tab's
// top-processes pane: the wireframe's columns fed by src, with total physical
// memory scaling the %Mem column. minVisibleRows keeps every top-N row
// reachable when the table sits in a scroll container.
func newMemProcessTable(src memProcessSource, total uint64) *dataTable {
	return newDataTable(
		&memProcessTableSource{src: src, total: total},
		tableColumns(
			tableColumn{header: colHeaderPID, width: procColPIDW, align: fyne.TextAlignLeading},
			tableColumn{header: colHeaderProcess, width: procColNameW, align: fyne.TextAlignLeading, color: palette.Text, flex: true},
			tableColumn{header: colHeaderUser, width: procColUserW, align: fyne.TextAlignLeading},
			tableColumn{header: colHeaderRSS + sortDescMarker, width: procColMemW, align: fyne.TextAlignTrailing},
			tableColumn{header: "", width: procColBarW, kind: columnBar},
			tableColumn{header: colHeaderPctMem, width: procColPctW, align: fyne.TextAlignTrailing},
		),
		rowHeight(processTableRowHeight),
		minVisibleRows(topMemProcessLimit),
	)
}
