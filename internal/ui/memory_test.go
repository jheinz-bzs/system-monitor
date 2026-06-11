package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/ringbuffer"
	"github.com/josephheinz/system-monitor/internal/series"
)

const testTotalMem = uint64(32) << 30 // 32 GiB

func testMemSources(used, cached, free *ringbuffer.RingBuffer[uint64]) memSources {
	return memSources{
		used:   series.SourceFrom(used),
		cached: series.SourceFrom(cached),
		free:   series.SourceFrom(free),
		total:  testTotalMem,
	}
}

// The Memory view must render through the full widget path and stay stable as
// its band buffers fill, including the empty-buffer case. This mirrors how the
// poller feeds it live: append a sample to each buffer, then refresh.
func TestMemoryViewRendersAndRefreshes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	used := ringbuffer.New[uint64](metrics.HistoryCapacity)
	cached := ringbuffer.New[uint64](metrics.HistoryCapacity)
	free := ringbuffer.New[uint64](metrics.HistoryCapacity)
	v := newMemoryView(testMemSources(used, cached, free))

	w := test.NewWindow(v.object())
	defer w.Close()
	w.Resize(fyne.NewSize(1100, 760))

	// Empty buffers: nothing panics.
	v.refresh()

	// Feeding samples and refreshing must stay stable.
	for i := 0; i <= 100; i++ {
		u := uint64(i+1) << 27
		c := uint64(4) << 30
		used.Add(u)
		cached.Add(c)
		free.Add(testTotalMem - u - c)
		v.refresh()
	}
}

// wired must require every band source plus a non-zero total — a partially
// adapted collector falls back to the tab placeholder instead of a broken chart.
func TestMemSourcesWired(t *testing.T) {
	buf := ringbuffer.New[uint64](metrics.HistoryCapacity)
	full := testMemSources(buf, buf, buf)

	if !full.wired() {
		t.Fatal("fully populated sources must report wired")
	}
	cases := map[string]memSources{
		"zero value":   {},
		"missing free": {used: full.used, cached: full.cached, total: full.total},
		"zero total":   {used: full.used, cached: full.cached, free: full.free},
	}
	for name, src := range cases {
		if src.wired() {
			t.Errorf("%s: must not report wired", name)
		}
	}
}

func TestFormatBytesAxis(t *testing.T) {
	cases := map[float64]string{
		0:          "0B",
		8 << 30:    "8.0G",
		32 << 30:   "32G",
		1.5 * 1024: "1.5K",
		-5:         "0B", // defensive clamp; can't occur on a 0-based axis
	}
	for in, want := range cases {
		if got := formatBytesAxis(in); got != want {
			t.Errorf("formatBytesAxis(%v) = %q, want %q", in, got, want)
		}
	}
}
