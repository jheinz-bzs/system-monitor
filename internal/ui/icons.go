package ui

// Bundled nav icons — Lucide (line/stroke weight), ISC licensed (see
// icons/LICENSE). Line glyphs draw with stroke="currentColor" (fill="none"), so
// they're recolored per state with colorizeStroke (see colorize.go), not Fyne's
// fill-only theme.NewColoredResource. The bytes are compiled into the binary
// via assets_gen.go and reached through resource() (see assets.go), so the
// binary stays self-contained.

import "fyne.io/fyne/v2"

// iconSet groups the bundled icons so call sites read icon.Overview rather than
// a loose package global, making each icon's origin obvious cross-file.
type iconSet struct {
	Overview    fyne.Resource
	CPU         fyne.Resource
	Memory      fyne.Resource
	Disk        fyne.Resource
	Network     fyne.Resource
	Processes   fyne.Resource
	Ports       fyne.Resource
	Connections fyne.Resource
	Diamond     fyne.Resource // title-bar brand mark
}

var icon = iconSet{
	Overview:    resource("icons/overview.svg"),
	CPU:         resource("icons/cpu.svg"),
	Memory:      resource("icons/memory.svg"),
	Disk:        resource("icons/disk.svg"),
	Network:     resource("icons/network.svg"),
	Processes:   resource("icons/processes.svg"),
	Ports:       resource("icons/ports.svg"),
	Connections: resource("icons/connections.svg"),
	Diamond:     resource("icons/diamond.svg"),
}
