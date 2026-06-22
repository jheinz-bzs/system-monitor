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
	"os"
	"path/filepath"
	"strings"
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

	// Optional baseline instrumentation for the perf plans; no-op unless enabled.
	startMemStatsLogger(ctx, os.Getenv)

	// Build the live collectors and adapt their data into the UI sources.
	// A collector that fails to start is nil; its tab falls back to the
	// placeholder rather than crashing.
	cpu := monitor.NewCPUCollector(ctx)
	memory := monitor.NewMemoryCollector(ctx)
	diskCol := monitor.NewDiskCollector(ctx)
	network := monitor.NewNetworkCollector(ctx)
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
		src.cpuCores = coreSources(cpu)
		collectors = append(collectors, cpu)
	}
	if memory != nil {
		src.mem = memSources{
			used:   series.SourceOf(memory.Used),
			cached: series.SourceOf(memory.Cached),
			free:   series.SourceOf(memory.Free),
			total:  memory.Total(),
		}
		src.swap = swapSources{used: series.SourceOf(memory.SwapUsed), total: memory.SwapTotal()}
		collectors = append(collectors, memory)
	}
	if diskCol != nil {
		src.disk = diskUsageSourceFunc(func() []diskPartition {
			return toDiskPartitions(diskCol.Usage())
		})
		src.diskIO = diskIOSources{
			total: series.SourceFunc(sumRates(diskCol.ReadRate, diskCol.WriteRate)),
			read:  series.SourceOf(diskCol.ReadRate),
			write: series.SourceOf(diskCol.WriteRate),
		}
		// The directory treemap is fed by background filesystem walks (the disk
		// poll tick is far too fast for them). The scanner crawls every volume
		// once at launch and caches each result per volume; switching volumes
		// then just shows that volume's cache. The controller bridges the scanner
		// to the UI's directory seam; ctx cancellation stops the walk goroutine.
		ctrl := &diskScanController{
			scanner: monitor.NewDiskUsageScanner(ctx, scanRoots(diskCol.Usage())),
		}
		src.diskDirs = ctrl
		src.selectVolume = ctrl.selectVolume
		collectors = append(collectors, diskCol)
	}
	if network != nil {
		src.net = netSources{
			total:    series.SourceOf(network.TotalRate),
			upload:   series.SourceOf(network.UploadRate),
			download: series.SourceOf(network.DownloadRate),
		}
		collectors = append(collectors, network)
	}
	if procs != nil {
		// One feed for all three process tables: the full list, unordered. Each
		// table's model applies its own top-N selection, sort, and filtering.
		src.allProcs = allProcessSourceFunc(func() []processRow {
			return toProcessRows(procs.Processes())
		})
		// Ports resolve their owning-process name from the same process snapshot
		// (no separate gopsutil call), so the adapter joins both reads per tick.
		src.ports = allPortsSourceFunc(func() []portRow {
			return toPortRows(procs.Ports(), procs.Processes())
		})
		// Connections resolve their owning-process name from the same process
		// snapshot, exactly like ports, so the adapter joins both reads per tick.
		src.conns = allConnsSourceFunc(func() []connRow {
			return toConnRows(procs.Connections(), procs.Processes())
		})
		src.killProc = processKillerFunc(func(pid PID) error {
			return procs.Terminate(ctx, int32(pid))
		})
		src.procCount = series.SourceOf(procs.Count)
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

// coreSources adapts each logical core's usage history into a series.Source,
// in core order. Lives in app.go — the composition root — because that is the
// only place that knows the CPUCollector concrete type.
func coreSources(cpu *monitor.CPUCollector) []series.Source {
	out := make([]series.Source, cpu.CoreCount())
	for i := range out {
		out[i] = series.SourceFunc(func() []float64 { return cpu.Core(i) })
	}
	return out
}

// toDiskPartitions adapts monitor.PartitionUsage records to the UI's
// diskPartition type, order preserved, so the Disk view never imports monitor.
// The returned slice is freshly allocated per call.
func toDiskPartitions(usage []monitor.PartitionUsage) []diskPartition {
	parts := make([]diskPartition, len(usage))
	for i, u := range usage {
		parts[i] = diskPartition{mount: u.Mountpoint, total: u.Total, used: u.Used}
	}
	return parts
}

// diskScanController bridges the background directory scanner to the UI's
// diskDirSource seam. It lives in the composition root because it's the only
// place that knows both the monitor.DiskUsageScanner concrete and the UI seam;
// the scanner guards its own snapshot.
type diskScanController struct {
	scanner *monitor.DiskUsageScanner
}

// dirs adapts the scanner's latest snapshot into the UI's directory shape,
// labeling each bucket by its name under the scan root.
func (c *diskScanController) dirs() []diskDir {
	snap := c.scanner.Dirs()
	out := make([]diskDir, len(snap))
	for i, d := range snap {
		out[i] = diskDir{label: filepath.Base(d.Path), path: d.Path, bytes: d.Bytes}
	}
	return out
}

// selectVolume retargets the scan at mount.
func (c *diskScanController) selectVolume(mount string) {
	c.scanner.SetRoot(mount)
}

// scanRoots lists every volume with real capacity, in usage order — the set the
// directory scanner crawls at launch and the user switches between. The first
// entry is the initially displayed volume, matching the volume selector's
// default (first) segment, so the highlighted segment and the shown volume
// agree at startup.
func scanRoots(usage []monitor.PartitionUsage) []string {
	var roots []string
	for _, u := range usage {
		if u.Total > 0 {
			roots = append(roots, u.Mountpoint)
		}
	}
	return roots
}

// sumRates returns a snapshot func adding two rate histories element-wise — the
// I/O chart's "total" series from the read and write rate buffers. The buffers
// share a length (one append per tick), but the shorter one is honored
// defensively so a transient mismatch can't panic.
func sumRates(read, write func() []uint64) func() []float64 {
	return func() []float64 {
		r, w := read(), write()
		out := make([]float64, len(r))
		for i := range r {
			out[i] = float64(r[i])
			if i < len(w) {
				out[i] += float64(w[i])
			}
		}
		return out
	}
}

// toPortRows adapts monitor.PortInfo records to the UI's portRow type, resolving
// each port's owning-process name from the process snapshot by PID — the same
// snapshot the process tables read, so no extra gopsutil call is made. A port
// whose PID is absent from the snapshot (permission-restricted or already exited)
// gets an empty name, which the table renders as the unresolved-owner dash. The
// returned slice is freshly allocated per call, as allPortsSource requires.
func toPortRows(ports []monitor.PortInfo, procs []monitor.ProcessInfo) []portRow {
	names := make(map[int32]string, len(procs))
	for _, p := range procs {
		names[p.PID] = p.Name
	}
	rows := make([]portRow, len(ports))
	for i, p := range ports {
		rows[i] = portRow{
			proto:     toPortProto(p.Protocol),
			port:      p.Port,
			process:   names[p.PID],
			pid:       PID(p.PID),
			localAddr: p.LocalAddr,
		}
	}
	return rows
}

// toPortProto maps the monitor protocol vocabulary to the UI's display protocol.
// An unrecognized protocol passes through uppercased rather than being dropped.
func toPortProto(p monitor.Protocol) portProto {
	switch p {
	case monitor.ProtocolTCP:
		return portProtoTCP
	case monitor.ProtocolUDP:
		return portProtoUDP
	default:
		return portProto(strings.ToUpper(string(p)))
	}
}

// toConnRows adapts monitor.ConnectionInfo records to the UI's connRow type,
// resolving each connection's owning-process name from the process snapshot by
// PID — the same snapshot the process and ports tables read, so no extra gopsutil
// call is made. A connection whose PID is absent from the snapshot gets an empty
// name, which the table renders as the unresolved-owner dash. The returned slice
// is freshly allocated per call, as allConnsSource requires.
func toConnRows(conns []monitor.ConnectionInfo, procs []monitor.ProcessInfo) []connRow {
	names := make(map[int32]string, len(procs))
	for _, p := range procs {
		names[p.PID] = p.Name
	}
	rows := make([]connRow, len(conns))
	for i, c := range conns {
		rows[i] = connRow{
			proto:      toPortProto(c.Protocol),
			localAddr:  c.LocalAddr,
			remoteAddr: c.RemoteAddr,
			state:      connState(c.State),
			process:    names[c.PID],
			pid:        PID(c.PID),
		}
	}
	return rows
}

// toProcessRows adapts monitor.ProcessInfo records to the UI's processRow
// type, order preserved. The returned slice is freshly allocated per call, as
// allProcessSource requires (the table models sort and slice it in place).
func toProcessRows(procs []monitor.ProcessInfo) []processRow {
	rows := make([]processRow, len(procs))
	for i, p := range procs {
		rows[i] = processRow{
			pid:    PID(p.PID),
			name:   p.Name,
			user:   p.Username,
			cpu:    p.CPUPercent,
			mem:    p.MemoryBytes,
			status: procStatus(p.State),
		}
	}
	return rows
}
