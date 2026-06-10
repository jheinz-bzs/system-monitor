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
	charts  liveSources   // time-series chart sources, keyed by tabID
	procs   processSource // process snapshot source; nil when not wired
	cpuInfo cpuMeta       // static processor description; zero when unknown
}

// tabContent is the built content for one tab: the object to display and an
// optional refresh callback (nil for static tabs that never update).
type tabContent struct {
	object  fyne.CanvasObject
	refresh func()
}

// tabBuilder constructs a tab's content from the available live sources.
type tabBuilder func(src buildSources) tabContent

// tabRegistry maps tab IDs to their builder functions. To add a new live tab,
// register its builder here — newTabs is never edited for new metric areas.
var tabRegistry = map[tabID]tabBuilder{
	tabOverview: func(_ buildSources) tabContent {
		return tabContent{object: newOverview()}
	},
	tabCPU: func(src buildSources) tabContent {
		s := src.charts[tabCPU]
		if s == nil {
			return tabContent{object: newPlaceholder("CPU")}
		}
		v := newCPUView(s, src.procs, src.cpuInfo)
		return tabContent{object: v.object(), refresh: v.refresh}
	},
}

// newTabs returns the eight tab definitions with their content built fresh, and
// a refresh closure that redraws every live pane (see buildContent). Identity
// (id/name/icon) is declared first; content is built via tabRegistry so new
// metric areas are additive — only a registry entry is required, not an edit
// here. Returning fresh defs keeps repeated buildContent calls from
// double-appending to a shared slice.
func newTabs(src buildSources) ([]tabDef, func()) {
	tabs := []tabDef{
		{id: tabOverview, name: "Overview", icon: icon.Overview},
		{id: tabCPU, name: "CPU", icon: icon.CPU},
		{id: tabMemory, name: "Memory", icon: icon.Memory},
		{id: tabDisk, name: "Disk", icon: icon.Disk},
		{id: tabNetwork, name: "Network", icon: icon.Network},
		{id: tabProcesses, name: "Processes", icon: icon.Processes},
		{id: tabPorts, name: "Ports", icon: icon.Ports},
		{id: tabConnections, name: "Connections", icon: icon.Connections},
	}
	var refreshers []func()
	for i := range tabs {
		t := &tabs[i]
		var content tabContent
		if builder, ok := tabRegistry[t.id]; ok {
			content = builder(src)
		} else {
			content = tabContent{object: newPlaceholder(t.name)}
		}
		t.addChild(content.object)
		if content.refresh != nil {
			refreshers = append(refreshers, content.refresh)
		}
	}
	refresh := func() {
		for _, r := range refreshers {
			r()
		}
	}
	return tabs, refresh
}

// buildContent assembles the full window content from the available live sources
// and wires nav selection to content switching. It returns the content plus a
// refresh closure that redraws every live pane; the caller drives it on the UI
// goroutine each poll tick (see startUIRefresh).
func buildContent(src buildSources) (fyne.CanvasObject, func()) {
	tabs, refresh := newTabs(src)
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
		i := i // capture per iteration
		// Stack the tab's content panes. For a single pane this renders
		// identically to placing it directly; multi-pane tab layouts are a
		// follow-up (see refactor plan §13).
		panes[i] = container.NewStack(d.content...)
		items[i] = newNavItem(d.name, d.icon, i+1, func() { selectIndex(i) })
		list.Add(items[i])
	}
	selectIndex(0)

	body := newTightBorder(nil, nil, newSidebar(list), nil, holder)
	title := vStackTight(newTitleBar(), hLine())
	statusRegion := vStackTight(hLine(), newStatusBar())
	return newTightBorder(title, statusRegion, nil, nil, body), refresh
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
