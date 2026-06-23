package ui

import (
	"testing"

	"github.com/josephheinz/system-monitor/internal/series"
)

// TestStatusBarHealthAggregates checks the footer's overall state uses the shared
// aggregate model across CPU, memory, and swap: a single elevated metric is not
// enough to leave healthy, but two are.
func TestStatusBarHealthAggregates(t *testing.T) {
	const total = 100.0 // bytes; used/total drives the percentage
	at := func(v float64) series.Source {
		return series.SourceFunc(func() []float64 { return []float64{v} })
	}
	mem := func(usedPct float64) memSources {
		return memSources{used: at(usedPct), cached: at(0), free: at(0), total: total}
	}
	swap := func(usedPct float64) swapSources {
		return swapSources{used: at(usedPct), total: total}
	}

	// Only CPU elevated (warning) -> overall stays healthy.
	calm := &statusBarView{cpu: at(60), mem: mem(0), swap: swap(0)}
	if got := calm.health(); got != status.Healthy {
		t.Errorf("one elevated metric: health() = %v, want Healthy", got)
	}

	// CPU and memory both elevated -> overall warns.
	busy := &statusBarView{cpu: at(60), mem: mem(80), swap: swap(0)}
	if got := busy.health(); got != status.Warning {
		t.Errorf("two elevated metrics: health() = %v, want Warning", got)
	}
}
