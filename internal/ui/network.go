package ui

// Network tab content (BZS253-55), laid out to match
// tab-05-network-three-line-bandwidth.html and
// docs/pitch/5 _ Network _ three-line bandwidth.png:
//
//	page head   — "Network" title
//	stat row    — four metric cards: download, upload, total (live latest) and
//	              peak (max over the window)
//	chart pane  — bandwidth panel: the upload/download/total line chart filling
//	              the remaining height
//
// The chart is the same generic lineChart as the Disk I/O pane: an auto-scaled
// byte/sec Y axis over the history window, the emphasized total line plus the
// upload and download series in their categorical hues. Each stat card's swatch
// matches its line so card, chart, and legend stay consistent. app.go adapts the
// monitor.NetworkCollector's rate buffers behind the netSources seam.

import (
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/series"
)

const (
	labelNetworkPageTitle = "Network"
	labelBandwidthPanel   = "Bandwidth"
	labelLegendUpload     = "upload"
	labelLegendDownload   = "download"
)

// Stat-card labels (the wireframe's UPPERCASE metric-square headers).
const (
	labelStatDownload = "DOWNLOAD"
	labelStatUpload   = "UPLOAD"
	labelStatTotal    = "TOTAL"
	labelStatPeak     = "PEAK"
)

// Categorical-palette indices for the upload / download series (total is the
// emphasized accent line) — the same two secondary hues the Disk I/O chart
// uses: c2 (cyan) and c3 (violet).
const (
	netUploadSeriesIndex   = 1 // c2
	netDownloadSeriesIndex = 2 // c3
)

// Stat-card geometry (the wireframe's metric squares above the chart).
const (
	statCardMinHeight = 16 * spaceSM // 64; metric-card min height
	statCardRowGap    = spaceMD      // 8; gap between a card's title bar and value
	// statUnitBaselinePad lifts the small unit so its baseline sits on the big
	// value's baseline. canvas.Text bottom-aligns to its box, but the 26px value
	// has a deeper descent than the 11px unit; this offsets that difference.
	statUnitBaselinePad = 4 // px
)

// rateSuffixes are the per-second unit suffixes for formatRate, in ascending
// 1024 powers.
var rateSuffixes = []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s", "PB/s"}

// formatRate splits a bytes/sec rate into a display value and its unit
// ("4.1", "MB/s") for the stat cards' big-number + small-unit styling. It scales
// by the same 1024 step as formatBytesShort, keeping one decimal for scaled
// values under byteDecimalLimit.
func formatRate(bytesPerSec float64) (value, unit string) {
	if bytesPerSec < 0 {
		bytesPerSec = 0
	}
	v := bytesPerSec
	step := 0
	for v >= bytesPerStep && step < len(rateSuffixes)-1 {
		v /= bytesPerStep
		step++
	}
	decimals := 0
	if step > 0 && v < byteDecimalLimit {
		decimals = 1
	}
	return strconv.FormatFloat(v, 'f', decimals, 64), rateSuffixes[step]
}

// netSources bundles the bandwidth chart's three series: total (emphasized)
// plus the upload and download rates. The zero value means the network metric
// isn't wired and the tab keeps its placeholder.
type netSources struct {
	total    series.Source
	upload   series.Source
	download series.Source
}

// wired reports whether all three series were adapted.
func (s netSources) wired() bool {
	return s.total != nil && s.upload != nil && s.download != nil
}

// latestSample reduces a window to its newest value — the stat cards' live
// reading.
func latestSample(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	return vals[len(vals)-1]
}

// peakSample reduces a window to its maximum — the Peak card's reading over the
// history span.
func peakSample(vals []float64) float64 {
	peak := 0.0
	for _, v := range vals {
		if v > peak {
			peak = v
		}
	}
	return peak
}

// statCard is one metric square above the chart: a swatch+label title bar over a
// large rate readout (value + small unit). It re-reads its series on every
// refresh, reducing the window to a single number (latest or peak).
type statCard struct {
	src    series.Source
	reduce func([]float64) float64
	value  *canvas.Text
	unit   *canvas.Text
	card   fyne.CanvasObject
}

// newStatCard builds a card titled label (with a matching swatch) that displays
// reduce(src.Values()) formatted as a bandwidth rate.
func newStatCard(label string, swatch color.Color, src series.Source, reduce func([]float64) float64) *statCard {
	c := &statCard{
		src:    src,
		reduce: reduce,
		value:  newMetricValue(""),
		unit:   styledText("", font.MonoRegular, theme.SizeNameCaptionText, palette.Text3),
	}

	title := newLegend(legendEntry{label: label, col: swatch})
	// Bottom-align the unit, lifted by statUnitBaselinePad so it shares the
	// value's baseline rather than its (deeper) box bottom.
	liftedUnit := container.New(
		layout.NewCustomPaddedLayout(0, statUnitBaselinePad, 0, 0), c.unit)
	unitBox := container.New(layout.NewCustomPaddedVBoxLayout(0), layout.NewSpacer(), liftedUnit)
	valueRow := container.New(layout.NewCustomPaddedHBoxLayout(spaceSM), c.value, unitBox)
	body := container.New(layout.NewCustomPaddedVBoxLayout(statCardRowGap), title, valueRow)

	bg := canvas.NewRectangle(palette.Surface)
	bg.StrokeColor = palette.Border
	bg.StrokeWidth = panelBorderWidth
	bg.CornerRadius = theme.Size(sizeName.PanelRadius)
	bg.SetMinSize(fyne.NewSize(0, statCardMinHeight))
	padded := container.New(
		layout.NewCustomPaddedLayout(panelBodyPad, panelBodyPad, panelBodyPad, panelBodyPad), body)

	c.card = container.NewStack(bg, padded)
	c.refresh()
	return c
}

// refresh re-reads the series and updates the card's value and unit.
func (c *statCard) refresh() {
	value, unit := formatRate(c.reduce(c.src.Values()))
	c.value.Text = value
	c.unit.Text = unit
	c.value.Refresh()
	c.unit.Refresh()
}

// netView is the Network tab: page head, the stat-card row, and the full-height
// bandwidth chart pane. Build with newNetworkView and drive live updates through
// refresh.
type netView struct {
	chart *lineChart
	cards []*statCard
}

// newNetworkView builds the Network tab content from the bandwidth series.
func newNetworkView(src netSources) *netView {
	chart := newLineChart(
		autoRange(),
		valueFormat(formatBytesAxis),
		window(metrics.HistoryCapacity),
		timeAxis(historySpan()),
	)
	chart.addSeries(src.total, emphasized())
	chart.addSeries(src.upload, seriesColor(palette.Series[netUploadSeriesIndex]))
	chart.addSeries(src.download, seriesColor(palette.Series[netDownloadSeriesIndex]))

	return &netView{
		chart: chart,
		cards: []*statCard{
			newStatCard(labelStatDownload, palette.Series[netDownloadSeriesIndex], src.download, latestSample),
			newStatCard(labelStatUpload, palette.Series[netUploadSeriesIndex], src.upload, latestSample),
			newStatCard(labelStatTotal, palette.Accent, src.total, latestSample),
			newStatCard(peakCardLabel(), palette.Text3, src.total, peakSample),
		},
	}
}

// peakCardLabel composes "PEAK (1 min)" from the actual history window so the
// card stays truthful if the buffer capacity changes.
func peakCardLabel() string {
	return labelStatPeak + " (" + formatSpan(historySpan()) + ")"
}

// object assembles the tab: page head and stat row pinned on top, then the
// bandwidth panel filling the remaining height, inset by the shared tab padding.
func (v *netView) object() fyne.CanvasObject {
	head := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0), v.pageHead())
	cards := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0), v.cardsRow())
	legend := newLegend(
		legendEntry{label: labelLegendTotal, col: palette.Accent},
		legendEntry{label: labelLegendUpload, col: palette.Series[netUploadSeriesIndex]},
		legendEntry{label: labelLegendDownload, col: palette.Series[netDownloadSeriesIndex]},
	)
	panel := newPanel(historyTitle(labelBandwidthPanel), legend, v.chart)
	body := newTightBorder(vStackTight(head, cards), nil, nil, nil, panel)
	return container.New(layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
}

// pageHead is the wireframe's sm-pagehead row: the tab title.
func (v *netView) pageHead() fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedHBoxLayout(spaceLG),
		vCenter(newHeading(labelNetworkPageTitle)))
}

// cardsRow lays the stat cards out as equal-width columns separated by the tab
// gap, matching the wireframe's four-square row.
func (v *netView) cardsRow() fyne.CanvasObject {
	panes := make([]weightedPane, len(v.cards))
	for i, c := range v.cards {
		panes[i] = weightedPane{object: c.card, weight: 1}
	}
	return newWeightedHBox(tabPad, panes...)
}

// refresh redraws the chart and the live stat cards. It touches the canvas, so a
// background poller must marshal it onto the UI goroutine (fyne.Do).
func (v *netView) refresh() {
	v.chart.Refresh()
	for _, c := range v.cards {
		c.refresh()
	}
}
