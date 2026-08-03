// Command system-monitor-record is the headless recording agent: it runs the
// same session-tracking mode as the desktop app's record button (BZS253-77),
// writing one CSV row per poll tick until interrupted, with no GUI and no cgo —
// built for servers, where the Fyne binary can't even start. The output opens
// unchanged in the desktop app's Recordings tab.
//
// It is deliberately not a daemon: it runs in the foreground and stops on
// SIGINT/SIGTERM, leaving supervision (restarts, logging) to systemd or the
// Task Scheduler.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/josephheinz/system-monitor/internal/monitor"
	"github.com/josephheinz/system-monitor/internal/recorder"
	"github.com/josephheinz/system-monitor/internal/recorder/columns"
)

// Flag defaults. The interval floor matches the recorder's RFC3339 second-
// resolution timestamps — a faster cadence would write duplicate-looking rows.
const (
	defaultInterval = time.Second
	minInterval     = time.Second
	untilInterrupt  = 0 // --duration sentinel: record until a signal arrives
	processesOff    = 0 // --processes sentinel: no top-processes sidecar
)

// Flag usage strings.
const (
	usageOut       = "CSV output path (default \"tracking-<timestamp>.csv\" in the working directory)"
	usageInterval  = "sampling cadence, minimum 1s (e.g. 1s, 5s)"
	usageDuration  = "stop after this long (0 = record until interrupted)"
	usageCompact   = "gzip-compress the output (.csv.gz) instead of plain CSV"
	usageProcesses = "also record the top N processes by CPU to a <output>.processes.csv sidecar each tick (0 = off)"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run wires collectors, poller, and recorder — the same composition the GUI
// performs in ui.Run — then blocks until the duration elapses or a signal lands.
func run() error {
	out := flag.String("out", "", usageOut)
	interval := flag.Duration("interval", defaultInterval, usageInterval)
	duration := flag.Duration("duration", untilInterrupt, usageDuration)
	compact := flag.Bool("compact", false, usageCompact)
	topN := flag.Int("processes", processesOff, usageProcesses)
	flag.Parse()

	if *interval < minInterval {
		log.Printf("interval %s below minimum; using %s", *interval, minInterval)
		*interval = minInterval
	}
	path := *out
	if path == "" {
		path = columns.FileName(time.Now())
	}
	if *compact {
		path = columns.CompactFilePath(path)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cpu, memory, disk, network, procs, collectors := buildCollectors(ctx)
	if len(collectors) == 0 {
		return errors.New("no collectors available; nothing to record")
	}
	if *topN > 0 && procs == nil {
		log.Printf("--processes %d ignored: process collector unavailable", *topN)
	}

	rec := recorder.New(
		columns.Build(cpu, memory, disk, network, procs),
		columns.SessionOptions(recorder.OptionsSpec{Compact: *compact, TopN: *topN}, procs, path)...,
	)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := rec.Start(f); err != nil {
		// Recorder didn't adopt it; close so the handle doesn't leak.
		if closeErr := f.Close(); closeErr != nil {
			log.Printf("close after failed start: %v", closeErr)
		}
		return err
	}

	// Start the recorder before the poller so the poller's immediate first
	// collect lands in the file; register Tick before Start (the poller's
	// observer contract).
	poller := monitor.NewPoller(*interval, collectors...)
	poller.OnTick(rec.Tick)
	poller.Start(ctx)
	log.Printf("recording to %s every %s (interrupt to stop)", path, *interval)

	wait(ctx, *duration)

	poller.Stop()
	if err := rec.Stop(); err != nil {
		return err
	}
	log.Printf("recording saved: %s", path)
	return nil
}

// buildCollectors constructs every metric collector, tolerating the ones that
// can't start — their columns record 0, matching the GUI's degrade-quietly
// behavior. The individual returns feed the column builder; the slice holds
// only the live ones for the poller.
func buildCollectors(ctx context.Context) (*monitor.CPUCollector, *monitor.MemoryCollector, *monitor.DiskCollector, *monitor.NetworkCollector, *monitor.ProcessCollector, []monitor.Collector) {
	cpu := monitor.NewCPUCollector(ctx)
	memory := monitor.NewMemoryCollector(ctx)
	disk := monitor.NewDiskCollector(ctx)
	network := monitor.NewNetworkCollector(ctx)
	procs, err := monitor.NewProcessCollector(ctx)
	if err != nil {
		log.Printf("process collector unavailable (proc_count records 0): %v", err)
	}

	// Concrete nil checks (not through the interface), matching ui.Run: a failed
	// constructor's columns simply record 0.
	var collectors []monitor.Collector
	if cpu != nil {
		collectors = append(collectors, cpu)
	}
	if memory != nil {
		collectors = append(collectors, memory)
	}
	if disk != nil {
		collectors = append(collectors, disk)
	}
	if network != nil {
		collectors = append(collectors, network)
	}
	if procs != nil {
		collectors = append(collectors, procs)
	}
	return cpu, memory, disk, network, procs, collectors
}

// wait blocks until the recording window closes: the duration elapses (when
// bounded) or the signal context is cancelled.
func wait(ctx context.Context, duration time.Duration) {
	if duration == untilInterrupt {
		<-ctx.Done()
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
}
