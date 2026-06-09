package ui

// CPU tab content.
//
// The top pane is a time-series line chart of CPU utilization. It is built on
// the generic lineChart widget (linechart.go), configured for a percentage
// metric: a fixed 0–100 Y axis, "%"-suffixed tick labels, and a time axis
// spanning the full history window.
//
// For now the chart plots a single emphasized "overall" (total) series. The
// per-core lines (BZS253-46) attach to the SAME chart later via addSeries with
// the default categorical styling — no structural change here, just more
// series. The cpuView keeps the overall Source so its live readout can show the
// latest sample alongside the line.

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/series"
)

const (
	labelCPUUtilization  = "CPU Utilization"
	labelTopCPUProcesses = "Top CPU Processes"
)

// cpuView is the CPU tab: an overall-utilization line chart at the top and a
// top-CPU-processes table at the bottom. Build with newCPUView and drive live
// updates through refresh.
type cpuView struct {
	overall series.Source
	procs   processSource // nil when ProcessCollector is not available
	chart   *lineChart
	table   *dataTable // nil when procs is nil
	readout *canvas.Text
}

// newCPUView builds the CPU tab content. overall feeds the chart; procs feeds
// the process table and may be nil (the tab gracefully omits the table pane).
func newCPUView(overall series.Source, procs processSource) *cpuView {
	chart := newLineChart(
		fixedRange(0, 100),
		valueFormat(formatPercent),
		window(metrics.HistoryCapacity),
		timeAxis(metrics.HistoryCapacity*pollInterval),
	)
	chart.addSeries(overall, emphasized())

	v := &cpuView{
		overall: overall,
		procs:   procs,
		chart:   chart,
		readout: newMetricValue("--"),
	}
	if procs != nil {
		v.table = newProcessTable(procs)
	}
	return v
}

// object assembles the tab: the chart section fills the center; the process
// table is pinned below it when available. The whole view is inset from the
// tab edges on the spacing scale.
func (v *cpuView) object() fyne.CanvasObject {
	chartHeader := container.New(layout.NewCustomPaddedVBoxLayout(spaceXS),
		newColumnLabel(labelCPUUtilization),
		v.readout,
	)
	chartBody := container.New(layout.NewCustomPaddedLayout(spaceLG, 0, 0, 0), v.chart)
	chartSection := newTightBorder(chartHeader, nil, nil, nil, chartBody)

	var body fyne.CanvasObject = chartSection
	if v.table != nil {
		tableHeader := newColumnLabel(labelTopCPUProcesses)
		tableBody := container.New(layout.NewCustomPaddedLayout(spaceLG, 0, 0, 0), v.table)
		tableSection := newTightBorder(tableHeader, nil, nil, nil, tableBody)
		body = newTightBorder(nil, tableSection, nil, nil, chartSection)
	}

	return container.New(
		layout.NewCustomPaddedLayout(space2XL, space2XL, space2XL, space2XL), body)
}

// refresh re-reads live data and redraws both panes. It touches the canvas, so
// callers on a background poller must marshal it onto the UI goroutine (fyne.Do).
func (v *cpuView) refresh() {
	vals := v.overall.Values()
	if n := len(vals); n > 0 {
		v.readout.Text = formatPercent(vals[n-1])
	} else {
		v.readout.Text = "--"
	}
	v.readout.Refresh()
	v.chart.Refresh()
	if v.table != nil {
		v.table.Refresh()
	}
}

// formatPercent renders a CPU percentage as a whole-number "%": the chart's Y
// tick formatter and the headline readout share it so the axis and the readout
// agree.
func formatPercent(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64) + "%"
}
