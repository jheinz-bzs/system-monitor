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

// cpuView is the CPU tab's chart pane: the overall-utilization line chart plus
// a headline readout of the most recent sample. Build it with newCPUView and
// drive live updates through refresh.
type cpuView struct {
	overall series.Source
	chart   *lineChart
	readout *canvas.Text
}

// newCPUView builds the CPU chart pane fed by the overall-utilization Source
// (oldest → newest). The chart is pinned to 0–100% and the X axis spans the
// full retention window, so points keep stable spacing as the buffer fills.
func newCPUView(overall series.Source) *cpuView {
	chart := newLineChart(
		fixedRange(0, 100),
		valueFormat(formatPercent),
		window(metrics.HistoryCapacity),
		timeAxis(metrics.HistoryCapacity*pollInterval),
	)
	chart.addSeries(overall, emphasized())

	return &cpuView{
		overall: overall,
		chart:   chart,
		readout: newMetricValue("--"),
	}
}

// object assembles the pane: a header (label over the live readout) above the
// chart, which fills the remaining space. The whole pane is inset from the tab
// edges on the spacing scale.
func (v *cpuView) object() fyne.CanvasObject {
	header := container.New(layout.NewCustomPaddedVBoxLayout(spaceXS),
		newColumnLabel("CPU Utilization"),
		v.readout,
	)
	// Gap the chart off the header without a stray spacer object.
	chart := container.New(layout.NewCustomPaddedLayout(spaceLG, 0, 0, 0), v.chart)
	body := newTightBorder(header, nil, nil, nil, chart)
	return container.New(
		layout.NewCustomPaddedLayout(space2XL, space2XL, space2XL, space2XL), body)
}

// refresh re-reads the overall history and redraws: it updates the headline
// readout to the newest sample and regenerates the chart. It touches the
// canvas, so callers driving it from a background poller must marshal it onto
// the UI goroutine (fyne.Do); see startUIRefresh.
func (v *cpuView) refresh() {
	vals := v.overall.Values()
	if n := len(vals); n > 0 {
		v.readout.Text = formatPercent(vals[n-1])
	} else {
		v.readout.Text = "--"
	}
	v.readout.Refresh()
	v.chart.Refresh()
}

// formatPercent renders a CPU percentage as a whole-number "%": the chart's Y
// tick formatter and the headline readout share it so the axis and the readout
// agree.
func formatPercent(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64) + "%"
}
