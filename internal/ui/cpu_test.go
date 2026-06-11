package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/ringbuffer"
	"github.com/josephheinz/system-monitor/internal/series"
)

// testCoreCount sizes the fake per-core fixtures: enough cores to span
// multiple grid rows and wrap the categorical palette is what the live
// collector typically provides.
const testCoreCount = 12

// The CPU view must render through the full widget path and stay stable as its
// overall and per-core buffers fill, including the empty-buffer case. This
// mirrors how the poller feeds it live: append a sample, then refresh.
func TestCPUViewRendersAndRefreshes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	overall := ringbuffer.New[float64](metrics.HistoryCapacity)
	coreBufs := make([]*ringbuffer.RingBuffer[float64], testCoreCount)
	cores := make([]series.Source, testCoreCount)
	for i := range coreBufs {
		coreBufs[i] = ringbuffer.New[float64](metrics.HistoryCapacity)
		cores[i] = series.SourceFrom(coreBufs[i])
	}
	procs := processSourceFunc(func(n int) []processRow {
		return []processRow{
			{pid: 3412, name: "chrome", user: "you", cpu: 42, mem: 1 << 31},
		}
	})
	v := newCPUView(series.SourceFrom(overall), cores, procs,
		cpuMeta{cores: testCoreCount, model: "Test CPU"})

	w := test.NewWindow(v.object())
	defer w.Close()
	w.Resize(fyne.NewSize(1100, 760))

	// Empty buffers: nothing panics.
	v.refresh()

	// Feeding samples and refreshing must stay stable.
	for i := 0; i <= 100; i++ {
		overall.Add(float64(i))
		for c, buf := range coreBufs {
			buf.Add(float64((i + c) % (percentMax + 1)))
		}
		v.refresh()
	}

	// Hiding and re-showing the per-core lines must stay stable across
	// refreshes (the header toggle drives this path).
	v.setPerCoreVisible(false)
	v.refresh()
	v.setPerCoreVisible(true)
	v.refresh()
}

// A nil process source must degrade to the per-core panel taking the full
// bottom row, and empty per-core sources to a blank panel body — never a
// nil panic.
func TestCPUViewWithoutProcessSource(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	overall := ringbuffer.New[float64](metrics.HistoryCapacity)
	v := newCPUView(series.SourceFrom(overall), nil, nil, cpuMeta{})

	w := test.NewWindow(v.object())
	defer w.Close()
	w.Resize(fyne.NewSize(640, 320))
	v.refresh()
}

func TestFormatPercent(t *testing.T) {
	cases := map[float64]string{0: "0%", 42.4: "42%", 42.6: "43%", 100: "100%"}
	for in, want := range cases {
		if got := formatPercent(in); got != want {
			t.Errorf("formatPercent(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatPercent1(t *testing.T) {
	cases := map[float64]string{0: "0.0", 42: "42.0", 7.25: "7.2", 100: "100.0"}
	for in, want := range cases {
		if got := formatPercent1(in); got != want {
			t.Errorf("formatPercent1(%v) = %q, want %q", in, got, want)
		}
	}
}
