package ui

// Overview tab content: a 3-column grid of self-contained metric panels (adapted
// from the wireframe's tab-01-overview-panel-grid), giving an at-a-glance health
// view of the machine. Each panel shows a live headline value, a 1-minute
// sparkline of the same metric, and two footer readouts — all re-read from the
// ring buffers on every refresh. A panel whose source isn't wired falls back to
// its static placeholder content.

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/series"
)

// Overview page heading and panel titles.
const (
	labelOverviewPageTitle = "Overview"
	labelOverviewSubtitle  = "Live · all systems"
	labelOverallStatus     = "HEALTHY"

	labelCPUPanel       = "CPU"
	labelMemoryPanel    = "Memory"
	labelDiskIOPanel    = "Disk I/O"
	labelNetworkPanel   = "Network"
	labelSwapPanel      = "Swap"
	labelProcessesPanel = "Processes"
)

// Live readout fragments composed into panel values and footers.
const (
	labelUnitPercent = "%"
	labelUnitProcs   = "procs"
	labelCoresSuffix = " cores"
	labelPeakPrefix  = "peak "
	labelUsedSuffix  = " used"
	labelCachePrefix = "cache "
	labelReadPrefix  = "r "
	labelWritePrefix = "w "
	labelDownPrefix  = "↓ "
	labelUpPrefix    = "↑ "
	labelTotalSlash  = "/"
	labelRateGap     = " " // between a rate value and its unit in a footer
)

// Overview panel geometry.
const (
	overviewSparkMinHeight = 64 // px; reserved height for a chartless panel's plot
	overviewDotSize        = 8  // px; status indicator dot
	overviewColumns        = 3  // panels per row
)

// overviewMetric is one panel's identity, static fallback content, and the
// builder for its live behavior. bind returns the zero overviewLive when the
// metric's source isn't wired, leaving the panel on its static fallback.
type overviewMetric struct {
	title     string
	value     string
	unit      string
	state     statusKind
	footLeft  string
	footRight string
	tab       tabID // the tab a click on this panel navigates to
	bind      func(overviewSources) overviewLive
}

// Swap shares the Memory tab — there is no dedicated Swap tab.
var overviewMetrics = []overviewMetric{
	{labelCPUPanel, "0", labelUnitPercent, status.Healthy, "", "", tabCPU, bindCPU},
	{labelMemoryPanel, "0", "", status.Healthy, "", "", tabMemory, bindMemory},
	{labelDiskIOPanel, "0", "", status.Warning, "", "", tabDisk, bindDiskIO},
	{labelNetworkPanel, "0", "", status.Healthy, "", "", tabNetwork, bindNetwork},
	{labelSwapPanel, "0", "", status.Healthy, "", "", tabMemory, bindSwap},
	{labelProcessesPanel, "0", labelUnitProcs, status.Healthy, "", "", tabProcesses, bindProcesses},
}

// overviewSources bundles the live series the panels read. Each field may be
// zero/nil; a panel without its source falls back to static content. Built in
// the tab registry from the same collector sources the metric tabs use, so the
// Overview reuses existing series rather than re-collecting.
type overviewSources struct {
	cpu      series.Source
	cpuCores int
	mem      memSources
	diskIO   diskIOSources
	net      netSources
	swap     swapSources
	procs    series.Source // process-count history; nil when not wired
}

// swapSources is the Overview Swap panel's live data: a used-swap history plus
// total swap (static — it pins the panel's fixed chart range). The zero value
// means swap isn't wired (no swap, or unreadable), and the panel stays static.
type swapSources struct {
	used  series.Source
	total uint64
}

// wired reports whether swap was adapted and the machine actually has swap.
func (s swapSources) wired() bool {
	return s.used != nil && s.total > 0
}

// readout is one panel's text content for a refresh: the big value, its small
// unit, and the two footer cells. Each field maps to a distinct styled widget.
type readout struct {
	value, unit, footLeft, footRight string
}

// overviewLive is a panel's live behavior: the sparkline source (nil = no
// chart, reserved plot area instead), the chart's range/format options, and the
// readout function producing the panel's text each refresh.
type overviewLive struct {
	src  series.Source
	opts []lineChartOption
	read func() readout
}

// bindings maps each wired metric's title to its live behavior. Unwired metrics
// are absent, so their panels keep the static fallback. add inserts a binding
// only when its metric is wired; an overviewLive is safe to build eagerly even
// when unwired because its read closure is never invoked and nil sources are
// only stored, never dereferenced.
func (s overviewSources) bindings() map[string]overviewLive {
	m := make(map[string]overviewLive)
	add := func(ok bool, title string, live overviewLive) {
		if ok {
			m[title] = live
		}
	}

	add(s.cpu != nil, labelCPUPanel, overviewLive{
		src:  s.cpu,
		opts: []lineChartOption{fixedRange(0, percentMax), valueFormat(formatPercentAxis)},
		read: func() readout {
			vals := s.cpu.Values()
			return readout{
				value:     formatWhole(latestSample(vals)),
				unit:      labelUnitPercent,
				footLeft:  strconv.Itoa(s.cpuCores) + labelCoresSuffix,
				footRight: labelPeakPrefix + formatWhole(peakSample(vals)) + labelUnitPercent,
			}
		},
	})

	add(s.mem.wired(), labelMemoryPanel, overviewLive{
		src:  s.mem.used,
		opts: []lineChartOption{fixedRange(0, float64(s.mem.total)), valueFormat(formatBytesAxis)},
		read: func() readout {
			r := byteUsage(latestSample(s.mem.used.Values()), s.mem.total)
			r.footRight = labelCachePrefix + formatBytesShort(uint64(latestSample(s.mem.cached.Values())))
			return r
		},
	})

	add(s.swap.wired(), labelSwapPanel, overviewLive{
		src:  s.swap.used,
		opts: []lineChartOption{autoRange(), valueFormat(formatBytesAxis)},
		read: func() readout {
			vals := s.swap.used.Values()
			r := byteUsage(latestSample(vals), s.swap.total)
			r.footRight = labelPeakPrefix + formatBytesShort(uint64(peakSample(vals)))
			return r
		},
	})

	add(s.diskIO.wired(), labelDiskIOPanel,
		rateBinding(s.diskIO.total, s.diskIO.read, s.diskIO.write, labelReadPrefix, labelWritePrefix))
	add(s.net.wired(), labelNetworkPanel,
		rateBinding(s.net.total, s.net.download, s.net.upload, labelDownPrefix, labelUpPrefix))

	add(s.procs != nil, labelProcessesPanel, overviewLive{
		src:  s.procs,
		opts: []lineChartOption{autoRange(), valueFormat(formatWhole)},
		read: func() readout {
			vals := s.procs.Values()
			return readout{
				value:    formatWhole(latestSample(vals)),
				unit:     labelUnitProcs,
				footLeft: labelPeakPrefix + formatWhole(peakSample(vals)),
			}
		},
	})

	return m
}

// byteUsage fills the value, unit, and footLeft of a "used / total" memory-style
// panel (Memory and Swap share this); the caller sets footRight. footLeft is the
// percent-used readout.
func byteUsage(used float64, total uint64) readout {
	pct := used / float64(total) * percentMax
	return readout{
		value:    formatBytesShort(uint64(used)),
		unit:     labelTotalSlash + formatBytesShort(total),
		footLeft: formatWhole(pct) + labelUnitPercent + labelUsedSuffix,
	}
}

// rateBinding builds the live behavior for a bytes/sec panel: an auto-scaled
// total headline plus two labeled tributary rates in the footer (read/write,
// download/upload). The Disk I/O and Network panels share this exact shape.
func rateBinding(total, a, b series.Source, prefixA, prefixB string) overviewLive {
	return overviewLive{
		src:  total,
		opts: []lineChartOption{autoRange(), valueFormat(formatBytesAxis)},
		read: func() readout {
			value, unit := formatRate(latestSample(total.Values()))
			return readout{
				value:     value,
				unit:      unit,
				footLeft:  prefixA + formatRateText(latestSample(a.Values())),
				footRight: prefixB + formatRateText(latestSample(b.Values())),
			}
		},
	}
}

// overviewView is the live Overview tab. Build with newOverviewView and drive
// updates through refresh.
type overviewView struct {
	panels []*overviewPanel
	nav    tabNavigator // jumps to a panel's tab on click; nil leaves panels inert
}

// overviewPanel is one live card: the readout text widgets, an optional
// sparkline, and the readout function. read is nil for a static panel.
type overviewPanel struct {
	read                      func() readout
	value, unit, footL, footR *canvas.Text
	chart                     *lineChart
	card                      fyne.CanvasObject
}

// refresh re-reads the panel's data and redraws its text and sparkline. It
// touches the canvas, so a background poller must marshal it via fyne.Do.
func (p *overviewPanel) refresh() {
	if p.read != nil {
		r := p.read()
		p.value.Text, p.unit.Text, p.footL.Text, p.footR.Text = r.value, r.unit, r.footLeft, r.footRight
		p.value.Refresh()
		p.unit.Refresh()
		p.footL.Refresh()
		p.footR.Refresh()
	}
	if p.chart != nil {
		p.chart.Refresh()
	}
}

// refresh redraws every live panel.
func (v *overviewView) refresh() {
	for _, p := range v.panels {
		p.refresh()
	}
}

// newOverview builds the Overview tab with no live sources and no navigation —
// every panel shows its static fallback and is inert. Retained for the
// widget-path render test.
func newOverview() fyne.CanvasObject {
	return newOverviewView(overviewSources{}, nil).object()
}

// newOverviewView builds the live Overview tab from the available sources. nav
// makes each panel clickable, jumping to that metric's tab; pass nil for inert
// panels.
func newOverviewView(src overviewSources, nav tabNavigator) *overviewView {
	binds := src.bindings()
	v := &overviewView{nav: nav}
	for _, m := range overviewMetrics {
		v.newPanel(m, binds[m.title]) // newPanel appends to v.panels
	}
	return v
}

// object assembles the tab: page header above the equal-weight panel grid.
func (v *overviewView) object() fyne.CanvasObject {
	rows := make([]weightedPane, 0, 2)
	for start := 0; start < len(v.panels); start += overviewColumns {
		end := min(start+overviewColumns, len(v.panels))
		panes := make([]weightedPane, 0, overviewColumns)
		for _, p := range v.panels[start:end] {
			panes = append(panes, weightedPane{object: p.card, weight: 1})
		}
		for len(panes) < overviewColumns {
			panes = append(panes, weightedPane{object: layout.NewSpacer(), weight: 1})
		}
		rows = append(rows, weightedPane{object: newWeightedHBox(tabPad, panes...), weight: 1})
	}

	head := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0), overviewHead())
	body := newTightBorder(head, nil, nil, nil, newWeightedVBox(tabPad, rows...))
	return container.New(layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
}

// overviewHead is the page header: title and subtitle on the left, the overall
// machine-health pill on the right.
func overviewHead() fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedHBoxLayout(spaceMD),
		vCenter(newHeading(labelOverviewPageTitle)),
		vCenter(newPageSubtitle(labelOverviewSubtitle)),
		layout.NewSpacer(),
		vCenter(newStatusPill(labelOverallStatus, status.Healthy)),
	)
}

// newPanel builds one card, attaching live behavior when b is wired (b.read !=
// nil) and a sparkline when b carries a source. It registers the panel for
// refresh and returns it.
func (v *overviewView) newPanel(m overviewMetric, b overviewLive) *overviewPanel {
	p := &overviewPanel{
		read:  b.read,
		value: newMetricValue(m.value),
		unit:  newTableText(m.unit),
		footL: newMeta(m.footLeft),
		footR: newMeta(m.footRight),
	}

	spark := v.sparkArea(p, b)
	value := container.New(layout.NewCustomPaddedHBoxLayout(spaceMD), p.value, vCenter(p.unit))
	footer := container.NewHBox(p.footL, layout.NewSpacer(), p.footR)
	content := newTightBorder(value, footer, nil, nil, spark)
	p.card = newMetricPanel(m.title, m.state, content)
	if v.nav != nil {
		tab := m.tab
		p.card = newClickableCard(p.card, func() { v.nav.showTab(tab) })
	}

	if p.read != nil {
		p.refresh() // paint initial live values before the first poll tick
	}
	v.panels = append(v.panels, p)
	return p
}

// clickableCard wraps a panel so a click anywhere on it (including the
// sparkline, which isn't itself tappable) jumps to the panel's tab, with a
// pointer cursor over it. Mirrors jumpLink's tappable/cursor pattern.
type clickableCard struct {
	widget.BaseWidget

	content fyne.CanvasObject
	onTap   func()
}

var (
	_ fyne.Tappable      = (*clickableCard)(nil)
	_ desktop.Cursorable = (*clickableCard)(nil)
)

// newClickableCard wraps content so a tap invokes onTap.
func newClickableCard(content fyne.CanvasObject, onTap func()) *clickableCard {
	c := &clickableCard{content: content, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

// Tapped implements fyne.Tappable.
func (c *clickableCard) Tapped(_ *fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

// Cursor implements desktop.Cursorable — a pointer when the card navigates.
func (c *clickableCard) Cursor() desktop.Cursor {
	if c.onTap == nil {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (c *clickableCard) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.content)
}

// sparkArea returns the panel's plot region: a 1-minute line chart of b.src
// when wired, otherwise a reserved empty plot rectangle so chartless panels
// keep the same height.
func (v *overviewView) sparkArea(p *overviewPanel, b overviewLive) fyne.CanvasObject {
	var plot fyne.CanvasObject
	if b.src != nil {
		opts := append([]lineChartOption{window(metrics.HistoryCapacity)}, b.opts...)
		p.chart = newLineChart(opts...)
		p.chart.addSeries(b.src, emphasized())
		plot = p.chart
	} else {
		rect := canvas.NewRectangle(palette.PlotBG)
		rect.CornerRadius = theme.Size(sizeName.PanelRadius)
		rect.SetMinSize(fyne.NewSize(0, overviewSparkMinHeight))
		plot = rect
	}
	return container.New(layout.NewCustomPaddedLayout(spaceMD, spaceMD, 0, 0), plot)
}

// statusDot is the small filled circle in a panel header, colored by health.
func statusDot(kind statusKind) fyne.CanvasObject {
	dot := canvas.NewCircle(statusColor(kind))
	return container.NewGridWrap(fyne.NewSize(overviewDotSize, overviewDotSize), dot)
}

// newMetricPanel is the reusable Overview metric-card shell: a rounded surface
// card with a header row — uppercase title left, health dot right — above a
// caller-supplied content area.
func newMetricPanel(title string, state statusKind, content fyne.CanvasObject) fyne.CanvasObject {
	card := canvas.NewRectangle(palette.Surface)
	card.StrokeColor = palette.Border
	card.StrokeWidth = panelBorderWidth
	card.CornerRadius = theme.Size(sizeName.PanelRadius)

	header := container.NewHBox(
		vCenter(newColumnLabel(title)), layout.NewSpacer(), vCenter(statusDot(state)))
	body := container.New(layout.NewCustomPaddedLayout(spaceMD, 0, 0, 0), content)

	inner := newTightBorder(header, nil, nil, nil, body)
	padded := container.New(
		layout.NewCustomPaddedLayout(panelBodyPad, panelBodyPad, panelBodyPad, panelBodyPad), inner)
	return container.NewStack(card, padded)
}

// formatWhole renders a value with no decimals ("34") — shared by the percent
// readouts and the process-count axis.
func formatWhole(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64)
}

// formatPercentAxis labels a Y tick on a percentage axis ("50%").
func formatPercentAxis(v float64) string {
	return formatWhole(v) + labelUnitPercent
}

// formatRateText joins a bytes/sec rate's value and unit for a footer readout
// ("12.1 MB/s").
func formatRateText(bytesPerSec float64) string {
	value, unit := formatRate(bytesPerSec)
	return value + labelRateGap + unit
}
