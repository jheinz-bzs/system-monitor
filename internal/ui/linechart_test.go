package ui

import (
	"math"
	"strconv"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/josephheinz/system-monitor/internal/ringbuffer"
)

// A chart built like the CPU multi-line use case (BZS253-46) must render
// through the full widget path — CreateRenderer, Layout, Refresh, Objects —
// without panicking, including the empty-buffer and live-update cases.
func TestLineChartRendersEndToEnd(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	overall := ringbuffer.New[float64](60)
	core0 := ringbuffer.New[float64](60)
	rxBytes := ringbuffer.New[uint64](60) // a uint64 series shares the chart fine

	c := newLineChart(fixedRange(0, 100), valueFormat(func(v float64) string {
		return strconv.FormatFloat(v, 'f', 0, 64) + "%"
	}))
	c.addSeries(sourceFrom(overall), emphasized())
	core := c.addSeries(sourceFrom(core0))
	c.addSeries(sourceFrom(rxBytes))

	w := test.NewWindow(c)
	defer w.Close()
	w.Resize(fyne.NewSize(480, 240))

	// Empty buffers: must not panic and must produce a scene.
	if got := len(test.WidgetRenderer(c).Objects()); got == 0 {
		t.Fatal("renderer produced no objects for an empty chart")
	}

	// Live updates: feeding the buffers and refreshing must stay stable.
	for i := 0; i < 80; i++ { // exceed capacity to exercise eviction
		overall.Add(float64(i % 100))
		core0.Add(float64((i * 3) % 100))
		rxBytes.Add(uint64(i) * 1024)
		c.Refresh()
	}

	// Toggling a series off must not panic either.
	core.setVisible(false)
	c.Refresh()

	if got := len(test.WidgetRenderer(c).Objects()); got == 0 {
		t.Fatal("renderer produced no objects after updates")
	}
}

// sourceFrom must adapt a ring buffer regardless of its element kind, since the
// metric history is float64 (CPU percent) for some tabs and uint64 (memory /
// network bytes) for others. Type inference should pick the element type up
// from the argument with no explicit type argument.
func TestSourceFromIsTypeAgnostic(t *testing.T) {
	t.Run("float64", func(t *testing.T) {
		rb := ringbuffer.New[float64](4)
		rb.Add(12.5)
		rb.Add(40)
		rb.Add(99.9)

		got := sourceFrom(rb).Values()
		want := []float64{12.5, 40, 99.9}
		assertFloats(t, got, want)
	})

	t.Run("uint64", func(t *testing.T) {
		rb := ringbuffer.New[uint64](4)
		rb.Add(1024)
		rb.Add(2048)
		rb.Add(1 << 40) // 1 TiB — well within float64's exact-integer range

		got := sourceFrom(rb).Values()
		want := []float64{1024, 2048, 1 << 40}
		assertFloats(t, got, want)
	})
}

// Values must re-read the buffer each call, so the chart always reflects the
// latest window rather than a snapshot taken when the series was added.
func TestSourceFromReflectsLatestWindow(t *testing.T) {
	rb := ringbuffer.New[float64](2)
	src := sourceFrom(rb)

	if got := src.Values(); len(got) != 0 {
		t.Fatalf("empty buffer: got %v, want no values", got)
	}
	rb.Add(1)
	rb.Add(2)
	rb.Add(3) // evicts the oldest; buffer now holds {2, 3}

	assertFloats(t, src.Values(), []float64{2, 3})
}

func TestResolveRangeFixed(t *testing.T) {
	c := newLineChart(fixedRange(0, 100))
	lo, hi := c.resolveRange([][]float64{{42, 73}})
	if lo != 0 || hi != 100 {
		t.Fatalf("fixed range: got [%v, %v], want [0, 100]", lo, hi)
	}
}

func TestResolveRangeAuto(t *testing.T) {
	c := newLineChart() // auto by default
	lo, hi := c.resolveRange([][]float64{{3, 88}, {17}})
	// Bounds must enclose the data and round to nice numbers.
	if lo > 3 || hi < 88 {
		t.Fatalf("auto range [%v, %v] does not enclose data 3..88", lo, hi)
	}
	if lo != 0 || hi != 100 {
		t.Fatalf("auto range: got [%v, %v], want nice [0, 100]", lo, hi)
	}
}

func TestResolveRangeNoData(t *testing.T) {
	c := newLineChart()
	lo, hi := c.resolveRange(nil)
	if lo != 0 || hi != 1 {
		t.Fatalf("no data: got [%v, %v], want fallback [0, 1]", lo, hi)
	}
}

func TestValueToY(t *testing.T) {
	plot := chartBox{x: 10, y: 0, width: 100, height: 200}
	cases := []struct {
		name  string
		v     float64
		wantY float32
	}{
		{"top maps to plot top", 100, 0},
		{"bottom maps to plot bottom", 0, 200},
		{"midpoint maps to mid-plot", 50, 100},
		{"above range clamps to top", 150, 0},
		{"below range clamps to bottom", -20, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := valueToY(tc.v, 0, 100, plot); got != tc.wantY {
				t.Fatalf("valueToY(%v) = %v, want %v", tc.v, got, tc.wantY)
			}
		})
	}
}

func TestTickValuesTopToBottom(t *testing.T) {
	got := tickValues(0, 100, 5)
	want := []float64{100, 75, 50, 25, 0} // top (hi) → bottom (lo)
	assertFloats(t, got, want)
}

func TestNiceRangeEnclosesAndRounds(t *testing.T) {
	lo, hi := niceRange(2, 97, 4)
	if lo > 2 || hi < 97 {
		t.Fatalf("niceRange [%v, %v] must enclose 2..97", lo, hi)
	}
	if lo != 0 || hi != 100 {
		t.Fatalf("niceRange = [%v, %v], want [0, 100]", lo, hi)
	}
}

func TestNiceRangeFlatSeries(t *testing.T) {
	lo, hi := niceRange(50, 50, 4)
	if !(lo < 50 && hi > 50) {
		t.Fatalf("flat series should pad around the value: got [%v, %v]", lo, hi)
	}
}

func TestFormatAge(t *testing.T) {
	cases := map[string]string{
		"now":     formatAge(0),
		"seconds": formatAge(45e9),  // 45s
		"minutes": formatAge(180e9), // 3m
	}
	want := map[string]string{"now": "now", "seconds": "-45s", "minutes": "-3m"}
	for k, got := range cases {
		if got != want[k] {
			t.Errorf("%s: got %q, want %q", k, got, want[k])
		}
	}
}

func assertFloats(t *testing.T, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("index %d: got %v, want %v", i, got[i], want[i])
		}
	}
}
