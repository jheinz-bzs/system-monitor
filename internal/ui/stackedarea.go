package ui

// Stacked-area rendering for lineChart (design-system-06 "Stacked area"): the
// chart's series become bands stacked bottom → top in declaration order. Each
// band is filled at its series hue at 50% opacity with the series' full-hue
// polyline stroked along its top boundary — the fill/stroke treatment taken
// from the design system's stacked-area example. The Memory tab's
// used/cached/free composition chart is the first consumer.
//
// The stacking math (stackSeries, bandPolygon) is pure — no Fyne widget state —
// so it is independently testable, matching the raster.go / format.go split.

import (
	"image/color"

	"fyne.io/fyne/v2"
)

// chartStackFillAlpha is the band fill opacity (50%, from the design-system
// stacked-area example: fill-opacity 0.5 at the series hue).
const chartStackFillAlpha = 0x80

// stackedArea renders the chart's series as a stacked area chart: each series'
// polyline becomes the top boundary of a band stacked on the series added
// before it (declaration order = bottom → top), filled in the series hue.
func stackedArea() lineChartOption {
	return func(c *lineChart) { c.stacked = true }
}

// bandFill is one resolved stacked band ready to fill: a closed plot-local
// polygon and its translucent fill color.
type bandFill struct {
	pts []fyne.Position
	col color.Color
}

// buildStack resolves every visible series into its boundary polyline and band
// polygon. bounds holds the cumulative stack boundaries (see stackSeries), so
// each band fills between its own boundary and the visible series below it (or
// the plot's bottom edge for the lowest band). Like buildLines, skipped series
// still advance the categorical color slot so colors stay stable across
// visibility toggles.
func (r *lineChartRenderer) buildStack(plot chartBox, lo, hi float64, bounds [][]float64) ([]polyline, []bandFill) {
	lines := make([]polyline, 0, len(r.chart.series))
	fills := make([]bandFill, 0, len(r.chart.series))
	colorIdx := 0
	var lower []fyne.Position // nil = the plot's bottom edge
	for i, s := range r.chart.series {
		stroke := r.resolveStroke(s, &colorIdx)
		if !s.visible || len(bounds[i]) < 2 {
			continue
		}
		upper := r.seriesPoints(plot, lo, hi, bounds[i])
		lines = append(lines, polyline{pts: upper, width: s.width, col: stroke})
		fills = append(fills, bandFill{
			pts: bandPolygon(upper, lower, plot.height),
			col: withAlpha(stroke, chartStackFillAlpha),
		})
		lower = upper
	}
	return lines, fills
}

// stackSeries converts per-series samples into cumulative stack boundaries:
// out[i][j] = data[0][j] + … + data[i][j], so each series' polyline is the top
// edge of its band. Empty (hidden) series stay empty and don't advance the
// running boundary.
//
// Series are aligned on their newest samples: the collector appends to its
// buffers one after another, so a read between appends can see lengths differ
// by one. Trimming each series to the shortest length from the oldest end
// keeps every column a same-tick sum, with the newest sample pinned right.
func stackSeries(data [][]float64) [][]float64 {
	width := stackWidth(data)
	out := make([][]float64, len(data))
	var running []float64
	for i, vals := range data {
		if len(vals) == 0 {
			continue
		}
		tail := vals[len(vals)-width:]
		boundary := make([]float64, width)
		for j, v := range tail {
			boundary[j] = v
			if running != nil {
				boundary[j] += running[j]
			}
		}
		out[i] = boundary
		running = boundary
	}
	return out
}

// stackWidth returns the number of columns the stack can fill: the shortest
// non-empty series length (0 when every series is empty).
func stackWidth(data [][]float64) int {
	width := 0
	for _, vals := range data {
		if len(vals) == 0 {
			continue
		}
		if width == 0 || len(vals) < width {
			width = len(vals)
		}
	}
	return width
}

// bandPolygon closes the region between a band's upper boundary and the band
// below it into one plot-local polygon: the upper polyline left → right, then
// the lower boundary walked back right → left. A nil lower means the band sits
// on the plot's bottom edge (bottom, in plot-local pixels). upper must be
// non-empty — buildStack guarantees ≥ 2 points.
func bandPolygon(upper, lower []fyne.Position, bottom float32) []fyne.Position {
	pts := make([]fyne.Position, 0, len(upper)+max(len(lower), 2))
	pts = append(pts, upper...)
	if len(lower) == 0 {
		last, first := upper[len(upper)-1], upper[0]
		return append(pts, fyne.NewPos(last.X, bottom), fyne.NewPos(first.X, bottom))
	}
	for i := len(lower) - 1; i >= 0; i-- {
		pts = append(pts, lower[i])
	}
	return pts
}
