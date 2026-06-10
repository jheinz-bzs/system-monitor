package ui

// Process table adapter — wires process data into the generic dataTable widget.
//
// This file is the process-domain analog of cpu.go's relationship to lineChart:
// it knows about processRow and processSource, formats cell values, declares
// the wireframe's six columns (PID / Process / User / CPU% / bar / Mem), and
// builds a *dataTable via newProcessTable. The generic dataTable widget
// (datatable.go) never sees processRow.
//
// processSource is the seam between the monitor layer and this adapter; it is
// consumed here and implemented in app.go (the composition root), which is the
// only place that knows both monitor.ProcessInfo and processRow.

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

// Process data constants.
const (
	topCPUProcessLimit    = 10 // rows returned by topByCPU
	processTableRowHeight = 29 // px; passed as rowHeight() option to dataTable
)

// Process table column widths (px), taken from the CPU tab wireframe's table.
// All are off-grid component dimensions, not spacing-scale steps.
const (
	procColPIDW  = 74  // px; PID
	procColNameW = 195 // px; process name
	procColUserW = 74  // px; owning user
	procColCPUW  = 87  // px; CPU% value — right-aligned numeric
	procColBarW  = 90  // px; inline CPU mini-bar
	procColMemW  = 74  // px; resident memory — right-aligned numeric
)

// Column header labels. Named constants so changes propagate from one place.
const (
	colHeaderPID     = "PID"
	colHeaderProcess = "Process"
	colHeaderUser    = "User"
	colHeaderCPU     = "CPU%"
	colHeaderMem     = "Mem"
)

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

// formatPercent1 renders a CPU percentage for the table's numeric column:
// one decimal, no unit suffix ("42.0"), matching the wireframe cells.
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
