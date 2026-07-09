package ui

import (
	"strings"
	"testing"
)

// TestThresholdWatcherEdgeTrigger feeds a rising-then-falling series through the
// watcher and asserts exactly one notification per up-crossing, with a re-arm on
// the drop below — the whole point of edge- over level-triggering (a busy
// machine must not toast every tick).
func TestThresholdWatcherEdgeTrigger(t *testing.T) {
	// Scripted CPU usage: below, cross up (fire), stay high (silent), stay high
	// (silent), drop below (re-arm), cross up again (fire). Memory/disk read
	// below their thresholds throughout, so only CPU fires.
	series := []float64{50, 95, 96, 91, 40, 92}
	i := 0
	prefs := newSettings(newFakePrefs())
	prefs.setAlertEnabled(thresholdCPU, true)
	prefs.setAlertThreshold(thresholdCPU, 90)

	var fires int
	w := newThresholdWatcher(prefs, thresholdReaders{
		cpu:    func() (float64, bool) { v := series[i]; return v, true },
		memory: func() (float64, bool) { return 0, true },
		disk:   func() (float64, bool) { return 0, true },
	}, func(_, _ string) { fires++ })

	for ; i < len(series); i++ {
		w.tick()
	}

	if fires != 2 {
		t.Fatalf("got %d notifications, want 2 (one per up-crossing)", fires)
	}
}

// TestThresholdWatcherDisabledAndUnavailable: a disabled metric never fires, and
// an unreadable reader (ok=false) is skipped rather than firing on a zero.
func TestThresholdWatcherDisabledAndUnavailable(t *testing.T) {
	prefs := newSettings(newFakePrefs())
	prefs.setAlertEnabled(thresholdMemory, true) // enabled, but reader unavailable
	prefs.setAlertThreshold(thresholdMemory, 80)
	// CPU left disabled (default) even though it reads sky-high.

	var fires int
	w := newThresholdWatcher(prefs, thresholdReaders{
		cpu:    func() (float64, bool) { return 100, true },
		memory: func() (float64, bool) { return 99, false },
		disk:   func() (float64, bool) { return 0, false },
	}, func(_, _ string) { fires++ })

	w.tick()
	w.tick()

	if fires != 0 {
		t.Fatalf("got %d notifications, want 0 (disabled + unavailable)", fires)
	}
}

// TestThresholdWatcherLiveConfig: a threshold change made after construction
// applies on the next tick, and the notification body names the threshold that
// actually fired — config is read live from prefs, never cached at startup.
func TestThresholdWatcherLiveConfig(t *testing.T) {
	prefs := newSettings(newFakePrefs())
	prefs.setAlertEnabled(thresholdCPU, true)
	prefs.setAlertThreshold(thresholdCPU, 90)

	var bodies []string
	w := newThresholdWatcher(prefs, thresholdReaders{
		cpu:    func() (float64, bool) { return 85, true },
		memory: func() (float64, bool) { return 0, true },
		disk:   func() (float64, bool) { return 0, true },
	}, func(_, body string) { bodies = append(bodies, body) })

	w.tick() // 85 < 90: silent
	if len(bodies) != 0 {
		t.Fatalf("fired below threshold: %q", bodies)
	}

	prefs.setAlertThreshold(thresholdCPU, 80) // user lowers it mid-run
	w.tick()                                  // 85 >= 80: fires with the new threshold
	if len(bodies) != 1 {
		t.Fatalf("got %d notifications after lowering threshold, want 1", len(bodies))
	}
	if want := "(threshold 80%)"; !strings.Contains(bodies[0], want) {
		t.Errorf("body %q does not name the live threshold %q", bodies[0], want)
	}
}

// TestAlertPrefsRoundTrip: the alert enable/threshold keys default off/90 and
// read back what was set, per metric.
func TestAlertPrefsRoundTrip(t *testing.T) {
	s := newSettings(newFakePrefs())

	for _, m := range []thresholdMetric{thresholdCPU, thresholdMemory, thresholdDisk} {
		if s.alertEnabled(m) {
			t.Errorf("metric %d: alertEnabled default = true, want off", m)
		}
		if got := s.alertThreshold(m); got != defaultAlertThreshold {
			t.Errorf("metric %d: alertThreshold default = %d, want %d", m, got, defaultAlertThreshold)
		}
	}

	s.setAlertEnabled(thresholdDisk, true)
	s.setAlertThreshold(thresholdDisk, 70)
	if !s.alertEnabled(thresholdDisk) {
		t.Error("disk alertEnabled = false after setting true")
	}
	if got := s.alertThreshold(thresholdDisk); got != 70 {
		t.Errorf("disk alertThreshold = %d, want 70", got)
	}
	// Setting one metric must not bleed into another (independent keys).
	if s.alertEnabled(thresholdCPU) {
		t.Error("cpu alertEnabled leaked true from disk set")
	}
}
