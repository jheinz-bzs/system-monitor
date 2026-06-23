package ui

// Health model: per-metric usage classification plus the aggregate-badge rule,
// shared by the Overview header badge (overview.go) and the status-bar footer
// indicator (statusbar.go) so the two always report the same overall state.

// healthThresholds classifies a usage percentage: at or above crit it is
// critical, at or above warn it is warning, otherwise healthy.
type healthThresholds struct{ warn, crit float64 }

func (t healthThresholds) classify(pct float64) statusKind {
	switch {
	case pct >= t.crit:
		return status.Critical
	case pct >= t.warn:
		return status.Warning
	default:
		return status.Healthy
	}
}

// Per-metric thresholds (percent of capacity). CPU spikes routinely under normal
// work, so it warns early; memory and swap normally sit higher, so they warn
// later. Rate and count metrics (Disk I/O, Network, Processes) have no capacity
// ceiling and so get no classification — their panels stay healthy.
var (
	cpuHealth  = healthThresholds{warn: 50, crit: 75}
	memHealth  = healthThresholds{warn: 75, crit: 90}
	swapHealth = healthThresholds{warn: 50, crit: 80}
)

// Aggregate-badge thresholds: the overall reads warning once warnElevatedCount
// metrics are elevated (warning or critical), and critical once critCriticalCount
// are critical — or once none of the classified metrics remain healthy.
const (
	warnElevatedCount = 2
	critCriticalCount = 2
)

// aggregateHealth summarizes per-metric states into the overall badge state.
// An empty set (no classified metric wired) reads healthy.
func aggregateHealth(states []statusKind) statusKind {
	if len(states) == 0 {
		return status.Healthy
	}
	var healthy, elevated, critical int
	for _, s := range states {
		switch s {
		case status.Critical:
			critical++
			elevated++
		case status.Warning:
			elevated++
		default:
			healthy++
		}
	}
	switch {
	case healthy == 0 || critical >= critCriticalCount:
		return status.Critical
	case elevated >= warnElevatedCount:
		return status.Warning
	default:
		return status.Healthy
	}
}
