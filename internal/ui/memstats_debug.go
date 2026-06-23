package ui

// ponytail: temporary baseline instrumentation for the idle CPU/memory plans
// (docs/performance/*.md). Env-gated and self-contained so it deletes in one
// file once baselines are captured. Remove with its call in Run().

import (
	"context"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"time"
)

const (
	// memStatsEnv enables the logger when set to a non-empty value.
	memStatsEnv = "SYSMON_MEMSTATS"
	// memStatsFile is where the logger writes, sidestepping stderr/console
	// capture issues under a GUI app + PowerShell redirection.
	memStatsFile = "memstats.log"
	// bytesPerMiB scales runtime byte counts to MiB for readable logs.
	bytesPerMiB = 1 << 20
	// memStatsLogInterval is how often the live heap/Sys snapshot is logged.
	memStatsLogInterval = 3 * time.Second
	// memStatsScavengeInterval is how often FreeOSMemory is forced to confirm
	// the footprint is churn (RSS drops) rather than a leak (it doesn't).
	memStatsScavengeInterval = 30 * time.Second
)

// startMemStatsLogger spawns a goroutine that logs heap/Sys every few seconds
// and periodically scavenges, until ctx is cancelled. It is a no-op unless the
// memStatsEnv variable is set, so it costs nothing in normal runs.
func startMemStatsLogger(ctx context.Context, getenv func(string) string) {
	if getenv(memStatsEnv) == "" {
		return
	}
	f, err := os.Create(memStatsFile)
	if err != nil {
		log.Printf("memstats: cannot open %s: %v", memStatsFile, err)
		return
	}
	logger := log.New(f, "", log.LstdFlags)
	go runMemStatsLogger(ctx, logger, f)
}

func runMemStatsLogger(ctx context.Context, logger *log.Logger, f *os.File) {
	defer f.Close()
	logTicker := time.NewTicker(memStatsLogInterval)
	scavengeTicker := time.NewTicker(memStatsScavengeInterval)
	defer logTicker.Stop()
	defer scavengeTicker.Stop()

	logger.Printf("memstats: logging enabled (interval %s)", memStatsLogInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-logTicker.C:
			logMemStats(logger, "live")
		case <-scavengeTicker.C:
			logMemStats(logger, "pre-scavenge")
			debug.FreeOSMemory()
			logMemStats(logger, "post-scavenge")
		}
	}
}

func logMemStats(logger *log.Logger, label string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	logger.Printf("memstats[%s]: HeapAlloc=%dMiB HeapInuse=%dMiB Sys=%dMiB NumGC=%d",
		label, m.HeapAlloc/bytesPerMiB, m.HeapInuse/bytesPerMiB, m.Sys/bytesPerMiB, m.NumGC)
}
