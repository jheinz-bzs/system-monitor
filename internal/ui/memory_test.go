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
	procs := memProcessSourceFunc(func(n int) []processRow {
		return []processRow{
			{pid: 3412, name: "chrome", user: "you", mem: 1 << 31},
			{pid: 540, name: "postgres", user: "pg", mem: 1 << 29},
		}
	})
	v := newMemoryView(testMemSources(used, cached, free), procs)

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

// A nil process source must degrade to the placeholder bottom pane — never a
// nil panic (the CPU tab's nil-collector behavior).
func TestMemoryViewWithoutProcessSource(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	used := ringbuffer.New[uint64](metrics.HistoryCapacity)
	cached := ringbuffer.New[uint64](metrics.HistoryCapacity)
	free := ringbuffer.New[uint64](metrics.HistoryCapacity)
	v := newMemoryView(testMemSources(used, cached, free), nil)

	w := test.NewWindow(v.object())
	defer w.Close()
	w.Resize(fyne.NewSize(640, 320))
	v.refresh()
}

// The memory table source must format the wireframe's columns: PID, name,
// short user, compact RSS, a bar fraction linear in the row's share of total
// physical memory (full track at memBarFullScalePct, clamped), and Mem%
// carrying that share as percentage points.
func TestMemProcessTableSourceSnapshot(t *testing.T) {
	src := memProcessSourceFunc(func(n int) []processRow {
		return []processRow{
			{pid: 3412, name: "chrome", user: `DOMAIN\you`, mem: 5 << 30}, // 15.625% of total
			{pid: 540, name: "postgres", user: "pg", mem: 1 << 28},        // 0.78125% of total
		}
	})
	cells := (&memProcessTableSource{src: src, total: testTotalMem}).Snapshot()

	if len(cells) != 2 {
		t.Fatalf("got %d rows, want 2", len(cells))
	}
	want := [][]string{
		{"3412", "chrome", "you", "5.0G", "", "15.6"},
		{"540", "postgres", "pg", "256M", "", "0.8"},
	}
	for i, row := range want {
		for j, text := range row {
			if got := cells[i][j].text; got != text {
				t.Errorf("cell[%d][%d].text = %q, want %q", i, j, got, text)
			}
		}
	}
	// Row 0 exceeds the bar's full scale (15.625% > 10%) and clamps to a full
	// track; row 1 fills at its percentage over that scale (0.78125 / 10).
	if got := cells[0][4].frac; got != 1 {
		t.Errorf("row 0 bar frac = %v, want 1", got)
	}
	if got := cells[1][4].frac; got != 0.078125 {
		t.Errorf("row 1 bar frac = %v, want 0.078125", got)
	}
}

// A zero total (unknown physical memory) must not divide by zero; %Mem and the
// bars degrade to zero. An empty snapshot must stay empty without panicking on
// the max-RSS lookup.
func TestMemProcessTableSourceDegenerateInputs(t *testing.T) {
	src := memProcessSourceFunc(func(n int) []processRow {
		return []processRow{{pid: 1, name: "init", mem: 0}}
	})
	cells := (&memProcessTableSource{src: src, total: 0}).Snapshot()
	if got := cells[0][5].text; got != "0.0" {
		t.Errorf("zero-total %%Mem = %q, want \"0.0\"", got)
	}
	if got := cells[0][4].frac; got != 0 {
		t.Errorf("zero-RSS bar frac = %v, want 0", got)
	}

	empty := memProcessSourceFunc(func(n int) []processRow { return nil })
	if cells := (&memProcessTableSource{src: empty, total: testTotalMem}).Snapshot(); len(cells) != 0 {
		t.Errorf("empty source: got %d rows, want 0", len(cells))
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
