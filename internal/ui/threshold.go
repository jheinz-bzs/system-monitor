package ui

// Threshold notifications (BZS253-75). A pure, Fyne-free watcher: on every poll
// tick it reads the current CPU / memory / disk usage the charts already read
// and sends a native OS notification the moment usage crosses up through a
// user-set threshold. It is edge-triggered — one notification per up-crossing,
// re-armed only after usage drops back below — so a machine pinned above the
// line doesn't emit a toast every tick. Keeping the crossing logic free of Fyne
// makes it unit-testable without a running app (see threshold_test.go); the
// composition root (app.go) supplies the readers and the notify sink.

import (
	"fmt"
	"time"
)

// notifyCooldown is the minimum time between notifications for one metric.
// Edge-triggering alone stops a *pinned* metric from re-toasting, but usage
// oscillating across the threshold re-crosses every few ticks — and the OS
// queues each toast and plays them back seconds apart, so a burst arrives as
// a stack of already-stale alerts. At most one notification per cooldown per
// metric keeps the queue empty; crossings inside the window still latch and
// re-arm, they just don't toast.
const notifyCooldown = 15 * time.Second

// thresholdMetric identifies which resource a rule watches. It is the typed key
// for the persisted enable/threshold preference pair (prefs.go) and selects the
// rule's display label.
type thresholdMetric int

const (
	thresholdCPU thresholdMetric = iota
	thresholdMemory
	thresholdDisk
)

// metricLabel is the human name for each metric, used in the notification body.
// A namespaced dictionary rather than loose literals (no-string-literals).
var metricLabel = map[thresholdMetric]string{
	thresholdCPU:    "CPU",
	thresholdMemory: "Memory",
	thresholdDisk:   "Disk",
}

// Notification format strings, composed at the single call site in tick
// (allowed under no-string-literals). Body example: "CPU 94% (threshold 90%)".
const (
	notifyTitleFmt = "%s usage high"
	notifyBodyFmt  = "%s %.0f%% (threshold %.0f%%)"
)

// usageReader returns the metric's current usage percentage, or ok=false when
// it can't be read this tick (collector absent or no sample yet) — the watcher
// then skips the rule rather than firing on a zero.
type usageReader func() (value float64, ok bool)

// thresholdReaders bundles one usage reader per metric, supplied by the
// composition root (which alone knows the collector concretes).
type thresholdReaders struct {
	cpu    usageReader
	memory usageReader
	disk   usageReader
}

// notifyFunc delivers a fired alert. Production wraps fyne.App.SendNotification;
// the test supplies a recorder, so the watcher needs no running app.
type notifyFunc func(title, body string)

// thresholdRule is one metric's watch state: its reader, the prefs it reads
// live, and the edge-trigger latch. crossedUp holds the entire notify-once
// policy. Config is read from prefs every tick — not cached at construction —
// so a Settings change takes effect immediately and the notification body
// always names the threshold that actually fired (Fyne preferences reads are
// in-memory, so per-tick reads cost nothing).
type thresholdRule struct {
	metric    thresholdMetric
	label     string
	read      usageReader
	prefs     settings
	firing    bool      // true once latched, until usage drops back below (re-arm)
	lastFired time.Time // when this rule last notified, for the cooldown
	value     float64   // last usage read, for the notification body
	threshold float64   // threshold at the last evaluation, for the notification body
}

// crossedUp reports whether this tick is an up-crossing that should notify:
// usage at or above the threshold when the rule was previously armed, and no
// notification sent within the cooldown. It stays silent while usage remains
// high (already latched) and re-arms the moment usage falls back below, so a
// sustained spike notifies exactly once and an oscillating one at most once
// per notifyCooldown.
func (r *thresholdRule) crossedUp(now time.Time) bool {
	if !r.prefs.alertEnabled(r.metric) {
		r.firing = false
		return false
	}
	v, ok := r.read()
	if !ok {
		return false
	}
	r.value = v
	r.threshold = float64(r.prefs.alertThreshold(r.metric))
	if v < r.threshold {
		r.firing = false
		return false
	}
	if r.firing {
		return false
	}
	r.firing = true
	if now.Sub(r.lastFired) < notifyCooldown {
		return false // latched, but too soon after the last toast to send another
	}
	r.lastFired = now
	return true
}

// thresholdWatcher fires notifications for the metrics whose alert is enabled.
// tick is registered as a poller OnTick observer (app.go). now is the cooldown
// clock — time.Now in production, a fake in tests.
type thresholdWatcher struct {
	rules  []*thresholdRule
	notify notifyFunc
	now    func() time.Time
}

// newThresholdWatcher builds the watcher over the persisted settings and the
// live usage readers. Every rule is created; a disabled one simply never fires.
func newThresholdWatcher(prefs settings, r thresholdReaders, notify notifyFunc) *thresholdWatcher {
	rule := func(m thresholdMetric, read usageReader) *thresholdRule {
		return &thresholdRule{
			metric: m,
			label:  metricLabel[m],
			read:   read,
			prefs:  prefs,
		}
	}
	return &thresholdWatcher{
		rules: []*thresholdRule{
			rule(thresholdCPU, r.cpu),
			rule(thresholdMemory, r.memory),
			rule(thresholdDisk, r.disk),
		},
		notify: notify,
		now:    time.Now,
	}
}

// tick evaluates every rule once and notifies on each up-crossing. Runs on the
// poller goroutine (Fyne-free); the body is composed here at the single site.
func (w *thresholdWatcher) tick() {
	now := w.now()
	for _, r := range w.rules {
		if r.crossedUp(now) {
			w.notify(
				fmt.Sprintf(notifyTitleFmt, r.label),
				fmt.Sprintf(notifyBodyFmt, r.label, r.value, r.threshold),
			)
		}
	}
}
