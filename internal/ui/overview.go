package ui

// Overview tab content: a 3×2 grid of self-contained metric panels (adapted
// from the wireframe's tab-01-overview-panel-grid — dropping the metrics this
// app doesn't track), giving an at-a-glance health view of the machine. This
// card (BZS253-62) builds the grid and the reusable metric-
// panel shell; the values shown are static placeholders mirroring the
// wireframe, and the sparkline area is reserved (empty plot region) — live
// numbers and the actual sparklines arrive in subsequent cards.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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

// Overview panel geometry.
const (
	overviewSparkMinHeight = 64 // px; reserved height for the future sparkline
	overviewDotSize        = 8  // px; status indicator dot
	overviewColumns        = 3  // panels per row (3×2 grid)
)

// overviewMetric is one panel's static content.
// ponytail: these values mirror the wireframe's sample readouts and are
// replaced by live ring-buffer data in later cards — this card is layout only.
type overviewMetric struct {
	title     string
	value     string
	unit      string
	state     statusKind
	footLeft  string
	footRight string
}

var overviewMetrics = []overviewMetric{
	{labelCPUPanel, "34", "%", status.Healthy, "12 cores", "peak 71%"},
	{labelMemoryPanel, "11.2", "/32 GB", status.Healthy, "35% used", "cache 6.1G"},
	{labelDiskIOPanel, "18.4", "MB/s", status.Warning, "r 12.1", "w 6.3"},
	{labelNetworkPanel, "4.9", "MB/s", status.Healthy, "↓ 4.1", "↑ 0.8"},
	{labelSwapPanel, "0.4", "/8 GB", status.Healthy, "5% used", "no pressure"},
	{labelProcessesPanel, "187", "procs", status.Healthy, "6 running", "2.1k threads"},
}

// newOverview builds the Overview tab: a page header above a 3-column grid of
// metric panels (two equal-height rows). Equal-weight rows and columns make the
// grid resize gracefully with the window.
func newOverview() fyne.CanvasObject {
	rows := make([]weightedPane, 0, 2)
	for start := 0; start < len(overviewMetrics); start += overviewColumns {
		end := start + overviewColumns
		if end > len(overviewMetrics) {
			end = len(overviewMetrics)
		}
		rows = append(rows, weightedPane{object: overviewRow(overviewMetrics[start:end]), weight: 1})
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

// overviewRow lays a slice of metrics out as equal-width panels in one row.
func overviewRow(ms []overviewMetric) fyne.CanvasObject {
	panes := make([]weightedPane, 0, overviewColumns)
	for _, m := range ms {
		panes = append(panes, weightedPane{object: overviewPanel(m), weight: 1})
	}
	// Pad a short final row with empty cells so its panels keep the same column
	// width as the full rows above (otherwise 3 panels would stretch wider).
	for len(panes) < overviewColumns {
		panes = append(panes, weightedPane{object: layout.NewSpacer(), weight: 1})
	}
	return newWeightedHBox(tabPad, panes...)
}

// newMetricPanel is the reusable Overview metric-card shell: a rounded surface
// card (theme surface fill + border token, no hardcoded colors) with a header
// row — uppercase title on the left, health dot on the right — above a content
// area. Callers build their own content (a value readout, a sparkline, …) and
// slot it in here, so every Overview card reads identically and future metric
// widgets drop straight into the content slot.
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

// overviewPanel composes one metric's content — the big value with its unit, a
// reserved sparkline area that fills the slack, and a footer meta row pinned to
// the bottom — and slots it into the shared metric-panel shell.
func overviewPanel(m overviewMetric) fyne.CanvasObject {
	value := container.New(layout.NewCustomPaddedHBoxLayout(spaceMD),
		newMetricValue(m.value), vCenter(newTableText(m.unit)))
	footer := container.NewHBox(
		newMeta(m.footLeft), layout.NewSpacer(), newMeta(m.footRight))

	spark := canvas.NewRectangle(palette.PlotBG)
	spark.CornerRadius = theme.Size(sizeName.PanelRadius)
	spark.SetMinSize(fyne.NewSize(0, overviewSparkMinHeight))
	sparkArea := container.New(layout.NewCustomPaddedLayout(spaceMD, spaceMD, 0, 0), spark)

	content := newTightBorder(value, footer, nil, nil, sparkArea)
	return newMetricPanel(m.title, m.state, content)
}

// statusDot is the small filled circle in a panel header, colored by health.
func statusDot(kind statusKind) fyne.CanvasObject {
	dot := canvas.NewCircle(statusColor(kind))
	return container.NewGridWrap(fyne.NewSize(overviewDotSize, overviewDotSize), dot)
}
