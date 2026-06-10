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
// For now the chart plots a single emphasized "overall" (total) series. The
// per-core lines (BZS253-46) attach to the SAME chart later via addSeries with
// the default categorical styling, and the per-core panel body receives its
// bar grid — no structural change here, just more content in reserved slots.

import (
	"fmt"
	"strconv"

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
// across c2–c8 when they attach (BZS253-46); the legend shows one
// representative hue.
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
	meta  cpuMeta
	chart *lineChart
	table *dataTable // nil when ProcessCollector is not available
}

// newCPUView builds the CPU tab content. overall feeds the chart; procs feeds
// the process table and may be nil (the tab gracefully omits the table pane);
// meta fills the page-head subtitle.
func newCPUView(overall series.Source, procs processSource, meta cpuMeta) *cpuView {
	chart := newLineChart(
		fixedRange(0, percentMax),
		valueFormat(formatPercent),
		window(metrics.HistoryCapacity),
		timeAxis(historySpan()),
	)
	chart.addSeries(overall, emphasized())

	v := &cpuView{meta: meta, chart: chart}
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

// chartPanel wraps the utilisation chart in panel chrome with the
// overall/per-core legend.
func (v *cpuView) chartPanel() fyne.CanvasObject {
	legend := newLegend(
		legendEntry{label: labelLegendOverall, col: palette.Accent},
		legendEntry{label: labelLegendPerCore, col: palette.Series[perCoreSwatchSeries]},
	)
	return newPanel(historyTitle(labelUtilisation), legend, v.chart)
}

// bottomRow pairs the per-core panel with the top-processes table panel. The
// per-core body is reserved space for the bar grid (BZS253-46). When the
// process source isn't available the per-core panel takes the full row instead
// (nil-collector degradation, matching the chart tabs' placeholder fallback).
func (v *cpuView) bottomRow() fyne.CanvasObject {
	perCore := newPanel(labelPerCore, nil, layout.NewSpacer())
	if v.table == nil {
		return perCore
	}
	procs := newPanel(labelTopCPUProcesses, newJumpLink(labelAllProcessesLink), v.table)
	return newWeightedHBox(
		tabPad,
		weightedPane{object: perCore, weight: perCorePaneWeight},
		weightedPane{object: procs, weight: processPaneWeight},
	)
}

// refresh redraws both live panes. It touches the canvas, so callers on a
// background poller must marshal it onto the UI goroutine (fyne.Do).
func (v *cpuView) refresh() {
	v.chart.Refresh()
	if v.table != nil {
		v.table.Refresh()
	}
}

// formatPercent renders a CPU percentage as a whole-number "%" for the chart's
// Y tick labels.
func formatPercent(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64) + "%"
}
