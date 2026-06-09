package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/ringbuffer"
	"github.com/josephheinz/system-monitor/internal/series"
)

// The CPU view must render through the full widget path and stay stable as its
// overall-utilization buffer fills, including the empty-buffer case. This mirrors
// how the poller feeds it live (BZS253-46): append a sample, then refresh.
func TestCPUViewRendersAndRefreshes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	overall := ringbuffer.New[float64](metrics.HistoryCapacity)
	v := newCPUView(series.SourceFrom(overall), nil)

	w := test.NewWindow(v.object())
	defer w.Close()
	w.Resize(fyne.NewSize(640, 320))

	// Empty buffer: readout shows the placeholder and nothing panics.
	v.refresh()
	if got := v.readout.Text; got != "--" {
		t.Fatalf("empty buffer readout: got %q, want %q", got, "--")
	}

	// Feeding samples and refreshing must stay stable and track the newest value.
	for i := 0; i <= 100; i++ {
		overall.Add(float64(i))
		v.refresh()
	}
	if got, want := v.readout.Text, "100%"; got != want {
		t.Fatalf("readout after updates: got %q, want %q", got, want)
	}
}

func TestFormatPercent(t *testing.T) {
	cases := map[float64]string{0: "0%", 42.4: "42%", 42.6: "43%", 100: "100%"}
	for in, want := range cases {
		if got := formatPercent(in); got != want {
			t.Errorf("formatPercent(%v) = %q, want %q", in, got, want)
		}
	}
}
