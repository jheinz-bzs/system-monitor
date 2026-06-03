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
)

const (
	titleBarHeight  = 38
	statusBarHeight = 26
	barHPad         = 16 // leading inset for bar text (--sm-4)
	titleLogoSize   = 14 // accent diamond logo mark in the title bar
	titleLogoGap    = 8  // gap between logo and wordmark (--sm-2)
)

// tabDef describes one nav entry; the content builder uses the name so each
// placeholder identifies its own tab. Icons are Fyne built-ins (placeholders).
type tabDef struct {
	name string
	icon fyne.Resource
}

var tabDefs = []tabDef{
	{"Overview", icon.Overview},
	{"CPU", icon.CPU},
	{"Memory", icon.Memory},
	{"Disk", icon.Disk},
	{"Network", icon.Network},
	{"Processes", icon.Processes},
	{"Ports", icon.Ports},
	{"Connections", icon.Connections},
}

// buildContent assembles the full window content and wires nav selection to
// content switching.
func buildContent() fyne.CanvasObject {
	n := len(tabDefs)
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
	for i, d := range tabDefs {
		i := i // capture per iteration
		if d.name == "Overview" {
			panes[i] = newOverview()
		} else {
			panes[i] = newPlaceholder(d.name)
		}
		items[i] = newNavItem(d.name, d.icon, i+1, func() { selectIndex(i) })
		list.Add(items[i])
	}
	selectIndex(0)

	body := newTightBorder(nil, nil, newSidebar(list), nil, holder)
	title := vStackTight(newTitleBar(), hLine())
	status := vStackTight(hLine(), newStatusBar())
	return newTightBorder(title, status, nil, nil, body)
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

	wordmark := newColumnLabel("System Monitor") // Mono 11 UPPERCASE, text-2

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
