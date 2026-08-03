package update

import (
	"context"
	"time"
)

// DefaultCheckInterval is the periodic update-check cadence (issue #84).
// latestReleaseURL is an unauthenticated GitHub API call capped at ~60
// requests/hr per IP, and a rate-limited Check degrades to a silent logged
// no-op — so the cadence has to fit inside the budget, not sit at it. At 5
// minutes a session burns at most 12/hr even while the app stays up to date,
// leaving headroom for NAT/shared-IP users. A future GitHub-token preference
// could tighten this without changing the loop.
const DefaultCheckInterval = 5 * time.Minute

// shouldCheck reports whether a periodic check should run now, given the
// controller's current status. Only StatusIdle is worth re-querying: an already
// known release (StatusAvailable) has its banner up, so re-checking only burns
// rate-limit budget; a check or install in flight (StatusChecking,
// StatusDownloading, StatusInstalling) must not race the state machine; and a
// failed install (StatusFailed) must not auto-retry into a nag loop.
func shouldCheck(s Status) bool { return s == StatusIdle }

// RunPeriodicChecks checks for updates immediately on launch, then once per
// interval while the app runs — a release that ships mid-session shows up in
// the status bar/banner without a relaunch (issue #84). It runs on its own
// goroutine and stops as soon as ctx is cancelled (window close, or the restart
// that follows a self-update), so no ticker leaks past shutdown. An interval of
// 0 or less falls back to DefaultCheckInterval.
//
// Each check is gated by shouldCheck, so once a newer release is known the loop
// stops hitting the API. onAvailable, when non-nil, runs after a check that
// lands on StatusAvailable; the composition root uses it to honor the
// auto-install preference, keeping this package policy-agnostic (ADR-010).
func RunPeriodicChecks(ctx context.Context, c *Controller, interval time.Duration, onAvailable func()) {
	if interval <= 0 {
		interval = DefaultCheckInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			if shouldCheck(c.Snapshot().State) {
				c.Check(ctx)
				if onAvailable != nil && c.Snapshot().State == StatusAvailable {
					onAvailable()
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
