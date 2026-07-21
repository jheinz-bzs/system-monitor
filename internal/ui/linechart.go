package ui

// A generic, design-system-styled time-series line chart, built as a custom
// Fyne widget (like navItem in sidebar.go) because no Fyne widget expresses the
// chart language documented in docs/wireframe designs/design-system-06-chart-
// language.html.
//
// The chart is deliberately decoupled from any collector and from the element
// type of the metric history:
//
//   - Decoupled from the type. A series is fed by a Source (Values() []float64).
//     The generic helpers series.SourceFrom / series.SourceOf adapt ANY ring
//     buffer or snapshot func whose elements are a numeric kind —
//     *ringbuffer.RingBuffer[float64] (CPU percent), MemoryCollector.Used
//     (memory / network bytes), etc. — by
//     converting samples to float64 once, at the chart boundary. The chart core
//     does all pixel math in float64. (float64 represents integers exactly up to
//     2^53, far above any realistic byte count, so the conversion is lossless in
//     practice.)
//
//   - Decoupled from units and scale. The Y range is either fixed (e.g. 0–100
//     for a percentage) or auto-scaled to the data (e.g. bytes), and tick labels
//     are produced by a caller-supplied formatter, so the same widget serves a
//     "%" axis and a "GB" axis. See the range/format options.
//
// Design language (translated from the wireframe into Fyne canvas primitives):
//   - plot area filled with plot-bg, framed by a 1px border
//   - horizontal gridlines in border (#262e3a); vertical gridlines quieter, in
//     surface-2 (#1b212b)
//   - one emphasized series (accent, 2.2px, opaque) for the headline line, the
//     rest secondary (categorical hue, 1px, ~55% opacity)
//   - axis tick labels in muted mono (text-3), the meta typographic role
//   - time axis runs left (oldest) → right (now)
//
// Each series is stroked into a single canvas.Raster with an anti-aliased
// rasterizer (golang.org/x/image/vector), one coverage pass per series. That
// yields a smooth line of uniform opacity; the simpler approach of stacking many
// translucent canvas.Line segments compounds alpha at every overlap and beads
// the line. The grid, frame, and axis labels stay as plain canvas objects.
//
// Live updates re-run the layout and regenerate the raster on Refresh. (Refresh
// touches the canvas, so a background poller must marshal the call onto the UI
// goroutine via fyne.Do; reading the Source itself is safe because RingBuffer is
// concurrency-safe.)

import (
	"image"
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/image/vector"

	"github.com/josephheinz/system-monitor/internal/series"
)

// Chart geometry. Line weights and the secondary-line opacity come from the
// chart-language spec; gutters/min-size are translated to the 4px grid.
const (
	chartPrimaryWidth   = 2.2     // emphasized series stroke width
	chartSecondaryWidth = 1.0     // secondary series stroke width
	chartSecondaryAlpha = 0x8c    // ~55% opacity for secondary series
	chartYTickCount     = 5       // horizontal gridlines / Y labels (incl. ends)
	chartXGridCount     = 6       // vertical gridline columns
	// chartAutoRangePad is the headroom added above and below an auto-scaled
	// series (as a fraction of its span) so the line never glues to the frame.
	chartAutoRangePad = 0.1
	chartLabelGap       = spaceSM // 4; gap between a tick label and the plot
	chartMinWidth       = 60 * spaceSM
	chartMinHeight      = 30 * spaceSM
)

// Hover overlay geometry (the point tooltip shown while the pointer is over a
// hover-interactive chart).
const (
	hoverMarkerSize = 6       // px; hovered-point dot diameter (off-grid)
	hoverTipPad     = spaceSM // 4; tooltip inner padding
	hoverTipGap     = spaceMD // 8; offset from the point to the tooltip box
	hoverTipRadius  = spaceXS // 2; tooltip corner radius
)

// yRange describes how the chart derives its vertical scale.
type yRange struct {
	fixed    bool
	min, max float64
}

// lineChartOption configures a lineChart at construction.
type lineChartOption func(*lineChart)

// fixedRange pins the Y axis to [min, max] (e.g. 0–100 for a percentage). Use
// for metrics with a known, stable domain.
func fixedRange(min, max float64) lineChartOption {
	return func(c *lineChart) { c.yr = yRange{fixed: true, min: min, max: max} }
}

// autoRange scales the Y axis to the data each refresh (rounded to nice tick
// values). This is the default; the option exists for explicitness at call
// sites.
func autoRange() lineChartOption {
	return func(c *lineChart) { c.yr = yRange{} }
}

// valueFormat sets the formatter for Y tick labels, e.g. percent or bytes. The
// default renders a compact decimal.
func valueFormat(f func(float64) string) lineChartOption {
	return func(c *lineChart) {
		if f != nil {
			c.format = f
		}
	}
}

// window fixes the number of samples the X axis spans, so points keep a stable
// horizontal spacing as the buffer fills (newest pinned to the right edge,
// older points stepping left). When unset (0), points fill the full plot width.
func window(samples int) lineChartOption {
	return func(c *lineChart) {
		if samples > 0 {
			c.window = samples
		}
	}
}

// timeAxis labels the X axis with elapsed time across span, oldest (−span) on
// the left to "now" on the right. When unset, the chart draws vertical
// gridlines without time labels.
func timeAxis(span time.Duration) lineChartOption {
	return func(c *lineChart) { c.span = span }
}

// thresholdBandSpec shades the plot wherever the driving series sits at or above
// threshold — the "high usage" highlight.
type thresholdBandSpec struct {
	threshold float64
	fill      color.Color
}

// hoverReadout makes the chart hover-interactive: as the pointer moves over the
// plot, the chart snaps a crosshair + marker to the nearest sample of the driving
// series and shows a tooltip of that sample's value and the label f returns for
// its index (the Recordings tab passes the sample's absolute timestamp). Without
// this option the chart isn't hover-interactive.
func hoverReadout(f func(index int) string) lineChartOption {
	return func(c *lineChart) { c.hover = f }
}

// thresholdBand shades the vertical span of the plot wherever the first (driving)
// series' value is at or above threshold, so an over-threshold stretch reads at a
// glance (the highlight from the BZS253-77 follow-up). fill should be translucent
// so the line stays visible on top — pass a design token such as palette.RedDim.
func thresholdBand(threshold float64, fill color.Color) lineChartOption {
	return func(c *lineChart) { c.band = &thresholdBandSpec{threshold: threshold, fill: fill} }
}

// chartSeries is one plotted line. Construct it with addSeries; style it with
// the series options.
type chartSeries struct {
	src     series.Source
	stroke  color.Color
	width   float32
	visible bool

	// set when the caller hasn't chosen a color, so it's assigned from
	// seriesPalette in declaration order at draw time.
	autoColor bool
}

// seriesOption configures a series when it is added.
type seriesOption func(*chartSeries)

// emphasized marks a series as the headline line: accent color, thicker stroke,
// full opacity — visually distinct from the secondary series. Used for the
// "overall" line on a multi-line chart.
func emphasized() seriesOption {
	return func(s *chartSeries) {
		s.stroke = palette.Accent
		s.width = chartPrimaryWidth
		s.autoColor = false
	}
}

// seriesColor overrides a series' line color. Without it, secondary series take
// the next categorical hue.
func seriesColor(c color.Color) seriesOption {
	return func(s *chartSeries) {
		s.stroke = c
		s.autoColor = false
	}
}

// lineChart is a multi-series time-series chart widget. The zero value is not
// usable; build one with newLineChart.
type lineChart struct {
	widget.BaseWidget

	series  []*chartSeries
	yr      yRange
	format  func(float64) string
	window  int
	span    time.Duration
	stacked bool
	band    *thresholdBandSpec // nil = no threshold highlight
	hover   func(index int) string // sample→label for the hover tooltip; nil = not interactive

	renderer *lineChartRenderer // captured in CreateRenderer, for hover hit-testing
}

// newLineChart returns a chart configured by opts. Default Y range is
// auto-scaled with a compact decimal formatter; add series with addSeries.
func newLineChart(opts ...lineChartOption) *lineChart {
	c := &lineChart{format: formatCompact}
	for _, opt := range opts {
		opt(c)
	}
	c.ExtendBaseWidget(c)
	return c
}

// addSeries adds a line fed by src and returns it so the caller can keep a
// handle (e.g. to toggle visibility). Secondary series are auto-colored from
// the categorical palette in the order they're added.
func (c *lineChart) addSeries(src series.Source, opts ...seriesOption) *chartSeries {
	s := &chartSeries{src: src, width: chartSecondaryWidth, visible: true, autoColor: true}
	for _, opt := range opts {
		opt(s)
	}
	c.series = append(c.series, s)
	c.Refresh()
	return s
}

// setVisible shows or hides a series without removing it. Hidden series keep
// their palette slot, so colors stay stable across toggles.
func (s *chartSeries) setVisible(v bool) { s.visible = v }

func (c *lineChart) CreateRenderer() fyne.WidgetRenderer {
	r := &lineChartRenderer{chart: c}
	r.bg = canvas.NewRectangle(palette.PlotBG)
	r.border = canvas.NewRectangle(color.Transparent)
	r.border.StrokeColor = palette.Border
	r.border.StrokeWidth = 1
	r.series = canvas.NewRaster(r.renderSeries)

	for i := 0; i < chartXGridCount-1; i++ {
		r.vGrid = append(r.vGrid, newGridLine(palette.Surface2))
	}
	for i := 0; i < chartYTickCount; i++ {
		r.hGrid = append(r.hGrid, newGridLine(palette.Border))
		r.yLabels = append(r.yLabels, newMeta(""))
	}

	// Hover overlay, built once and hidden until the pointer enters the plot.
	r.crosshair = newGridLine(palette.BorderStrong)
	r.marker = canvas.NewCircle(palette.Accent)
	r.tipBG = canvas.NewRectangle(palette.Surface3)
	r.tipBG.StrokeColor = palette.Border
	r.tipBG.StrokeWidth = 1
	r.tipBG.CornerRadius = hoverTipRadius
	// Larger than axis meta so the readout is easily legible: 12px table size, the
	// value in medium/primary over the label in regular/secondary.
	r.tipVal = styledText("", font.MonoMedium, sizeName.TableText, palette.Text)
	r.tipLabel = styledText("", font.MonoRegular, sizeName.TableText, palette.Text2)
	r.hideHover()

	c.renderer = r
	return r
}

var (
	_ desktop.Hoverable  = (*lineChart)(nil)
	_ desktop.Cursorable = (*lineChart)(nil)
)

// MouseIn / MouseMoved / MouseOut implement desktop.Hoverable: on a
// hover-interactive chart they snap the overlay to the nearest sample; on a plain
// chart (no hoverReadout) they no-op.
func (c *lineChart) MouseIn(ev *desktop.MouseEvent) { c.MouseMoved(ev) }

func (c *lineChart) MouseMoved(ev *desktop.MouseEvent) {
	if c.hover == nil || c.renderer == nil {
		return
	}
	c.renderer.showHover(ev.Position)
}

func (c *lineChart) MouseOut() {
	if c.renderer != nil {
		c.renderer.hideHover()
	}
}

// Cursor implements desktop.Cursorable — a crosshair over a hover-interactive
// plot, the default pointer otherwise.
func (c *lineChart) Cursor() desktop.Cursor {
	if c.hover == nil {
		return desktop.DefaultCursor
	}
	return desktop.CrosshairCursor
}

// newGridLine builds a 1px gridline in the given color.
func newGridLine(c color.Color) *canvas.Line {
	l := canvas.NewLine(c)
	l.StrokeWidth = 1
	return l
}

type lineChartRenderer struct {
	chart *lineChart

	bg      *canvas.Rectangle
	border  *canvas.Rectangle
	vGrid   []*canvas.Line
	hGrid   []*canvas.Line
	yLabels []*canvas.Text
	xLabels []*canvas.Text

	// All series are drawn into one raster, stroked with an anti-aliased
	// rasterizer. A single coverage pass per series gives a smooth line of
	// uniform opacity — stacking many translucent canvas.Line segments instead
	// compounds alpha at every overlap and beads the line. lines holds the
	// plot-local polylines to stroke on the next generation; fills holds the
	// band polygons drawn beneath them in stacked mode; plot is the box the
	// raster fills (cached so the generator can scale DP → device pixels).
	series *canvas.Raster
	lines  []polyline
	fills  []bandFill
	bands  []bandFill // threshold highlight rectangles, drawn beneath the lines
	plot   chartBox

	// Hover overlay, shown only while the pointer is over a hover-interactive
	// plot: a crosshair at the sample, a marker on the driving line, and a
	// tooltip of the value + its label. hoverVals is that driving series' samples,
	// index-aligned with lines[0].pts, so a snapped index reads its own value.
	crosshair *canvas.Line
	marker    *canvas.Circle
	tipBG     *canvas.Rectangle
	tipVal    *canvas.Text
	tipLabel  *canvas.Text
	hoverVals []float64

	// Buffers reused across frames so a 1 Hz refresh doesn't churn a fresh
	// image + a rasterizer-per-series every tick (the dominant memory pressure
	// — see docs/performance/memory-footprint-plan.md, Fix A). img reallocates
	// only on a size/scale change; ras is reset between series.
	img *image.RGBA
	ras *vector.Rasterizer

	size fyne.Size
}

// polyline is one resolved series ready to stroke: plot-local points plus its
// pixel width and color.
type polyline struct {
	pts   []fyne.Position
	width float32
	col   color.Color
}

func (r *lineChartRenderer) Layout(size fyne.Size) {
	r.size = size
	r.arrange()
}

func (r *lineChartRenderer) Refresh() {
	r.arrange()
	canvas.Refresh(r.chart)
}

// arrange recomputes the whole scene for the current size and data: plot box,
// gridlines, tick labels, and every series polyline. It reads each Source once.
func (r *lineChartRenderer) arrange() {
	size := r.size
	if size.Width <= 0 || size.Height <= 0 {
		return
	}

	// Pull data first; the left gutter depends on the widest Y label, which
	// depends on the resolved range. In stacked mode the plotted values are the
	// cumulative band boundaries, so the transform happens before the range is
	// resolved.
	data := make([][]float64, len(r.chart.series))
	for i, s := range r.chart.series {
		if s.visible {
			data[i] = s.src.Values()
		}
	}
	if r.chart.stacked {
		data = stackSeries(data)
	}
	lo, hi := r.chart.resolveRange(data)

	left := r.layoutYLabels(lo, hi)
	bottom := r.layoutXLabels()

	plot := chartBox{
		x:      left,
		y:      0,
		width:  size.Width - left,
		height: size.Height - bottom,
	}
	r.layoutPlotFrame(plot)
	r.layoutGrid(plot)

	// Resolve the series into plot-local polylines (plus band fills when
	// stacked), then (re)generate the raster that draws them.
	r.plot = plot
	if r.chart.stacked {
		r.lines, r.fills = r.buildStack(plot, lo, hi, data)
	} else {
		r.lines, r.fills = r.buildLines(plot, lo, hi, data), nil
	}
	r.bands = r.buildBands(plot, data)
	r.hoverVals = firstVisibleSeries(data) // driving series for the hover tooltip
	r.series.Resize(fyne.NewSize(plot.width, plot.height))
	r.series.Move(fyne.NewPos(plot.x, plot.y))
	r.series.Refresh()
}

// layoutYLabels positions the Y tick labels (top→bottom: hi→lo) and returns the
// left gutter width needed to fit the widest one.
func (r *lineChartRenderer) layoutYLabels(lo, hi float64) float32 {
	var widest float32
	vals := tickValues(lo, hi, chartYTickCount)
	for i, lbl := range r.yLabels {
		lbl.Text = r.chart.format(vals[i])
		lbl.Refresh()
		if w := lbl.MinSize().Width; w > widest {
			widest = w
		}
	}
	return widest + chartLabelGap
}

// layoutXLabels positions the optional time-axis labels along the bottom and
// returns the bottom gutter height. With no time axis, the gutter is zero.
func (r *lineChartRenderer) layoutXLabels() float32 {
	if r.chart.span <= 0 {
		for _, l := range r.xLabels {
			l.Hide()
		}
		return 0
	}
	// One label per vertical division boundary (chartXGridCount + 1 ticks).
	r.ensureXLabels(chartXGridCount + 1)
	h := r.xLabels[0].MinSize().Height
	for _, l := range r.xLabels {
		l.Show()
	}
	return h + chartLabelGap
}

// ensureXLabels grows the X label pool to n, building any missing labels.
func (r *lineChartRenderer) ensureXLabels(n int) {
	for len(r.xLabels) < n {
		r.xLabels = append(r.xLabels, newMeta(""))
	}
}

func (r *lineChartRenderer) layoutPlotFrame(plot chartBox) {
	r.bg.Resize(fyne.NewSize(plot.width, plot.height))
	r.bg.Move(fyne.NewPos(plot.x, plot.y))
	r.border.Resize(fyne.NewSize(plot.width, plot.height))
	r.border.Move(fyne.NewPos(plot.x, plot.y))
}

// layoutGrid positions horizontal gridlines + Y labels at each tick and the
// interior vertical gridlines (+ time labels when enabled).
func (r *lineChartRenderer) layoutGrid(plot chartBox) {
	// Horizontal: one per Y tick, top (hi) to bottom (lo).
	for i, line := range r.hGrid {
		frac := float32(i) / float32(chartYTickCount-1)
		y := plot.y + frac*plot.height
		line.Position1 = fyne.NewPos(plot.x, y)
		line.Position2 = fyne.NewPos(plot.x+plot.width, y)
		line.Refresh()

		lbl := r.yLabels[i]
		sz := lbl.MinSize()
		lbl.Resize(sz)
		lbl.Move(fyne.NewPos(plot.x-chartLabelGap-sz.Width, y-sz.Height/2))
	}

	// Vertical: interior columns only (the frame draws the edges).
	for i, line := range r.vGrid {
		frac := float32(i+1) / float32(chartXGridCount)
		x := plot.x + frac*plot.width
		line.Position1 = fyne.NewPos(x, plot.y)
		line.Position2 = fyne.NewPos(x, plot.y+plot.height)
		line.Refresh()
	}

	r.layoutTimeLabels(plot)
}

// layoutTimeLabels places the time-axis labels under each vertical division,
// "−span" on the left to "now" on the right.
func (r *lineChartRenderer) layoutTimeLabels(plot chartBox) {
	if r.chart.span <= 0 {
		return
	}
	ticks := chartXGridCount + 1
	for i := 0; i < ticks; i++ {
		frac := float32(i) / float32(chartXGridCount)
		age := time.Duration(float64(r.chart.span) * float64(1-frac))
		lbl := r.xLabels[i]
		lbl.Text = formatAge(age)
		lbl.Refresh()
		sz := lbl.MinSize()
		x := plot.x + frac*plot.width - sz.Width/2
		// Keep the end labels inside the widget bounds.
		x = clamp32(x, plot.x, plot.x+plot.width-sz.Width)
		lbl.Resize(sz)
		lbl.Move(fyne.NewPos(x, plot.y+plot.height+chartLabelGap))
	}
}

// buildLines resolves every visible series into a plot-local polyline. Hidden
// series are skipped but still advance the categorical color slot, so colors
// stay stable across visibility toggles.
func (r *lineChartRenderer) buildLines(plot chartBox, lo, hi float64, data [][]float64) []polyline {
	lines := make([]polyline, 0, len(r.chart.series))
	colorIdx := 0
	for i, s := range r.chart.series {
		stroke := r.resolveStroke(s, &colorIdx)
		if !s.visible || len(data[i]) < 2 {
			continue
		}
		lines = append(lines, polyline{
			pts:   r.seriesPoints(plot, lo, hi, data[i]),
			width: s.width,
			col:   stroke,
		})
	}
	return lines
}

// buildBands resolves the threshold highlight into plot-local rectangles: one
// per contiguous run where the driving series is at or above the threshold. The
// driving series is the first visible one (the emphasized headline line), so its
// x mapping matches seriesPoints. Each run is widened by half a sample step on
// both sides — so a single-sample spike still shades a visible cell — then
// clamped to the plot box.
func (r *lineChartRenderer) buildBands(plot chartBox, data [][]float64) []bandFill {
	if r.chart.band == nil {
		return nil
	}
	vals := firstVisibleSeries(data)
	if len(vals) == 0 {
		return nil
	}
	span := r.chart.window
	if span < len(vals) {
		span = len(vals)
	}
	step := plot.width / float32(max(span-1, 1))
	xAt := func(j int) float32 { return plot.width - float32(len(vals)-1-j)*step }

	var bands []bandFill
	for _, sp := range thresholdSpans(vals, r.chart.band.threshold) {
		x0 := clamp32(xAt(sp[0])-step/2, 0, plot.width)
		x1 := clamp32(xAt(sp[1])+step/2, 0, plot.width)
		bands = append(bands, bandFill{
			col: r.chart.band.fill,
			pts: []fyne.Position{
				{X: x0, Y: 0}, {X: x1, Y: 0},
				{X: x1, Y: plot.height}, {X: x0, Y: plot.height},
			},
		})
	}
	return bands
}

// firstVisibleSeries returns the first visible series' samples (arrange leaves an
// invisible series' slot nil), or nil when none has data — the series that drives
// the threshold highlight.
func firstVisibleSeries(data [][]float64) []float64 {
	for _, d := range data {
		if len(d) > 0 {
			return d
		}
	}
	return nil
}

// thresholdSpans returns the contiguous [start, end] index runs where vals is at
// or above threshold.
func thresholdSpans(vals []float64, threshold float64) [][2]int {
	var spans [][2]int
	start := -1
	for i, v := range vals {
		switch {
		case v >= threshold && start < 0:
			start = i
		case v < threshold && start >= 0:
			spans = append(spans, [2]int{start, i - 1})
			start = -1
		}
	}
	if start >= 0 {
		spans = append(spans, [2]int{start, len(vals) - 1})
	}
	return spans
}

// seriesPoints maps a series' samples to plot-local pixels: newest pinned to
// the right edge, older samples stepping left by a window-stable interval.
func (r *lineChartRenderer) seriesPoints(plot chartBox, lo, hi float64, vals []float64) []fyne.Position {
	span := r.chart.window
	if span < len(vals) {
		span = len(vals) // fill the width when no fixed window is set
	}
	step := plot.width / float32(max(span-1, 1))

	pts := make([]fyne.Position, len(vals))
	for j, v := range vals {
		x := plot.width - float32(len(vals)-1-j)*step // plot-local: right edge = width
		pts[j] = fyne.NewPos(x, valueToY(v, lo, hi, plot)-plot.y)
	}
	return pts
}

// showHover snaps the overlay to the driving series' sample nearest pos.X: a
// crosshair at that column, a marker on the line, and a tooltip of the sample's
// value and its label (the timestamp, for a recording). Only the overlay objects
// are refreshed, so a mouse move never regenerates the series raster.
func (r *lineChartRenderer) showHover(pos fyne.Position) {
	if len(r.lines) == 0 || len(r.lines[0].pts) == 0 || len(r.hoverVals) == 0 {
		return
	}
	pts := r.lines[0].pts
	idx, best := 0, float32(math.Inf(1))
	for j, p := range pts {
		if d := float32(math.Abs(float64(r.plot.x + p.X - pos.X))); d < best {
			best, idx = d, j
		}
	}
	if idx >= len(r.hoverVals) {
		return
	}
	px, py := r.plot.x+pts[idx].X, r.plot.y+pts[idx].Y

	r.crosshair.Position1 = fyne.NewPos(px, r.plot.y)
	r.crosshair.Position2 = fyne.NewPos(px, r.plot.y+r.plot.height)

	r.marker.Resize(fyne.NewSize(hoverMarkerSize, hoverMarkerSize))
	r.marker.Move(fyne.NewPos(px-hoverMarkerSize/2, py-hoverMarkerSize/2))

	r.tipVal.Text = r.chart.format(r.hoverVals[idx])
	r.tipLabel.Text = r.chart.hover(idx)
	valSize, lblSize := r.tipVal.MinSize(), r.tipLabel.MinSize()
	tw := max(valSize.Width, lblSize.Width) + 2*hoverTipPad
	th := valSize.Height + lblSize.Height + 2*hoverTipPad
	// Prefer up-and-right of the point, clamped inside the plot box.
	tx := clamp32(px+hoverTipGap, r.plot.x, r.plot.x+r.plot.width-tw)
	ty := clamp32(py-th-hoverTipGap, r.plot.y, r.plot.y+r.plot.height-th)
	r.tipBG.Resize(fyne.NewSize(tw, th))
	r.tipBG.Move(fyne.NewPos(tx, ty))
	r.tipVal.Move(fyne.NewPos(tx+hoverTipPad, ty+hoverTipPad))
	r.tipLabel.Move(fyne.NewPos(tx+hoverTipPad, ty+hoverTipPad+valSize.Height))

	for _, o := range r.hoverObjects() {
		o.Show()
		o.Refresh()
	}
}

// hideHover hides the overlay (the pointer left, or the chart isn't interactive).
func (r *lineChartRenderer) hideHover() {
	for _, o := range r.hoverObjects() {
		o.Hide()
		o.Refresh()
	}
}

// hoverObjects is the overlay set, listed once so show/hide stay in lockstep.
func (r *lineChartRenderer) hoverObjects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.crosshair, r.marker, r.tipBG, r.tipVal, r.tipLabel}
}

// renderSeries is the raster generator: it strokes each resolved polyline into
// a w×h image (device pixels). Scaling DP → pixels here keeps the lines crisp
// at any output scale, and one anti-aliased fill per series gives a smooth line
// of uniform opacity. Band fills (stacked mode) go down first so every boundary
// stroke stays crisp on top of its neighbor's translucent fill.
func (r *lineChartRenderer) renderSeries(w, h int) image.Image {
	img := r.imageFor(w, h)
	if w <= 0 || h <= 0 || r.plot.width <= 0 || r.plot.height <= 0 {
		return img
	}
	if r.ras == nil {
		r.ras = &vector.Rasterizer{}
	}
	sx := float32(w) / r.plot.width
	sy := float32(h) / r.plot.height
	// Threshold highlights go down first of all, so both the band fills (stacked
	// mode) and the line strokes sit on top of them.
	for _, b := range r.bands {
		if len(b.pts) < 3 {
			continue
		}
		r.ras.Reset(w, h)
		fillPolygon(r.ras, b.pts, sx, sy)
		r.ras.Draw(img, img.Bounds(), image.NewUniform(b.col), image.Point{})
	}
	for _, f := range r.fills {
		if len(f.pts) < 3 {
			continue
		}
		r.ras.Reset(w, h)
		fillPolygon(r.ras, f.pts, sx, sy)
		r.ras.Draw(img, img.Bounds(), image.NewUniform(f.col), image.Point{})
	}
	for _, ln := range r.lines {
		if len(ln.pts) < 2 {
			continue
		}
		r.ras.Reset(w, h)
		strokePolyline(r.ras, ln.pts, sx, sy, max(ln.width*sx, 1))
		r.ras.Draw(img, img.Bounds(), image.NewUniform(ln.col), image.Point{})
	}
	return img
}

// imageFor returns the cached plot image sized w×h, reallocating only when the
// dimensions change (resize / HiDPI scale change) and clearing the reused
// buffer for a fresh frame — a fresh image.NewRGBA is already zeroed.
func (r *lineChartRenderer) imageFor(w, h int) *image.RGBA {
	if r.img == nil || r.img.Rect.Dx() != w || r.img.Rect.Dy() != h {
		r.img = image.NewRGBA(image.Rect(0, 0, w, h))
		return r.img
	}
	clear(r.img.Pix)
	return r.img
}

// resolveStroke returns the final stroke color for a series, assigning and
// advancing the categorical palette slot for auto-colored series. c1 (the
// accent) is skipped so secondary series stay distinct from an emphasized
// headline line; auto colors cycle through c2–c8.
func (r *lineChartRenderer) resolveStroke(s *chartSeries, colorIdx *int) color.Color {
	if !s.autoColor {
		return s.stroke
	}
	secondary := palette.Series[1:] // c2–c8
	c := secondary[*colorIdx%len(secondary)]
	*colorIdx++
	c.A = chartSecondaryAlpha
	return c
}

func (r *lineChartRenderer) MinSize() fyne.Size {
	return fyne.NewSize(chartMinWidth, chartMinHeight)
}

// Objects assembles the draw order: plot fill, gridlines, the series raster,
// frame, then labels on top. The objects are reused across frames, so listing
// them here doesn't cause flicker.
func (r *lineChartRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.bg}
	for _, l := range r.vGrid {
		objs = append(objs, l)
	}
	for _, l := range r.hGrid {
		objs = append(objs, l)
	}
	objs = append(objs, r.series)
	objs = append(objs, r.border)
	for _, l := range r.yLabels {
		objs = append(objs, l)
	}
	for _, l := range r.xLabels {
		objs = append(objs, l)
	}
	// Hover overlay on top of everything (hidden until the pointer enters).
	objs = append(objs, r.crosshair, r.marker, r.tipBG, r.tipVal, r.tipLabel)
	return objs
}

func (r *lineChartRenderer) Destroy() {}

// resolveRange returns the [lo, hi] plotting bounds: the fixed range when set,
// otherwise a nice-rounded range covering the data (falling back to 0–1 when
// there is none).
func (c *lineChart) resolveRange(data [][]float64) (lo, hi float64) {
	if c.yr.fixed {
		return c.yr.min, c.yr.max
	}
	min, max := math.Inf(1), math.Inf(-1)
	for _, vals := range data {
		for _, v := range vals {
			min = math.Min(min, v)
			max = math.Max(max, v)
		}
	}
	if math.IsInf(min, 1) { // no data
		return 0, 1
	}
	return niceRange(padRange(min, max))
}

// padRange adds headroom above and below [min, max] so an auto-scaled line
// keeps clear of the frame, then hands the padded bounds to niceRange. A flat
// series (min == max) is padded relative to its magnitude. The lower bound is
// not pushed below zero for a non-negative series — rates and byte counts rest
// on a true zero baseline rather than a fabricated negative axis.
func padRange(min, max float64) (lo, hi float64, ticks int) {
	pad := (max - min) * chartAutoRangePad
	if pad == 0 {
		pad = math.Max(math.Abs(max), 1) * chartAutoRangePad
	}
	lo, hi = min-pad, max+pad
	if min >= 0 && lo < 0 {
		lo = 0
	}
	return lo, hi, chartYTickCount - 1
}
