package ui

// Bundled nav icons — Lucide (line/stroke weight), ISC licensed (see
// icons/LICENSE). Line glyphs draw with stroke="currentColor" (fill="none"), so
// they're recolored per state with colorizeStroke (see colorize.go), not Fyne's
// fill-only theme.NewColoredResource. The faces are embedded so the binary
// stays self-contained.

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icons/overview.svg
var iconOverviewSVG []byte

//go:embed icons/cpu.svg
var iconCPUSVG []byte

//go:embed icons/memory.svg
var iconMemorySVG []byte

//go:embed icons/disk.svg
var iconDiskSVG []byte

//go:embed icons/network.svg
var iconNetworkSVG []byte

//go:embed icons/processes.svg
var iconProcessesSVG []byte

//go:embed icons/ports.svg
var iconPortsSVG []byte

//go:embed icons/connections.svg
var iconConnectionsSVG []byte

var (
	iconOverview    = fyne.NewStaticResource("overview.svg", iconOverviewSVG)
	iconCPU         = fyne.NewStaticResource("cpu.svg", iconCPUSVG)
	iconMemory      = fyne.NewStaticResource("memory.svg", iconMemorySVG)
	iconDisk        = fyne.NewStaticResource("disk.svg", iconDiskSVG)
	iconNetwork     = fyne.NewStaticResource("network.svg", iconNetworkSVG)
	iconProcesses   = fyne.NewStaticResource("processes.svg", iconProcessesSVG)
	iconPorts       = fyne.NewStaticResource("ports.svg", iconPortsSVG)
	iconConnections = fyne.NewStaticResource("connections.svg", iconConnectionsSVG)
)
