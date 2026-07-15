package ui

// Recordings tab: an in-app viewer for the tracking-session CSVs the recorder
// writes (BZS253-77 follow-up). It reads a session back and plots it as a small
// set of panels that mirror the live metric tabs — Memory used on a fixed
// 0..total-RAM axis, Network rx/tx/total, Disk read/write/total — rather than a
// lone chart per raw column, so a recording reads like the app it came from. Each panel
// carries a legend and the metric's absolute min/max over the whole session.
// Loading is on demand through the platform's native open dialog; a recording is
// a static snapshot, so the tab has no per-tick refresh.
//
// Parsing lives in internal/recorder (Fyne-free); this file maps the parsed
// columns onto grouped charts. The group set targets the recorder's own schema;
// a column outside it is ignored, and a group whose metric is absent is skipped.

import (
	"errors"
	"image/color"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"github.com/ncruces/zenity"

	"github.com/josephheinz/system-monitor/internal/recorder"
	"github.com/josephheinz/system-monitor/internal/recorder/columns"
	"github.com/josephheinz/system-monitor/internal/series"
)

const (
	labelRecordingsPageTitle = "Recordings"
	labelLoadRecording       = "Load recording…"
	labelNoRecording         = "No recording loaded. Load a tracking CSV to view it."
	recordingDialogTitle     = "Open a recording"
)

// Recording panel titles not already named by a page-title const, plus the
// per-series legend labels and the min/max readout chrome.
const (
	labelRecSwap   = "Swap"
	labelRecDiskIO = "Disk I/O"

	labelRecTotal = "total"
	labelRecRx    = "rx"
	labelRecTx    = "tx"
	labelRecRead  = "read"
	labelRecWrite = "write"

	labelRecMin = "min "
	labelRecMax = " · max "
)

// Recordings viewer geometry.
const (
	recordingChartHeight = 140 // px; per-panel chart height
	recordingGridCols    = 2   // panels per row
)

// recordingScale selects both a chart's Y-axis options and the text formatter for
// its min/max readout.
type recordingScale int

const (
	scalePercent recordingScale = iota
	scaleBytes
	scaleRate
	scaleCount
)

// recordingGroup is one panel: a title, the columns it plots (in legend order),
// its value scale, whether to prepend a computed total line (the emphasized
// headline, as the Network/Disk tabs do), and whether to band the high-usage
// threshold (CPU only). The primary line — the total when present, else the first
// column — drives the min/max readout. ceilCol pins the Y axis to 0..max(ceilCol)
// instead of auto-scaling — Memory plots used against a fixed 0..total-RAM axis.
type recordingGroup struct {
	title   string
	cols    []string
	labels  []string // legend labels aligned with cols; empty ⇒ no legend
	scale   recordingScale
	total   bool
	band    bool
	ceilCol string // when set, Y axis is fixed 0..max(ceilCol), byte-scaled
}

// recordingGroups mirrors the live tabs: CPU (with its high-usage band), Memory
// used vs total, Swap, Network rx/tx with a total, Disk I/O read/write with a
// total, and the process count. Keyed by the shared schema consts
// (recorder/columns) so the grouping stays in sync with what the recorder
// writes — in the GUI and the headless binary alike.
var recordingGroups = []recordingGroup{
	{title: labelCPUPageTitle, cols: []string{columns.CPUPct}, scale: scalePercent, band: true},
	{title: labelMemoryPageTitle, cols: []string{columns.MemUsed}, scale: scaleBytes, ceilCol: columns.MemTotal},
	{title: labelRecSwap, cols: []string{columns.SwapUsed}, scale: scaleBytes},
	{title: labelNetworkPageTitle, cols: []string{columns.NetRx, columns.NetTx}, labels: []string{labelRecRx, labelRecTx}, scale: scaleRate, total: true},
	{title: labelRecDiskIO, cols: []string{columns.DiskRead, columns.DiskWrite}, labels: []string{labelRecRead, labelRecWrite}, scale: scaleRate, total: true},
	{title: labelProcessesPageTitle, cols: []string{columns.ProcCount}, scale: scaleCount},
}

// recordingsView is the tab: a header with a load action and the loaded file's
// name, over a body that swaps between an empty-state prompt and the panel grid.
type recordingsView struct {
	prefs    settings        // for the CPU high-usage threshold band (reuses BZS253-75)
	body     *fyne.Container // swappable center: empty state or the chart grid
	fileText *canvas.Text    // loaded file name shown in the header
}

func newRecordingsView(prefs settings) *recordingsView {
	return &recordingsView{
		prefs:    prefs,
		body:     container.NewStack(recordingsEmptyState()),
		fileText: newMeta(""),
	}
}

// object assembles the tab: a page head (title, loaded file name, load link)
// over the swappable body, inset by the shared tab padding.
func (v *recordingsView) object() fyne.CanvasObject {
	head := container.New(layout.NewCustomPaddedHBoxLayout(spaceLG),
		vCenter(newHeading(labelRecordingsPageTitle)),
		layout.NewSpacer(),
		vCenter(v.fileText),
		vCenter(newJumpLink(labelLoadRecording, v.load)),
	)
	padHead := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0), head)
	body := newTightBorder(padHead, nil, nil, nil, v.body)
	return container.New(layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
}

// load opens the native file picker and, on a chosen file, parses it and swaps
// in the chart grid. zenity blocks, so it runs on its own goroutine; the UI swap
// is marshalled back with fyne.Do. Cancel is silent; a real error is logged and
// the current view left as-is.
func (v *recordingsView) load() {
	go func() {
		path, err := zenity.SelectFile(
			zenity.Title(recordingDialogTitle),
			zenity.FileFilters{{Name: recordFilterName, Patterns: []string{recordFilterPattern}}},
		)
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				log.Printf("open dialog: %v", err)
			}
			return
		}
		rec, err := readRecording(path)
		if err != nil {
			log.Printf("read recording: %v", err)
			return
		}
		fyne.Do(func() { v.show(rec, filepath.Base(path)) })
	}()
}

// show replaces the body with the parsed recording's chart grid and names the
// loaded file in the header. Runs on the UI goroutine (called via fyne.Do).
func (v *recordingsView) show(rec *recorder.Recording, name string) {
	v.fileText.Text = name
	v.fileText.Refresh()
	v.body.Objects = []fyne.CanvasObject{v.charts(rec)}
	v.body.Refresh()
}

// readRecording opens and parses a recording file, closing it before returning.
func readRecording(path string) (*recorder.Recording, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return recorder.Read(f)
}

// charts builds the scrollable grid, one panel per group whose metric the
// recording actually contains.
func (v *recordingsView) charts(rec *recorder.Recording) fyne.CanvasObject {
	var cells []fyne.CanvasObject
	for _, g := range recordingGroups {
		if columnValues(rec, g.cols[0]) == nil {
			continue // this recording doesn't carry the group's metric
		}
		cells = append(cells, v.panel(g, rec))
	}
	return container.NewVScroll(container.NewGridWithColumns(recordingGridCols, cells...))
}

// panel builds one group's titled chart: its lines (with a computed total and
// the CPU threshold band where the group asks for them), a legend for its named
// series, and the primary series' absolute min/max in the header.
func (v *recordingsView) panel(g recordingGroup, rec *recorder.Recording) fyne.CanvasObject {
	opts := scaleAxis(g.scale)
	// Fix the axis to 0..physical-RAM (the ceiling column's max) so the used
	// line reads against total capacity rather than an auto-scaled window.
	if ceil := maxValue(columnValues(rec, g.ceilCol)); g.ceilCol != "" && ceil > 0 {
		opts = []lineChartOption{fixedRange(0, ceil), valueFormat(formatBytesAxis)}
	}
	if g.band {
		// Band the CPU high-usage zone in the critical-red tint, reusing the user's
		// configured CPU alert threshold (BZS253-75) so the highlight and the
		// notifications agree.
		opts = append(opts, thresholdBand(float64(v.prefs.alertThreshold(thresholdCPU)), palette.RedDim))
	}
	// Hover shows the sample's absolute timestamp (and value) — the whole point of
	// reviewing a recording after the fact.
	opts = append(opts, hoverReadout(recordingHover(rec)))
	chart := newLineChart(opts...)

	lines := recordingLines(g, rec)
	for _, ln := range lines {
		vals := ln.vals
		if ln.emph {
			chart.addSeries(series.SourceFunc(func() []float64 { return vals }), emphasized())
			continue
		}
		chart.addSeries(series.SourceFunc(func() []float64 { return vals }), seriesColor(ln.col))
	}

	var entries []legendEntry
	for _, ln := range lines {
		if ln.label != "" {
			entries = append(entries, legendEntry{label: ln.label, col: ln.col})
		}
	}
	readout := recordingRangeReadout(lines[0].vals, scaleText(g.scale))
	return newPanel(g.title, recordingTrailing(entries, readout), fixedHeight(chart, recordingChartHeight))
}

// plotLine is one resolved series for a panel: its samples, its legend color, and
// whether it is the emphasized headline line.
type plotLine struct {
	label string
	vals  []float64
	col   color.Color
	emph  bool
}

// recordingLines resolves a group into its plotted lines: an emphasized total
// (sum of the group's columns) first when requested, then each column in a
// categorical hue — except a single-column group's lone line, which is itself the
// emphasized headline. Colors are assigned here so the legend matches the strokes.
func recordingLines(g recordingGroup, rec *recorder.Recording) []plotLine {
	var lines []plotLine
	if g.total {
		lines = append(lines, plotLine{label: labelRecTotal, vals: sumColumns(rec, g.cols), col: palette.Accent, emph: true})
	}
	for i, col := range g.cols {
		var hue color.Color = palette.Series[(i+1)%len(palette.Series)]
		emph := false
		if !g.total && i == 0 {
			hue, emph = palette.Accent, true
		}
		label := ""
		if i < len(g.labels) {
			label = g.labels[i]
		}
		lines = append(lines, plotLine{label: label, vals: columnValues(rec, col), col: hue, emph: emph})
	}
	return lines
}

// recordingTrailing composes a panel header's right side: the legend (when the
// group has named series) beside the min/max readout.
func recordingTrailing(entries []legendEntry, readout string) fyne.CanvasObject {
	caption := newMeta(readout)
	if len(entries) == 0 {
		return caption
	}
	return container.New(layout.NewCustomPaddedHBoxLayout(legendItemGap),
		vCenter(newLegend(entries...)), vCenter(caption))
}

// recordingRangeReadout renders a series' absolute min and max over the whole
// recording, formatted for the metric's scale ("min 12G · max 51G").
func recordingRangeReadout(vals []float64, format func(float64) string) string {
	lo, hi := minMax(vals)
	return labelRecMin + format(lo) + labelRecMax + format(hi)
}

// scaleAxis picks a chart's Y range and axis formatter for a value scale.
func scaleAxis(s recordingScale) []lineChartOption {
	switch s {
	case scalePercent:
		return []lineChartOption{fixedRange(0, percentMax), valueFormat(formatPercentAxis)}
	case scaleCount:
		return []lineChartOption{autoRange(), valueFormat(formatCompact)}
	default: // bytes and rates share a byte-magnitude axis
		return []lineChartOption{autoRange(), valueFormat(formatBytesAxis)}
	}
}

// scaleText returns the text formatter for a scale's min/max readout.
func scaleText(s recordingScale) func(float64) string {
	switch s {
	case scalePercent:
		return formatPercent
	case scaleRate:
		return formatRateText
	case scaleCount:
		return formatWhole
	default:
		return func(v float64) string { return formatBytesShort(uint64(v)) }
	}
}

// columnValues returns the samples for a column header, or nil when the recording
// doesn't contain it.
func columnValues(rec *recorder.Recording, col string) []float64 {
	for i, c := range rec.Columns {
		if c == col {
			return rec.Series[i]
		}
	}
	return nil
}

// sumColumns adds the given columns element-wise into a fresh slice — the
// computed "total" line for a multi-series panel. Absent columns contribute
// nothing; a shorter column is honored defensively.
func sumColumns(rec *recorder.Recording, cols []string) []float64 {
	var out []float64
	for _, col := range cols {
		vals := columnValues(rec, col)
		if out == nil {
			out = make([]float64, len(vals))
		}
		for i := range vals {
			if i < len(out) {
				out[i] += vals[i]
			}
		}
	}
	return out
}

// maxValue returns the largest value in vals (0 when empty) — the fixed-axis
// ceiling for a group's ceilCol.
func maxValue(vals []float64) float64 {
	_, hi := minMax(vals)
	return hi
}

// minMax returns the smallest and largest values in vals, or 0,0 when empty.
func minMax(vals []float64) (lo, hi float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	lo, hi = vals[0], vals[0]
	for _, v := range vals {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return lo, hi
}

// recordingHoverTimeLayout renders a hovered sample's wall-clock time. Time of
// day suffices — a session is short — and it honors the offset the recorder wrote.
const recordingHoverTimeLayout = "15:04:05"

// recordingHover maps a sample index to its absolute timestamp for the chart's
// hover tooltip. Out-of-range indices yield an empty label rather than panicking.
func recordingHover(rec *recorder.Recording) func(int) string {
	ts := rec.Timestamps
	return func(i int) string {
		if i < 0 || i >= len(ts) {
			return ""
		}
		return ts[i].Format(recordingHoverTimeLayout)
	}
}

// recordingsEmptyState is the pre-load prompt centered in the body.
func recordingsEmptyState() fyne.CanvasObject {
	return container.NewCenter(newMeta(labelNoRecording))
}

// fixedHeight forces obj to a minimum height while letting it fill the available
// width — a transparent sizing rectangle stacked behind it. Used to give each
// otherwise-unbounded chart a stable height in the grid.
func fixedHeight(obj fyne.CanvasObject, h float32) fyne.CanvasObject {
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(0, h))
	return container.NewStack(spacer, obj)
}
