package ui

import (
	"math"
	"runtime/debug"
)

const (
	// softMemoryLimitDefault is the soft heap cap applied at startup when the
	// operator hasn't set GOMEMLIMIT. A soft limit makes the GC collect more
	// aggressively as the heap nears it, trading a little CPU for a lower RSS
	// plateau — the opposite trade from the chart-cache fixes. It stays a soft
	// target: the runtime collects harder near the limit but never OOM-kills.
	//
	// 320 MiB sits below the observed ~449 MiB Sys plateau to pull RSS down, but
	// stays above the launch-time churn peak (the disk scanner crawls every
	// volume at startup, transiently pushing the live heap past 200 MiB). A
	// lower cap pins the heap at the limit during that crawl and thrashes the GC
	// (~55 collections/s); 320 MiB clears the transient so collection stays calm.
	softMemoryLimitDefault = 320 * bytesPerMiB

	// memoryLimitUnset is what debug.SetMemoryLimit(-1) reports when no limit is
	// in effect (no GOMEMLIMIT and no prior SetMemoryLimit): the runtime's
	// math.MaxInt64 sentinel.
	memoryLimitUnset = math.MaxInt64
)

// applyDefaultMemoryLimit installs softMemoryLimitDefault as the GC's soft
// memory target, but only when no limit is already in effect — an
// operator-supplied GOMEMLIMIT (read by the runtime at startup) must win so ops
// can override or disable the default. The getLimit/setLimit seam lets tests
// exercise the override logic without mutating the real runtime.
func applyDefaultMemoryLimit(getLimit func() int64, setLimit func(int64) int64) {
	if getLimit() != memoryLimitUnset {
		return // operator (or a prior call) set a limit; leave it alone.
	}
	setLimit(softMemoryLimitDefault)
}

// installDefaultMemoryLimit wires applyDefaultMemoryLimit to the real runtime.
// debug.SetMemoryLimit(-1) reads the current limit without changing it.
func installDefaultMemoryLimit() {
	applyDefaultMemoryLimit(func() int64 { return debug.SetMemoryLimit(-1) }, debug.SetMemoryLimit)
}
