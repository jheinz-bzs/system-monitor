package ui

// Shell assembly: the persistent application chrome that hosts the eight tabs,
// laid out to match the design-system wireframes.
//
//	┌─────────────────────────────────┐
//	│ title bar (38px, surface-2)     │
//	├──────────┬──────────────────────┤  ← 1px border dividers between regions
//	│ sidebar  │  tab content         │
//	│ (178px,  │                      │
//	│ surface) │                      │
//	├──────────┴──────────────────────┤
//	│ status bar (26px, surface-2)    │
//	└─────────────────────────────────┘
//
// The bars and dividers go flush to the window edges (the window is created
// unpadded). Fyne's stock Border/Box layouts insert theme padding between
// regions, so a zero-padding border layout (tightBorderLayout) is used to keep
// the chrome flush, with the 1px dividers drawn explicitly.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/josephheinz/system-monitor/internal/series"
)

const (
	titleBarHeight  = 10 * spaceSM // 40; title bar height (38 rounded to grid)
	statusBarHeight = space2XL     // 24; status bar height (26 rounded to grid)
	barHPad         = spaceXL      // 16; leading inset for bar text (--sm-4)
	titleLogoSize   = spaceXL      // 16; accent diamond logo mark (14 rounded to grid)
	titleLogoGap    = spaceMD      // 8; gap between logo and wordmark (--sm-2)
)

// tabID identifies a tab by role rather than by display string, so content
// routing switches on a typed enum instead of matching a name literal.
type tabID uint

const (
	tabOverview tabID = iota
	tabCPU
	tabMemory
	tabDisk
	tabNetwork
	tabProcesses
	tabPorts
	tabConnections
)

// tabDef describes one nav entry: its identity, label, nav icon, and the
// content panes shown when it's selected. content is populated by the newTabs
// builder (via addChild) rather than at literal-declaration time, so the panes
// are built fresh per call and never shared across invocations.
type tabDef struct {
	id      tabID
	name    string
	icon    fyne.Resource
	content []fyne.CanvasObject
}

// addChild appends a content pane to the tab.
func (t *tabDef) addChild(child fyne.CanvasObject) {
	t.content = append(t.content, child)
}

// liveSources carries the time-series Sources for live chart tabs, keyed by
// tabID. A nil entry means that metric isn't wired yet; the tab falls back to
// its placeholder. New chart tabs add a map entry in app.go only.
type liveSources map[tabID]series.Source

// buildSources bundles all live data sources the tab builders need. Extend
// this struct (not the tabBuilder signature) when new source types are added.
type buildSources struct {
	charts       liveSources        // time-series chart sources, keyed by tabID
	cpuCores     []series.Source    // per-core CPU sources, core order; empty when not wired
	allProcs     allProcessSource   // full process list, feeding all process tables; nil when not wired
	procCount    series.Source      // process-count history for the Overview sparkline; nil when not wired
	ports        allPortsSource     // listening-port list feeding the Ports table; nil when not wired
	conns        allConnsSource     // active-connection list feeding the Connections table; nil when not wired
	killProc     processKiller      // process termination; nil when not wired
	disk         diskUsageSource    // per-partition usage feeding the volumes list; nil when not wired
	diskDirs     diskDirSource      // selected-volume directory sizes feeding the storage treemap; nil when not wired
	diskIO       diskIOSources      // disk read/write/total rate series; zero when not wired
	net          netSources         // network upload/download/total rate series; zero when not wired
	selectVolume func(mount string) // retargets the directory scan; nil when not wired
	cpuInfo      cpuMeta            // static processor description; zero when unknown
	mem          memSources         // memory band sources + total; zero when not wired
	swap         swapSources        // Overview swap usage source + total; zero when not wired
	nav          *crossNav          // cross-tab navigation target; populated by buildContent
}

// processNavigator is the cross-tab navigation seam the CPU and Memory tabs
// depend on to jump to — and optionally highlight a process row in — the
// Processes tab. Defined at the consumer per idiomatic Go; crossNav is the
// implementation buildContent supplies once the shell is assembled.
type processNavigator interface {
	showProcesses()
	showProcess(pid PID)
}

// crossNav is the late-bound cross-tab navigation target shared by the tab
// builders. Its action stays nil until buildContent has assembled the nav and
// located the Processes tab, so a builder can capture it before the shell
// exists. Calls made before wiring — or in a build with no Processes tab —
// no-op, so the navigator degrades like the tabs' other nil fallbacks.
type crossNav struct {
	selectProcess func(pid PID, highlight bool)
	selectTab     func(id tabID)
}

// tabNavigator is the seam the Overview panels depend on to jump to a metric's
// own tab when clicked. Defined at the consumer; crossNav implements it once the
// shell is assembled.
type tabNavigator interface {
	showTab(id tabID)
}

// showTab switches to the tab with the given id once navigation is wired;
// otherwise it no-ops (like the process-nav fallbacks).
func (n *crossNav) showTab(id tabID) {
	if n.selectTab != nil {
		n.selectTab(id)
	}
}

// showProcesses jumps to the Processes tab without changing its selection — the
// CPU tab's "→ all processes" link.
func (n *crossNav) showProcesses() { n.jump(0, false) }

// showProcess jumps to the Processes tab and highlights pid — a tapped CPU- or
// Memory-tab process row landing on its owning process.
func (n *crossNav) showProcess(pid PID) { n.jump(pid, true) }

// jump performs the navigation when it has been wired; otherwise it no-ops.
func (n *crossNav) jump(pid PID, highlight bool) {
	if n.selectProcess != nil {
		n.selectProcess(pid, highlight)
	}
}

// tabContent is the built content for one tab: the object to display, an
// optional refresh callback (nil for static tabs that never update), and an
// optional cross-tab entry point selecting a process by PID (only the
// Processes tab populates it; buildContent collects these so the crossNav can
// land a jump link or a tapped row on a highlighted process).
type tabContent struct {
	object    fyne.CanvasObject
	refresh   func()
	selectPID func(PID)
}

// tabBuilder constructs a tab's content from the available live sources.
type tabBuilder func(src buildSources) tabContent

// tabRegistry maps tab IDs to their builder functions. To add a new live tab,
// register its builder here — newTabs is never edited for new metric areas.
var tabRegistry = map[tabID]tabBuilder{
	tabOverview: func(src buildSources) tabContent {
		v := newOverviewView(overviewSources{
			cpu:      src.charts[tabCPU],
			cpuCores: src.cpuInfo.cores,
			mem:      src.mem,
			diskIO:   src.diskIO,
			net:      src.net,
			swap:     src.swap,
			procs:    src.procCount,
		}, src.nav)
		return tabContent{object: v.object(), refresh: v.refresh}
	},
	tabCPU: func(src buildSources) tabContent {
		s := src.charts[tabCPU]
		if s == nil {
			return tabContent{object: newPlaceholder(labelCPUPageTitle)}
		}
		v := newCPUView(s, src.cpuCores, src.allProcs, src.cpuInfo, src.nav)
		return tabContent{object: v.object(), refresh: v.refresh}
	},
	tabMemory: func(src buildSources) tabContent {
		if !src.mem.wired() {
			return tabContent{object: newPlaceholder(labelMemoryPageTitle)}
		}
		v := newMemoryView(src.mem, src.allProcs, src.nav)
		return tabContent{object: v.object(), refresh: v.refresh}
	},
	tabDisk: func(src buildSources) tabContent {
		if src.disk == nil {
			return tabContent{object: newPlaceholder(labelDiskPageTitle)}
		}
		v := newDiskView(src.disk, src.diskDirs, src.diskIO, src.selectVolume)
		return tabContent{object: v.object(), refresh: v.refresh}
	},
	tabNetwork: func(src buildSources) tabContent {
		if !src.net.wired() {
			return tabContent{object: newPlaceholder(labelNetworkPageTitle)}
		}
		v := newNetworkView(src.net)
		return tabContent{object: v.object(), refresh: v.refresh}
	},
	tabProcesses: func(src buildSources) tabContent {
		if src.allProcs == nil {
			return tabContent{object: newPlaceholder(labelProcessesPageTitle)}
		}
		v := newProcessesView(src.allProcs, src.killProc)
		return tabContent{object: v.object(), refresh: v.refresh, selectPID: v.selectPID}
	},
	tabPorts: func(src buildSources) tabContent {
		if src.ports == nil {
			return tabContent{object: newPlaceholder(labelPortsPageTitle)}
		}
		v := newPortsView(src.ports, src.nav)
		return tabContent{object: v.object(), refresh: v.refresh}
	},
	tabConnections: func(src buildSources) tabContent {
		if src.conns == nil {
			return tabContent{object: newPlaceholder(labelConnectionsPageTitle)}
		}
		v := newConnsView(src.conns, src.nav)
		return tabContent{object: v.object(), refresh: v.refresh}
	},
}

// tabRefresher drives per-tab redraws: the poll tick redraws only the active
// tab's pane, and switching tabs redraws the newly-shown one so it isn't stale
// for up to one poll interval. refreshers is aligned with tab positions (a nil
// entry marks a static tab with nothing to redraw). Tracking the active index
// in the UI layer keeps the per-tick Snapshot()/arrange() work from scaling
// with the number of live tabs — only the visible chart does work.
type tabRefresher struct {
	refreshers []func() // by tab position; nil for static tabs
	active     int      // index of the tab currently on screen
}

// refreshAt redraws tab i's pane when it is live, ignoring out-of-range or
// static tabs so callers needn't guard.
func (t *tabRefresher) refreshAt(i int) {
	if i < 0 || i >= len(t.refreshers) {
		return
	}
	if r := t.refreshers[i]; r != nil {
		r()
	}
}

// setActive marks tab i as on screen and redraws it immediately, so a freshly
// switched-to tab shows current data without waiting for the next tick.
func (t *tabRefresher) setActive(i int) {
	t.active = i
	t.refreshAt(i)
}

// refresh redraws only the active tab — the per-tick callback the poller drives.
func (t *tabRefresher) refresh() { t.refreshAt(t.active) }

// newTabs returns the eight tab definitions with their content built fresh, a
// tabRefresher that redraws the active pane (see buildContent), and the
// per-tab cross-nav selectors. Identity (id/name/icon) is declared first;
// content is built via tabRegistry so new metric areas are additive — only a
// registry entry is required, not an edit here. Returning fresh defs keeps
// repeated buildContent calls from double-appending to a shared slice.
func newTabs(src buildSources) ([]tabDef, *tabRefresher, map[tabID]func(PID)) {
	tabs := []tabDef{
		{id: tabOverview, name: "Overview", icon: icon.Overview},
		{id: tabCPU, name: labelCPUPageTitle, icon: icon.CPU},
		{id: tabMemory, name: labelMemoryPageTitle, icon: icon.Memory},
		{id: tabDisk, name: labelDiskPageTitle, icon: icon.Disk},
		{id: tabNetwork, name: labelNetworkPageTitle, icon: icon.Network},
		{id: tabProcesses, name: labelProcessesPageTitle, icon: icon.Processes},
		{id: tabPorts, name: labelPortsPageTitle, icon: icon.Ports},
		{id: tabConnections, name: labelConnectionsPageTitle, icon: icon.Connections},
	}
	refreshers := make([]func(), len(tabs)) // aligned with tab positions
	selectors := make(map[tabID]func(PID))  // cross-nav entry points, by tab
	for i := range tabs {
		t := &tabs[i]
		var content tabContent
		if builder, ok := tabRegistry[t.id]; ok {
			content = builder(src)
		} else {
			content = tabContent{object: newPlaceholder(t.name)}
		}
		t.addChild(content.object)
		refreshers[i] = content.refresh // nil for static tabs
		if content.selectPID != nil {
			selectors[t.id] = content.selectPID
		}
	}
	return tabs, &tabRefresher{refreshers: refreshers}, selectors
}

// indexOfTab returns the position of the tab with the given id, and whether it
// was found, so cross-nav can resolve a target tab to its content pane.
func indexOfTab(tabs []tabDef, id tabID) (int, bool) {
	for i, t := range tabs {
		if t.id == id {
			return i, true
		}
	}
	return 0, false
}

// buildContent assembles the full window content from the available live sources
// and wires nav selection to content switching. It returns the content plus a
// refresh closure that redraws every live pane; the caller drives it on the UI
// goroutine each poll tick (see startUIRefresh).
func buildContent(src buildSources) (fyne.CanvasObject, func()) {
	// Create the cross-nav target before building tabs so the CPU/Memory
	// builders capture it; its action is wired below, once the panes and nav
	// selection exist.
	nav := &crossNav{}
	src.nav = nav

	tabs, tabsRefresh, selectors := newTabs(src)
	n := len(tabs)
	panes := make([]fyne.CanvasObject, n)
	items := make([]*navItem, n)
	holder := container.NewStack()

	selectIndex := func(i int) {
		for j, it := range items {
			it.setActive(j == i)
		}
		holder.Objects = []fyne.CanvasObject{panes[i]}
		holder.Refresh()
	}

	list := container.New(layout.NewCustomPaddedVBoxLayout(navItemGap))
	for i, d := range tabs {
		// Stack the tab's content panes. For a single pane this renders
		// identically to placing it directly; multi-pane tab layouts are a
		// follow-up (see refactor plan §13).
		panes[i] = container.NewStack(d.content...)
		items[i] = newNavItem(d.name, d.icon, i+1, func() { selectIndex(i) })
		list.Add(items[i])
	}
	wireProcessNav(nav, tabs, selectIndex, selectors)
	nav.selectTab = func(id tabID) {
		if i, ok := indexOfTab(tabs, id); ok {
			selectIndex(i)
		}
	}
	selectIndex(0)

	body := newTightBorder(nil, nil, newSidebar(list), nil, holder)
	title := vStackTight(newTitleBar(), hLine())
	statusRegion := vStackTight(hLine(), newStatusBar())
	return newTightBorder(title, statusRegion, nil, nil, body), refresh
}

// wireProcessNav points the cross-nav at the Processes tab: navigating selects
// that tab, then (when a process is targeted) highlights it through the tab's
// registered selectPID. Left unwired when no Processes tab exists, so its
// callers no-op rather than jumping nowhere.
func wireProcessNav(nav *crossNav, tabs []tabDef, selectIndex func(int), selectors map[tabID]func(PID)) {
	idx, ok := indexOfTab(tabs, tabProcesses)
	if !ok {
		return
	}
	nav.selectProcess = func(pid PID, highlight bool) {
		selectIndex(idx)
		if highlight {
			if selectPID := selectors[tabProcesses]; selectPID != nil {
				selectPID(pid)
			}
		}
	}
}

// newSidebar wraps the top-aligned nav list in a surface-colored, fixed-width
// rail with a 1px right divider.
func newSidebar(list fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(palette.Surface)
	bg.SetMinSize(fyne.NewSize(navWidth, 0))
	// Top-align the list: place it in the top slot so it keeps its min height
	// and the remaining space below stays empty (showing the surface bg).
	railBody := container.NewStack(bg, newTightBorder(list, nil, nil, nil, nil))
	return newTightBorder(nil, nil, nil, vLine(), railBody)
}

// newPlaceholder is the temporary content for a tab. It names the active tab so
// switching nav items visibly changes the content, confirming the nav works.
func newPlaceholder(name string) fyne.CanvasObject {
	return container.NewCenter(container.NewVBox(
		newHeading(name),
		newMeta("You are on the "+name+" tab"),
	))
}

// newTitleBar is the 38px top bar: an accent diamond logo mark followed by the
// "SYSTEM MONITOR" wordmark. Per the wireframe (design-system-05 title bar) the
// wordmark is Mono UPPERCASE in the muted text-2 color — not a bright Sans
// heading — so it reads as quiet chrome rather than a page title. The window
// controls on the right are left to the native OS title bar.
func newTitleBar() fyne.CanvasObject {
	logoImg := canvas.NewImageFromResource(
		colorizeStroke(icon.Diamond, palette.Accent),
	)
	logoImg.FillMode = canvas.ImageFillContain
	logo := container.NewGridWrap(fyne.NewSize(titleLogoSize, titleLogoSize), logoImg)

	wordmark := newColumnLabel(appName) // Mono 11 UPPERCASE, text-2

	brand := container.New(layout.NewCustomPaddedHBoxLayout(titleLogoGap), logo, wordmark)
	return newBar(titleBarHeight, brand)
}

// newStatusBar is the 26px bottom bar (muted Mono meta text).
func newStatusBar() fyne.CanvasObject {
	return newBar(statusBarHeight, newMeta("scaffold build — no live data yet"))
}

// newBar builds a fixed-height surface-2 bar with its content inset from the
// left and vertically centered.
func newBar(height float32, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(palette.Surface2)
	bg.SetMinSize(fyne.NewSize(0, height))

	row := container.NewHBox(content) // left-packed
	inset := container.New(layout.NewCustomPaddedLayout(0, 0, barHPad, 0), row)
	centered := container.New(layout.NewCustomPaddedVBoxLayout(0),
		layout.NewSpacer(), inset, layout.NewSpacer(),
	)
	return container.NewStack(bg, centered)
}

// hLine / vLine are 1px dividers in the border color.
func hLine() fyne.CanvasObject {
	r := canvas.NewRectangle(palette.Border)
	r.SetMinSize(fyne.NewSize(0, 1))
	return r
}

func vLine() fyne.CanvasObject {
	r := canvas.NewRectangle(palette.Border)
	r.SetMinSize(fyne.NewSize(1, 0))
	return r
}

// vStackTight stacks objects vertically with no inter-element padding.
func vStackTight(objects ...fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewCustomPaddedVBoxLayout(0), objects...)
}
