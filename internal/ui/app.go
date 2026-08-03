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
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/systray"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/monitor"
	"github.com/josephheinz/system-monitor/internal/recorder"
	"github.com/josephheinz/system-monitor/internal/recorder/columns"
	"github.com/josephheinz/system-monitor/internal/series"
	"github.com/josephheinz/system-monitor/internal/update"
)

// pollInterval is the cadence at which collectors sample and the UI redraws,
// defaulting to 1s to match the ring buffers' 1-second resolution
// (metrics.HistoryCapacity). It is a var, not a const, because Run overrides it
// from the persisted poll-interval preference — at startup and again on a live
// Settings change (both on the UI goroutine); historySpan, the poller, and the
// status-bar poll label all read it, so the chosen cadence stays consistent
// across the time axes and chrome.
var pollInterval = time.Second

// historySpan is the wall-clock window the metric ring buffers cover — the
// span charts' time axes and "last N" panel titles describe. It derives from
// the same pair of constants the buffers and poller use, so the axes stay
// truthful if either changes.
func historySpan() time.Duration {
	return metrics.HistoryCapacity * pollInterval
}

const appName = "System Monitor"

// appID is the Fyne unique app ID — it keys Preferences storage and is the
// Windows AppUserModelID on toast notifications, so changing it loses saved
// settings.
const appID = "com.josephheinz.systemmonitor"

// Tray menu item labels (BZS253-76).
const (
	labelTrayShow = "Show"
	labelTrayQuit = "Quit"
)

// Run creates the application, starts metric collection, shows the main window,
// and blocks until it is closed. version is the build-time release version
// (main.version); a non-release value ("dev") disables the GitHub self-update
// check (BZS253-71).
func Run(version string) {
	a := app.NewWithID(appID)
	registerNotificationAppName()

	// Load persisted preferences before any UI or collector is built: the theme
	// palette, memory cap, and poll cadence are read here and applied at startup.
	// Settings changes made while running re-apply live through applyHooks below.
	prefs := newSettings(a.Preferences())
	applyTheme(prefs.theme())

	a.Settings().SetTheme(newTheme())
	// Taskbar/window icon: the same brandMark the title-bar logo uses, so the
	// window icon and the in-app logo match (and both get the heavier stroke).
	a.SetIcon(brandMark())
	w := a.NewWindow(appName)

	// One context governs collection and the UI refresh loop; cancelling it on
	// window close stops both cleanly.
	ctx, cancel := context.WithCancel(context.Background())

	// Cap the Go heap so the GC holds RSS down (trading a little CPU), unless the
	// operator already set GOMEMLIMIT. Applied before collectors run; capInstalled
	// remembers that we (not an operator) set it, so the live toggle below only
	// ever removes our own limit.
	capInstalled := false
	if prefs.memoryCapEnabled() {
		capInstalled = installDefaultMemoryLimit()
	}

	// Apply the persisted poll cadence before building charts and the poller, so
	// the time axes and the status-bar poll label all describe the chosen rate.
	pollInterval = prefs.pollInterval()

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
	// Static machine facts for the Settings "System" section, read once here
	// (BZS253-74). A failure leaves a zero summary; the panel shows dashes.
	hostSummary, err := monitor.Host(ctx)
	if err != nil {
		log.Printf("host info: %v", err)
	}

	// Live-apply hooks for the Settings tab. The struct is created (and handed
	// to the builders) now, but its fields are populated further down, once the
	// poller, shutdown path, and tray exist — a pointer so the late binding is
	// visible to the already-built Settings tab.
	apply := &applyHooks{}

	src := buildSources{
		charts:   make(liveSources),
		cpuInfo:  cpuMeta{cores: cpuInfo.Cores, model: cpuInfo.ModelName},
		settings: prefs,
		system:   toSystemInfo(hostSummary, cpuInfo.Cores, version),
		window:   w,
		apply:    apply,
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
		src.rescanDirs = ctrl.scanner.Rescan
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

	// Self-update (BZS253-71). CleanupOld clears any binary parked by a prior
	// swap. The controller is wired only on a real released build — a "dev" build
	// has no version to compare against, so both the check and its status-bar
	// affordance are skipped. shutdown is late-bound (the poller it stops doesn't
	// exist yet); the controller calls it after spawning the new binary.
	update.CleanupOld()
	var shutdown func()
	// Supported() gates Linux to self-updating launches (AppImage, writable bare
	// binary) plus dpkg-owned installs — the latter in "deferred" mode, where
	// the banner/pill appear but point at apt rather than self-updating
	// (InstallMode, issue #68).
	if update.IsRelease(version) && update.Supported() {
		mode := update.InstallMode()
		updater := update.NewController(version, mode, func() {
			if shutdown != nil {
				fyne.Do(shutdown)
			}
		})
		src.updateStatus = updater.Snapshot
		src.updateMode = mode
		switch mode {
		case update.ModeSelf:
			src.startUpdate = func() { updater.Start(ctx) }
		case update.ModePackageManager:
			// apt owns the binary; the affordance explains the upgrade command
			// rather than swapping the file under dpkg (issue #68).
			src.startUpdate = func() { showAptUpdateGuide(w) }
		}
		// Periodic update checks (issue #84): query GitHub on launch, then once
		// per the configured interval for as long as the app runs, so a release
		// that ships while the app is open shows up in the status bar/banner
		// without a relaunch. The interval is a Settings pref (updateCheckInterval,
		// launch-scoped: the loop reads it once when wired); "Off" (0) skips the
		// loop entirely — the app never queries GitHub during the session. The
		// loop skips while a release is already known (banner up — no point
		// re-querying the rate-limited API), degrades to a logged no-op when
		// offline or rate-limited, and stops with ctx on shutdown. Auto-install
		// keeps its startup semantics: a found release installs without a click
		// when the pref is on — the policy lives here (composition root), so the
		// controller stays unaware of preferences (ADR-010). A package-manager-
		// owned install never auto-installs: Start is a no-op for it, so the guard
		// is belt-and-suspenders.
		autoInstall := prefs.autoUpdateEnabled()
		if interval := prefs.updateCheckInterval(); interval > 0 {
			update.RunPeriodicChecks(ctx, updater, interval, func() {
				if autoInstall && mode == update.ModeSelf {
					updater.Start(ctx) // no-ops unless a periodic check found a newer release
				}
			})
		}
	}

	// Session tracking mode (BZS253-77): a recorder appends one CSV row per poll
	// tick while a user-started session is active — an explicit, user-initiated
	// export, distinct from the ambient in-memory history the no-persistence rule
	// governs (ADR-012). The column schema lives in recorder/columns so the
	// headless recording binary writes the identical format. The toggle opens a
	// modal whose choices (compact output, top-processes sidecar, save location)
	// build the session recorder; the recorder's data path stays Fyne-free.
	// Recorder options are construction-time, so each session gets a fresh
	// recorder; session points at whichever is live — nil when idle — and the
	// status bar, poller, and teardown all read it through atomic.Load, so the
	// swap between a stopped session and a new one can't race a poller tick
	// (a stopped recorder's Tick is a no-op). Registered as a third OnTick
	// observer below.
	var session atomic.Pointer[recorder.Recorder]
	startSession := func(spec recorder.OptionsSpec, path string) {
		if spec.Compact {
			path = columns.CompactFilePath(path)
		}
		opts := columns.SessionOptions(spec, procs, path)
		rec := recorder.New(columns.Build(cpu, memory, diskCol, network, procs), opts...)
		f, err := os.Create(path)
		if err != nil {
			log.Printf("create recording file: %v", err)
			return
		}
		if startErr := rec.Start(f); startErr != nil {
			log.Printf("start recording: %v", startErr)
			// Recorder didn't adopt it; close so the handle doesn't leak.
			if closeErr := f.Close(); closeErr != nil {
				log.Printf("close after failed start: %v", closeErr)
			}
			return
		}
		session.Store(rec)
	}
	recording := func() bool {
		if r := session.Load(); r != nil {
			return r.Recording()
		}
		return false
	}
	src.recording = recording
	src.toggleRecord = func() { toggleRecording(&session, w, startSession) }

	content, refresh := buildContent(src)

	// The shell draws its own chrome flush to the window edges, so suppress
	// Fyne's default padding around window content.
	w.SetPadded(false)
	w.SetContent(content)

	// rebuild reconstructs the whole widget tree from the same sources: the
	// structural settings (theme palette, poll cadence) are baked into widgets
	// at construction, so a live change re-bakes everything. Lazy tab building
	// keeps it cheap, and it lands back on the Settings tab — where the change
	// was made. Runs on the UI goroutine (a control callback), as does every
	// read of refresh (inside fyne.Do), so reassigning it here is race-free.
	rebuild := func() {
		settingsTab := tabSettings
		src.initialTab = &settingsTab
		var c fyne.CanvasObject
		c, refresh = buildContent(src)
		w.SetContent(c)
	}

	// Drive the redraw from the poller so the UI updates exactly once per poll,
	// right after fresh data lands. A separate UI ticker would run on its own
	// clock and drift against the poll clock, making the visible update cadence
	// beat between the two (sometimes <1s apart, sometimes >1s). The poller runs
	// the callback off the UI goroutine, so marshal the canvas work back with
	// fyne.Do (RingBuffer reads are concurrency-safe; touching the canvas is not).
	poller := monitor.NewPoller(pollInterval, collectors...)
	// Re-read refresh inside the closure (not captured by value): a rebuild
	// swaps in a new refresh for the new widget tree.
	poller.OnTick(func() { fyne.Do(func() { refresh() }) })

	// Threshold notifications (BZS253-75): a pure, Fyne-free watcher reads the
	// same live values the charts read and sends a native OS notification the
	// tick usage crosses up through a user-set threshold, re-arming when it
	// drops back below. Registered as a second OnTick observer — no new
	// collector, no Run() wiring beyond this. SendNotification is safe off the
	// UI goroutine, so unlike the redraw it needs no fyne.Do.
	watcher := newThresholdWatcher(prefs, thresholdReaders{
		cpu:    cpuUsageReader(cpu),
		memory: memUsageReader(memory),
		disk:   diskUsageReader(diskCol),
	}, func(title, body string) {
		a.SendNotification(fyne.NewNotification(title, body))
	})
	poller.OnTick(watcher.tick)

	// Session tracking (BZS253-77): the recorder writes one row per tick while
	// active. A third OnTick observer — no new collector, no Run() wiring beyond
	// this. Tick is inert until a session starts, so it costs nothing when idle;
	// a nil session (no session yet, or one just swapped out) also no-ops.
	poller.OnTick(func() {
		if r := session.Load(); r != nil {
			r.Tick()
		}
	})

	poller.Start(ctx)

	// One teardown path, shared by the window's close button and a self-update
	// restart (which needs to exit this process so the new binary takes over).
	shutdown = func() {
		cancel()
		poller.Stop()
		// Flush and close any in-progress tracking session so a quit mid-recording
		// still yields a complete file. No-op when not recording. After poller.Stop
		// so no Tick races the close.
		if r := session.Load(); r != nil {
			if err := r.Stop(); err != nil {
				log.Printf("stop recording: %v", err)
			}
		}
		// a.Quit, not w.Close: with a system tray active Fyne keeps the app
		// running after the last window closes, so closing the window would
		// leave a zombie process behind the tray icon (BZS253-76). Quit stops
		// the run loop and removes the tray icon on every path.
		a.Quit()
	}
	w.SetCloseIntercept(shutdown)

	// Minimize to tray (BZS253-76): when opted in and the driver has a tray
	// (desktop.App), closing the window hides it — collection keeps running —
	// and the tray menu carries the two intents: Show restores the window, Quit
	// runs the one real teardown above (whose a.Quit also removes the tray
	// icon, so neither tray-Quit nor a self-update restart leaves a ghost
	// icon). Non-desktop drivers fail the assertion, keep quit-on-close, and
	// leave the tray hook nil (the toggle then only persists).
	// No explicit SetSystemTrayIcon: the tray isn't initialized until
	// SetSystemTrayMenu runs (an eager set logs "tray not ready yet"), and once
	// ready Fyne applies the app icon — already brandMark via a.SetIcon above.
	if desk, ok := a.(desktop.App); ok {
		enableTray := func() {
			desk.SetSystemTrayMenu(fyne.NewMenu(appName,
				fyne.NewMenuItem(labelTrayShow, func() { fyne.Do(w.Show) }),
				fyne.NewMenuItem(labelTrayQuit, func() { fyne.Do(shutdown) }),
			))
			w.SetCloseIntercept(w.Hide)
		}
		if prefs.minimizeToTrayEnabled() {
			enableTray()
			// Hover tooltip on the tray icon. Fyne doesn't expose this, but its
			// own tray backend (fyne.io/systray — already linked into the binary)
			// does. Deferred to OnStarted because the icon must exist first: Fyne
			// starts the tray at the top of its run loop, before firing OnStarted,
			// on both Windows (ready even earlier, inside SetSystemTrayMenu) and
			// macOS (status item created synchronously in trayStart).
			a.Lifecycle().SetOnStarted(func() { systray.SetTooltip(appName) })
		}
		apply.tray = func(on bool) {
			if !on {
				// ponytail: Fyne has no tray-removal API, so a mid-run disable
				// leaves the icon until exit — but close-to-quit is restored
				// immediately, which is the behavior that matters.
				w.SetCloseIntercept(shutdown)
				return
			}
			enableTray()
			// Mid-run the app is already started, so the tray exists as soon as
			// the menu is set — the tooltip can be applied directly.
			systray.SetTooltip(appName)
		}
	}

	// Populate the remaining live-apply hooks now that the window, poller, and
	// teardown all exist (the Settings tab reads them through the shared
	// pointer). Each runs on the UI goroutine — Fyne control callbacks.
	apply.theme = func(c themeChoice) {
		applyTheme(c)
		// Re-set the theme so Fyne's stock widgets re-read the rebuilt color map;
		// the app's own palette-baked widgets are re-baked by the rebuild.
		a.Settings().SetTheme(newTheme())
		rebuild()
	}
	apply.poll = func(d time.Duration) {
		pollInterval = d
		poller.Stop()
		poller.SetInterval(d)
		poller.Start(ctx)
		rebuild() // time axes and the status-bar poll label bake the cadence in
	}
	apply.memCap = func(on bool) {
		if on {
			capInstalled = installDefaultMemoryLimit()
			return
		}
		if capInstalled {
			removeMemoryLimit()
			capInstalled = false
		}
	}

	w.Resize(defaultWindowSize())
	w.CenterOnScreen()

	// First launch after a version change shows the bundled "What's New" page as
	// a fullscreen overlay (BZS253-78); a fresh install records the version
	// silently. Reuses the same build version the updater compares against.
	maybeShowWhatsNew(w.Canvas(), prefs, version, os.Getenv)

	w.ShowAndRun()
}

// toSystemInfo maps the static host summary (plus the logical core count and
// build version, which come from other sources) into the UI's systemInfo, so
// the ui package never imports the monitor concrete. The OS row joins the
// platform product and version into one line; the rest is a field copy.
func toSystemInfo(h monitor.HostSummary, cores int, version string) systemInfo {
	return systemInfo{
		hostname: h.Hostname,
		os:       strings.TrimSpace(h.Platform + " " + h.PlatformVersion),
		kernel:   h.KernelVersion,
		bootTime: h.BootTime,
		uptime:   h.Uptime,
		cores:    cores,
		users:    h.Users,
		version:  version,
	}
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
			status: procStatusOf(p.State),
		}
	}
	return rows
}

// procStatusOf maps a monitor process state onto its UI display vocabulary.
// The monitor layer carries state as an opaque enum; the display name is a UI
// concern, so the switch lives here. StateUnknown maps to the empty status.
func procStatusOf(s monitor.ProcessState) procStatus {
	switch s {
	case monitor.StateRunning:
		return statusRunning
	case monitor.StateSleeping:
		return statusSleeping
	case monitor.StateStopped:
		return statusStopped
	default:
		return ""
	}
}

// byCPUDesc and byMemoryDesc are the hottest-first orderings for the CPU and
// Memory tabs' top-process tables.
func byCPUDesc(a, b monitor.ProcessInfo) bool    { return a.CPUPercent > b.CPUPercent }
func byMemoryDesc(a, b monitor.ProcessInfo) bool { return a.MemoryBytes > b.MemoryBytes }

// The three usage readers below feed the threshold watcher (BZS253-75) from the
// same collectors the charts read; each returns ok=false when its collector
// failed to start or hasn't produced a sample yet, so the watcher skips the rule
// rather than firing on a zero. They live in the composition root because it is
// the only place that knows the collector concretes.

// lastSample returns the most recent value of a usage history, false when empty.
func lastSample(v []float64) (float64, bool) {
	if len(v) == 0 {
		return 0, false
	}
	return v[len(v)-1], true
}

// cpuUsageReader reads the latest overall-CPU percentage.
func cpuUsageReader(c *monitor.CPUCollector) usageReader {
	return func() (float64, bool) {
		if c == nil {
			return 0, false
		}
		return lastSample(c.Overall())
	}
}

// memUsageReader reads current memory use as a percentage of total.
func memUsageReader(c *monitor.MemoryCollector) usageReader {
	return func() (float64, bool) {
		if c == nil {
			return 0, false
		}
		used, total := c.Used(), c.Total()
		if len(used) == 0 || total == 0 {
			return 0, false
		}
		return usagePercent(float64(used[len(used)-1]), total), true
	}
}

// diskUsageReader reads the busiest volume's used percentage — matching the
// Overview disk panel's "most-used volume" choice (BZS253-65).
func diskUsageReader(c *monitor.DiskCollector) usageReader {
	return func() (float64, bool) {
		if c == nil {
			return 0, false
		}
		return mostUsedPercent(c.Usage())
	}
}

// mostUsedPercent returns the highest used percentage across the volumes, false
// when none has real capacity.
func mostUsedPercent(parts []monitor.PartitionUsage) (float64, bool) {
	var top float64
	found := false
	for _, p := range parts {
		if p.Total == 0 {
			continue
		}
		if pct := usagePercent(float64(p.Used), p.Total); pct > top {
			top = pct
		}
		found = true
	}
	return top, found
}

// Session tracking mode (BZS253-77). The recorder wiring lives in the
// composition root because it is the only place that knows the live collectors
// and the window the record modal needs. The CSV schema and the session-option
// assembly (compact output, top-processes sidecar) live in
// internal/recorder/columns, shared with the headless recording binary, so both
// entry points start byte-identical sessions; this file supplies the window and
// wires the modal's confirmed spec into columns.SessionOptions.

// Native save-dialog chrome (zenity), reused by the record modal's Browse.
const (
	recordDialogTitle   = "Save tracking session"
	recordFilterName    = "CSV files"
	recordFilterPattern = "*.csv"
)

// toggleRecording is the status-bar toggle's action: stop an active session, or
// start a new one behind the record modal. While a session is active a tap
// stops it outright (no modal); idle, it opens the modal, whose confirm starts
// the session through start and whose cancel leaves it idle. Runs on the UI
// goroutine (a control tap), so the modal may call dialog directly.
func toggleRecording(session *atomic.Pointer[recorder.Recorder], win fyne.Window, start func(recorder.OptionsSpec, string)) {
	if r := session.Load(); r != nil && r.Recording() {
		if err := r.Stop(); err != nil {
			log.Printf("stop recording: %v", err)
		}
		session.Store(nil)
		return
	}
	showRecordModal(win, start)
}
