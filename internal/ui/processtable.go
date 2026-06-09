package ui

// Process table adapter — wires process data into the generic dataTable widget.
//
// This file is the process-domain analog of cpu.go's relationship to lineChart:
// it knows about processRow and processSource, formats cell values, declares
// the three columns, and builds a *dataTable via newProcessTable. The generic
// dataTable widget (datatable.go) never sees processRow.
//
// processSource is the seam between the monitor layer and this adapter; it is
// consumed here and implemented in app.go (the composition root), which is the
// only place that knows both monitor.ProcessInfo and processRow.

import (
	"strconv"

	"fyne.io/fyne/v2"
)

// Process data constants.
const (
	topCPUProcessLimit    = 10  // rows returned by topByCPU
	processTableRowHeight = 29  // px; passed as rowHeight() option to dataTable
)

// Process table column widths (px). Sized for the three-column layout; all
// are off-grid component dimensions, not spacing-scale steps.
const (
	procColNameW = 140 // px; process name
	procColCPUW  = 60  // px; CPU% value — fits "100%" at 12px mono
	procColPIDW  = 56  // px; PID — fits 6-digit PIDs at 12px mono
)

// Column header labels. Named constants so changes propagate from one place.
const (
	colHeaderProcess = "Process"
	colHeaderCPU     = "CPU%"
	colHeaderPID     = "PID"
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
	cpu  float64 // 0..100
}

// processTableSource implements TableSource for a processSource. It formats
// each processRow into a string triple that dataTable lays out as cells.
type processTableSource struct {
	src processSource
}

// Snapshot calls topByCPU on every Refresh to return a live snapshot, exactly
// as series.Source.Values() is called on every lineChart arrange().
func (s *processTableSource) Snapshot() [][]string {
	rows := s.src.topByCPU(topCPUProcessLimit)
	cells := make([][]string, len(rows))
	for i, r := range rows {
		cells[i] = []string{
			r.name,
			formatPercent(r.cpu),
			strconv.Itoa(int(r.pid)),
		}
	}
	return cells
}

// newProcessTable builds a *dataTable configured for the process view:
// three columns (Process / CPU% / PID) fed by src.
func newProcessTable(src processSource) *dataTable {
	return newDataTable(
		&processTableSource{src: src},
		tableColumns(
			tableColumn{header: colHeaderProcess, width: procColNameW, align: fyne.TextAlignLeading},
			tableColumn{header: colHeaderCPU, width: procColCPUW, align: fyne.TextAlignTrailing},
			tableColumn{header: colHeaderPID, width: procColPIDW, align: fyne.TextAlignTrailing},
		),
		rowHeight(processTableRowHeight),
	)
}
