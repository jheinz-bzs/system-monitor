package ui

// CPU tab content, laid out to match tab-02-cpu-chart-per-core-table.html:
//
//	page head   — "CPU" title, core-count/model subtitle, unit segmented control
//	top pane    — utilisation panel: multi-line chart (overall + per-core)
//	bottom row  — per-core panel (left, reserved for the bar grid) and the
//	              top-processes table panel (right)
//
// The chart is built on the generic lineChart widget (linechart.go), configured
// for a percentage metric: a fixed 0–100 Y axis, "%"-suffixed tick labels, and
// a time axis spanning the full history window.
//
// The chart plots the emphasized "overall" line plus one secondary line per
// logical core (default categorical styling, c2–c8). The same per-core sources
// feed the per-core panel's bar grid (coregrid.go), which shows each core's
// newest sample.

import (
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/series"
)

// Pane / panel text. The em-dash panel titles come verbatim from the wireframe.
const (
	labelCPUPageTitle     = "CPU"
	labelUtilisation      = "Utilisation"
	labelPerCore          = "Per-core"
	labelTopCPUProcesses  = "Top processes — by CPU"
	labelAllProcessesLink = "→ all processes"
	labelLegendOverall    = "overall"
	labelLegendPerCore    = "per-core"
)

// Unit toggle labels for the page head's segmented control (static chrome —
// only the "%" series is collected today; see controls.go).
const (
	segLabelPercent = "%"
	segLabelGHz     = "GHz"
	segLabelLoad    = "load"
)

// Pane weights, from the wireframe's flex-grow ratios: the chart pane is 1.3×
// the bottom row's height; the processes panel is 1.25× the per-core panel's
// width.
const (
	chartPaneWeight   = 1.3
	bottomPaneWeight  = 1
	perCorePaneWeight = 1
	processPaneWeight = 1.25
)

// tabPad is the CPU tab's content inset and inter-pane gap (the wireframe's
// 16px sm-content padding/gap).
const tabPad = spaceXL

// perCoreSwatchSeries is the categorical-palette index of the wireframe's
// per-core legend swatch hue (c3). The per-core lines themselves auto-color
// across c2–c8; the legend shows one representative hue.
const perCoreSwatchSeries = 2

// cpuMeta is the static processor description shown in the page head's
// subtitle. The zero value means "unknown" and the subtitle is omitted.
type cpuMeta struct {
	cores int
	model string
}

// cpuView is the CPU tab: page head, utilisation chart panel, and the
// per-core + top-processes bottom row. Build with newCPUView and drive live
// updates through refresh.
type cpuView struct {
	meta       cpuMeta
	chart      *lineChart
	coreSeries []*chartSeries // chart handles for the per-core lines, core order
	grid       *coreGrid      // nil when per-core sources are not available
	table      *dataTable     // nil when ProcessCollector is not available
}

// newCPUView builds the CPU tab content. overall feeds the chart's emphasized
// line; cores feed both the chart's secondary lines and the per-core bar grid,
// and may be empty (the per-core panel body stays blank); procs feeds the
// process table and may be nil (the tab gracefully omits the table pane);
// meta fills the page-head subtitle.
func newCPUView(overall series.Source, cores []series.Source, procs processSource, meta cpuMeta) *cpuView {
	chart := newLineChart(
		fixedRange(0, percentMax),
		valueFormat(formatPercent),
		window(metrics.HistoryCapacity),
		timeAxis(historySpan()),
	)
	chart.addSeries(overall, emphasized())

	v := &cpuView{meta: meta, chart: chart}
	v.coreSeries = make([]*chartSeries, len(cores))
	for i, core := range cores {
		v.coreSeries[i] = chart.addSeries(core)
	}
	if len(cores) > 0 {
		v.grid = newCoreGrid(cores)
	}
	if procs != nil {
		v.table = newProcessTable(procs)
	}
	return v
}

// object assembles the tab: page head pinned on top, then the chart panel and
// the bottom row splitting the remaining height by the wireframe's weights.
func (v *cpuView) object() fyne.CanvasObject {
	head := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0), v.pageHead())
	column := newWeightedVBox(tabPad,
		weightedPane{object: v.chartPanel(), weight: chartPaneWeight},
		weightedPane{object: v.bottomRow(), weight: bottomPaneWeight},
	)
	body := newTightBorder(head, nil, nil, nil, column)
	return container.New(
		layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
}

// pageHead is the wireframe's sm-pagehead row: title and subtitle on the left,
// the unit segmented control on the right.
func (v *cpuView) pageHead() fyne.CanvasObject {
	row := container.New(layout.NewCustomPaddedHBoxLayout(spaceLG),
		vCenter(newHeading(labelCPUPageTitle)))
	if v.meta != (cpuMeta{}) {
		sub := fmt.Sprintf("%d cores · %s", v.meta.cores, v.meta.model)
		row.Add(vCenter(newPageSubtitle(sub)))
	}
	row.Add(layout.NewSpacer())
	row.Add(vCenter(newSegmented(0, segLabelPercent, segLabelGHz, segLabelLoad)))
	return row
}

// chartPanel wraps the utilisation chart in panel chrome. The header shows the
// overall legend entry plus the per-core toggle — a tappable chip that shows
// or hides every per-core line at once. Without per-core series the toggle
// degrades to a plain legend entry (matching the tab's other nil fallbacks).
func (v *cpuView) chartPanel() fyne.CanvasObject {
	legend := newLegend(legendEntry{label: labelLegendOverall, col: palette.Accent})
	perCore := v.perCoreControl()
	trailing := container.New(
		layout.NewCustomPaddedHBoxLayout(legendItemGap), legend, vCenter(perCore))
	return newPanel(utilisationTitle(), trailing, v.chart)
}

// perCoreControl returns the header's per-core element: the visibility toggle
// when per-core lines exist, otherwise the static legend entry.
func (v *cpuView) perCoreControl() fyne.CanvasObject {
	swatch := palette.Series[perCoreSwatchSeries]
	if len(v.coreSeries) == 0 {
		return newLegend(legendEntry{label: labelLegendPerCore, col: swatch})
	}
	return newToggleChip(labelLegendPerCore, swatch, true, v.setPerCoreVisible)
}

// setPerCoreVisible shows or hides every per-core line on the chart. The
// overall line and the per-core bar grid stay live either way.
func (v *cpuView) setPerCoreVisible(on bool) {
	for _, s := range v.coreSeries {
		s.setVisible(on)
	}
	v.chart.Refresh()
}

// bottomRow pairs the per-core panel with the top-processes table panel. When
// the process source isn't available the per-core panel takes the full row
// instead (nil-collector degradation, matching the chart tabs' placeholder
// fallback); without per-core sources the panel body stays blank.
func (v *cpuView) bottomRow() fyne.CanvasObject {
	perCore := newPanel(labelPerCore, nil, v.perCoreBody())
	if v.table == nil {
		return perCore
	}
	procs := newFlushPanel(labelTopCPUProcesses, newJumpLink(labelAllProcessesLink), v.table)
	return newWeightedHBox(
		tabPad,
		weightedPane{object: perCore, weight: perCorePaneWeight},
		weightedPane{object: procs, weight: processPaneWeight},
	)
}

// perCoreBody returns the per-core panel's content: the bar grid, or a spacer
// while no per-core sources are wired.
func (v *cpuView) perCoreBody() fyne.CanvasObject {
	if v.grid == nil {
		return layout.NewSpacer()
	}
	return v.grid
}

// utilisationTitle composes "Utilisation — last 1 min" from the actual history
// window so the panel header stays truthful if the buffer capacity changes.
func utilisationTitle() string {
	span := time.Duration(metrics.HistoryCapacity) * pollInterval
	return labelUtilisation + " — last " + formatSpan(span)
}

// refresh redraws the live panes. It touches the canvas, so callers on a
// background poller must marshal it onto the UI goroutine (fyne.Do).
func (v *cpuView) refresh() {
	v.chart.Refresh()
	if v.grid != nil {
		v.grid.Refresh()
	}
	if v.table != nil {
		v.table.Refresh()
	}
}

// formatPercent renders a CPU percentage as a whole-number "%" for the chart's
// Y tick labels.
func formatPercent(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64) + "%"
}
