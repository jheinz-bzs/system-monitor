package ui

// Memory tab content (BZS253-49/-50), laid out to match
// tab-03-memory-line-chart-breakdown.html:
//
//	page head   — "Memory" title, total-physical-memory subtitle
//	top pane    — composition panel: stacked area chart (used / cached / free)
//	bottom pane — top-processes-by-memory table (the final split layout lands
//	              with BZS253-51)
//
// The chart is the generic lineChart widget in stacked-area mode: series stack
// bottom → top in declaration order (used, cached, free), the Y axis is FIXED
// to total physical memory — the stack always sums to ≈ total, so the fixed
// axis is what makes remaining headroom readable at a glance — and Y ticks are
// compact byte labels. Series hues come from the wireframe's swatches: used is
// the accent, cached is categorical c2, free is the muted remainder slate.
//
// The view reads MemoryCollector history through series.Source only — never
// gopsutil or monitor types directly (the composition root adapts in app.go).

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/series"
)

// Pane / panel text. The em-dash panel title comes verbatim from the wireframe;
// the chart legend labels are lowercase there, unlike the breakdown rows.
const (
	labelMemoryPageTitle = "Memory"
	labelComposition     = "Composition"
	labelTopMemProcesses = "Top processes — by memory"
	labelLegendUsed      = "used"
	labelLegendCached    = "cached"
	labelLegendFree      = "free"
)

// memCachedSeriesIndex is the categorical-palette index of the wireframe's
// cached-band hue (c2, #36c2d4).
const memCachedSeriesIndex = 1

// memSources bundles the Memory tab's live data: one Source per stacked band
// plus total physical memory (static — it pins the chart's fixed Y axis).
type memSources struct {
	used   series.Source
	cached series.Source
	free   series.Source
	total  uint64
}

// wired reports whether the memory collector was successfully adapted; an
// unwired tab falls back to its placeholder.
func (m memSources) wired() bool {
	hasUsed := m.used != nil
	hasCached := m.cached != nil
	hasFree := m.free != nil
	hasTotal := m.total > 0
	return hasUsed && hasCached && hasFree && hasTotal
}

// memoryView is the Memory tab: page head, composition chart panel, and the
// top-processes-by-memory table pane. Build with newMemoryView and drive live
// updates through refresh.
type memoryView struct {
	total       uint64
	chart       *lineChart
	tableScroll *scrollTable       // scroll-hosted top-processes table; nil when no source
	tableSrc    *processTableModel // backs row tap → PID and header sorting; nil with no table
	nav         processNavigator   // cross-tab navigation; nil when not wired
}

// newMemoryView builds the Memory tab content from the adapted collector
// sources. Series are added bottom → top: used anchors the stack, cached sits
// on it, free tops it off at ≈ total. procs feeds the top-processes table and
// may be nil (the bottom pane keeps its placeholder body).
func newMemoryView(src memSources, procs allProcessSource, nav processNavigator) *memoryView {
	chart := newLineChart(
		fixedRange(0, float64(src.total)),
		valueFormat(formatBytesAxis),
		window(metrics.HistoryCapacity),
		timeAxis(historySpan()),
		stackedArea(),
	)
	chart.addSeries(src.used, seriesColor(palette.Accent))
	chart.addSeries(src.cached, seriesColor(palette.Series[memCachedSeriesIndex]))
	chart.addSeries(src.free, seriesColor(palette.SeriesMuted))

	v := &memoryView{total: src.total, chart: chart, nav: nav}
	if procs != nil {
		table, ts := newMemProcessTable(procs, src.total, v.tapRow, v.tapHeader)
		v.tableSrc = ts
		v.tableScroll = newScrollTable(table)
		syncSortHeaders(table, ts)
	}
	return v
}

// tapRow follows a tapped top-process row to the Processes tab, highlighting the
// process it represents. A row whose process vanished between snapshot and tap
// (index out of range) is ignored.
func (v *memoryView) tapRow(row int) {
	if v.nav == nil {
		return
	}
	if pid, ok := v.tableSrc.pidAt(row); ok {
		v.nav.showProcess(pid)
	}
}

// tapHeader re-sorts the top-processes table by the tapped column (tap again to
// flip), re-tagging the sort markers — the same header behavior as the
// Processes tab.
func (v *memoryView) tapHeader(col int) {
	tapSortHeader(v.tableScroll.table, v.tableSrc, col)
}

// object assembles the tab: page head pinned on top, then the chart panel and
// the top-processes pane splitting the remaining height by the same weights
// as the CPU tab (BZS253-51 finalizes the Memory split).
func (v *memoryView) object() fyne.CanvasObject {
	head := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0), v.pageHead())
	column := newWeightedVBox(tabPad,
		weightedPane{object: v.chartPanel(), weight: chartPaneWeight},
		weightedPane{object: v.bottomPane(), weight: bottomPaneWeight},
	)
	body := newTightBorder(head, nil, nil, nil, column)
	return container.New(
		layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
}

// pageHead is the wireframe's sm-pagehead row: title and the static
// total-memory subtitle. (The wireframe's live used/total readout belongs to
// the breakdown pane, BZS253-50.)
func (v *memoryView) pageHead() fyne.CanvasObject {
	sub := fmt.Sprintf("%s total physical", formatBytesShort(v.total))
	return container.New(layout.NewCustomPaddedHBoxLayout(spaceLG),
		vCenter(newHeading(labelMemoryPageTitle)),
		vCenter(newPageSubtitle(sub)))
}

// chartPanel wraps the composition chart in panel chrome with the
// used/cached/free legend, swatches matching the bands' full hues.
func (v *memoryView) chartPanel() fyne.CanvasObject {
	legend := newLegend(
		legendEntry{label: labelLegendUsed, col: palette.Accent},
		legendEntry{label: labelLegendCached, col: palette.Series[memCachedSeriesIndex]},
		legendEntry{label: labelLegendFree, col: palette.SeriesMuted},
	)
	return newPanel(historyTitle(labelComposition), legend, v.chart)
}

// bottomPane is the top-processes-by-memory table panel. Its header carries the
// wireframe's RSS/% unit toggle (static, like the CPU page head's unit control;
// the toggle relabels the RSS column, so it shares that column's label const)
// plus the "→ all processes" cross-nav link to the Processes tab — the CPU
// tab's top-processes panel precedent. The table scrolls when the pane is too
// short for the full top-N list. Without a process source the panel body stays
// blank (nil-collector degradation, matching the CPU tab's fallbacks).
func (v *memoryView) bottomPane() fyne.CanvasObject {
	if v.tableScroll == nil {
		return newPanel(labelTopMemProcesses, nil, layout.NewSpacer())
	}
	toggle := newSegmented(0, colHeaderRSS, segLabelPercent)
	link := newJumpLink(labelAllProcessesLink, v.showAllProcesses)
	trailing := container.New(layout.NewCustomPaddedHBoxLayout(legendItemGap),
		vCenter(toggle), vCenter(link))
	return newFlushPanel(labelTopMemProcesses, trailing, v.tableScroll.object())
}

// showAllProcesses follows the "→ all processes" header link to the Processes
// tab. Inert when navigation isn't wired (the link still renders as chrome).
func (v *memoryView) showAllProcesses() {
	if v.nav != nil {
		v.nav.showProcesses()
	}
}

// refresh redraws the live panes. It touches the canvas, so callers on a
// background poller must marshal it onto the UI goroutine (fyne.Do).
func (v *memoryView) refresh() {
	v.chart.Refresh()
	if v.tableScroll != nil {
		v.tableScroll.refresh()
	}
}
