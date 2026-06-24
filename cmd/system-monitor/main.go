// Command system-monitor is the entry point for the System Monitor desktop app.
//
// It is a native desktop system monitoring tool built with the Fyne UI toolkit
// and gopsutil for system data. See the design artifacts in /.claude for the
// product spec, design system, and wireframes.
package main

import "github.com/josephheinz/system-monitor/internal/ui"

// version is the build-time application version, stamped at release time via
//
//	go build -ldflags "-X main.version=v1.2.3"
//
// (see the release workflow / Makefile release target). It stays "dev" for a
// plain `go run`/`make run`, which ui.Run treats as "no released version to
// compare against" and so disables the GitHub self-update check (BZS253-71).
var version = "dev"

func main() {
	ui.Run(version)
}
