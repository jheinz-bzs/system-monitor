// Command system-monitor is the entry point for the System Monitor desktop app.
//
// It is a native desktop system monitoring tool built with the Fyne UI toolkit
// and gopsutil for system data. See the design artifacts in /.claude for the
// product spec, design system, and wireframes.
package main

import "github.com/josephheinz/system-monitor/internal/ui"

func main() {
	ui.Run()
}
