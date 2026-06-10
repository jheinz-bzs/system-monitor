// Package ui builds and runs the System Monitor application window.
//
// The window hosts a persistent shell — a title bar, a vertical tab navigation,
// and a status bar — with one tab per metric area (Overview, CPU, Memory, Disk,
// Network, Processes, Ports, Connections). Tabs go live as their collectors are
// wired in; the CPU tab is the first, fed by a poller-driven CPUCollector.
package ui

import (
	"context"
	"log"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/monitor"
	"github.com/josephheinz/system-monitor/internal/series"
)

// pollInterval is the cadence at which collectors sample and the UI redraws:
// 1s, matching the ring buffers' 1-second resolution (metrics.HistoryCapacity).
const pollInterval = time.Second

// historySpan is the wall-clock window the metric ring buffers cover — the
// span charts' time axes and "last N" panel titles describe. It derives from
// the same pair of constants the buffers and poller use, so the axes stay
// truthful if either changes.
func historySpan() time.Duration {
	return metrics.HistoryCapacity * pollInterval
}

const appName = "System Monitor"

// Run creates the application, starts metric collection, shows the main window,
// and blocks until it is closed.
func Run() {
	a := app.NewWithID("com.josephheinz.systemmonitor")
	a.Settings().SetTheme(newTheme())
	w := a.NewWindow(appName)

	// One context governs collection and the UI refresh loop; cancelling it on
	// window close stops both cleanly.
	ctx, cancel := context.WithCancel(context.Background())

	// Build the live collectors and adapt their data into the UI sources.
	// A collector that fails to start is nil; its tab falls back to the
	// placeholder rather than crashing.
	cpu := monitor.NewCPUCollector(ctx)
	memory := monitor.NewMemoryCollector(ctx)
	procs, err := monitor.NewProcessCollector(ctx)
	if err != nil {
		log.Printf("process collector: %v", err)
	}
	cpuInfo, err := monitor.CPUInfo(ctx)
	if err != nil {
		log.Printf("cpu info: %v", err) // subtitle is omitted; the tab still works
	}

	src := buildSources{
		charts:  make(liveSources),
		cpuInfo: cpuMeta{cores: cpuInfo.Cores, model: cpuInfo.ModelName},
	}
	var collectors []monitor.Collector
	if cpu != nil {
		src.charts[tabCPU] = series.SourceFunc(cpu.Overall)
		collectors = append(collectors, cpu)
	}
	if memory != nil {
		src.mem = memSources{
			used:   series.SourceOf(memory.Used),
			cached: series.SourceOf(memory.Cached),
			free:   series.SourceOf(memory.Free),
			total:  memory.Total(),
		}
		collectors = append(collectors, memory)
	}
	if procs != nil {
		src.procs = processSourceFunc(func(n int) []processRow {
			return topNByCPU(procs.Processes(), n)
		})
		collectors = append(collectors, procs)
	}

	content, refresh := buildContent(src)

	// The shell draws its own chrome flush to the window edges, so suppress
	// Fyne's default padding around window content.
	w.SetPadded(false)
	w.SetContent(content)

	// Drive the redraw from the poller so the UI updates exactly once per poll,
	// right after fresh data lands. A separate UI ticker would run on its own
	// clock and drift against the poll clock, making the visible update cadence
	// beat between the two (sometimes <1s apart, sometimes >1s). The poller runs
	// the callback off the UI goroutine, so marshal the canvas work back with
	// fyne.Do (RingBuffer reads are concurrency-safe; touching the canvas is not).
	poller := monitor.NewPoller(pollInterval, collectors...)
	poller.OnTick(func() { fyne.Do(refresh) })
	poller.Start(ctx)

	w.SetCloseIntercept(func() {
		cancel()
		poller.Stop()
		w.Close()
	})

	w.Resize(defaultWindowSize())
	w.CenterOnScreen()
	w.ShowAndRun()
}

// topNByCPU adapts monitor.ProcessInfo to the UI's processRow type.
// Lives in app.go — the composition root — because that is the only place
// that knows both the monitor and ui concrete types.
func topNByCPU(procs []monitor.ProcessInfo, n int) []processRow {
	sorted := make([]monitor.ProcessInfo, len(procs))
	copy(sorted, procs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CPUPercent > sorted[j].CPUPercent
	})
	if n < len(sorted) {
		sorted = sorted[:n]
	}
	rows := make([]processRow, len(sorted))
	for i, p := range sorted {
		rows[i] = processRow{
			pid:  PID(p.PID),
			name: p.Name,
			user: p.Username,
			cpu:  p.CPUPercent,
			mem:  p.MemoryBytes,
		}
	}
	return rows
}
