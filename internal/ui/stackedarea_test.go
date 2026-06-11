package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/ringbuffer"
	"github.com/josephheinz/system-monitor/internal/series"
)

// Each boundary must be the running sum of the series below it, so bands
// stack rather than overlap.
func TestStackSeriesCumulative(t *testing.T) {
	got := stackSeries([][]float64{{1, 2}, {3, 4}, {5, 6}})
	want := [][]float64{{1, 2}, {4, 6}, {9, 12}}
	for i := range want {
		assertFloats(t, got[i], want[i])
	}
}

// A hidden (empty) series must stay empty and must not advance the running
// boundary — the band above it stacks directly on the band below.
func TestStackSeriesSkipsEmpty(t *testing.T) {
	got := stackSeries([][]float64{{1, 2}, nil, {5, 6}})
	assertFloats(t, got[0], []float64{1, 2})
	if got[1] != nil {
		t.Fatalf("hidden series: got %v, want nil", got[1])
	}
	assertFloats(t, got[2], []float64{6, 8})
}

// When buffer lengths differ (a read can land between a collector's appends),
// series must align on their newest samples so every column sums same-tick
// values.
func TestStackSeriesAlignsNewest(t *testing.T) {
	got := stackSeries([][]float64{{1, 2, 3}, {10, 20}})
	assertFloats(t, got[0], []float64{2, 3})
	assertFloats(t, got[1], []float64{12, 23})
}

func TestStackSeriesAllEmpty(t *testing.T) {
	got := stackSeries([][]float64{nil, {}})
	for i, vals := range got {
		if len(vals) != 0 {
			t.Fatalf("series %d: got %v, want empty", i, vals)
		}
	}
}

// The bottom band must close down to the plot's bottom edge.
func TestBandPolygonBottomBand(t *testing.T) {
	upper := []fyne.Position{{X: 0, Y: 10}, {X: 50, Y: 20}}
	const bottom = float32(100)

	got := bandPolygon(upper, nil, bottom)

	want := []fyne.Position{{X: 0, Y: 10}, {X: 50, Y: 20}, {X: 50, Y: 100}, {X: 0, Y: 100}}
	assertPositions(t, got, want)
}

// A stacked band must close against the boundary below it, walked in reverse
// so the polygon ring stays continuous.
func TestBandPolygonStackedBand(t *testing.T) {
	upper := []fyne.Position{{X: 0, Y: 5}, {X: 50, Y: 8}}
	lower := []fyne.Position{{X: 0, Y: 10}, {X: 50, Y: 20}}

	got := bandPolygon(upper, lower, 100)

	want := []fyne.Position{{X: 0, Y: 5}, {X: 50, Y: 8}, {X: 50, Y: 20}, {X: 0, Y: 10}}
	assertPositions(t, got, want)
}

// A stacked chart built like the Memory tab's composition chart must render
// through the full widget path — CreateRenderer, Layout, Refresh, Objects —
// without panicking, including the empty-buffer and live-update cases.
func TestStackedChartRendersEndToEnd(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	used := ringbuffer.New[uint64](metrics.HistoryCapacity)
	cached := ringbuffer.New[uint64](metrics.HistoryCapacity)
	free := ringbuffer.New[uint64](metrics.HistoryCapacity)

	c := newLineChart(
		fixedRange(0, float64(testTotalMem)),
		valueFormat(formatBytesAxis),
		window(metrics.HistoryCapacity),
		stackedArea(),
	)
	c.addSeries(series.SourceFrom(used), seriesColor(palette.Accent))
	cachedBand := c.addSeries(series.SourceFrom(cached), seriesColor(palette.Series[memCachedSeriesIndex]))
	c.addSeries(series.SourceFrom(free), seriesColor(palette.SeriesMuted))

	w := test.NewWindow(c)
	defer w.Close()
	w.Resize(fyne.NewSize(480, 240))

	// Empty buffers: must not panic and must produce a scene.
	if got := len(test.WidgetRenderer(c).Objects()); got == 0 {
		t.Fatal("renderer produced no objects for an empty stacked chart")
	}

	// Live updates: feeding the buffers and refreshing must stay stable.
	for i := 0; i < 80; i++ { // exceed capacity to exercise eviction
		u := uint64(i+1) << 28
		cd := uint64(2) << 30
		used.Add(u)
		cached.Add(cd)
		free.Add(testTotalMem - u - cd)
		c.Refresh()
	}

	// Toggling a middle band off must not panic; the band above restacks.
	cachedBand.setVisible(false)
	c.Refresh()

	if got := len(test.WidgetRenderer(c).Objects()); got == 0 {
		t.Fatal("renderer produced no objects after updates")
	}
}

func assertPositions(t *testing.T, got, want []fyne.Position) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %v, want %v", i, got[i], want[i])
		}
	}
}
