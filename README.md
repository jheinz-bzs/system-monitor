# System Monitor

A native desktop system monitoring app built in **Go** with the **Fyne** UI
toolkit and **gopsutil** for system data. Targets developers and power users.

Metric history is held entirely in memory (ring buffers, ~1 minute at 1s
resolution) — there is no persistence layer.

## Tabs

Nine tabs: **Overview** (2×4 metric grid with sparklines), **CPU** (overall +
per-core charts, top processes), **Memory** (stacked usage chart, top
processes), **Disk** (storage treemap, volumes, I/O chart), **Network**
(bandwidth stats and chart), **Processes** (treemap + sortable table),
**Ports** and **Connections** (filterable tables with cross-tab navigation to
the owning process), and **Settings**.

## Installation

Per-OS guides — each covers installing from the
[GitHub releases](https://github.com/jheinz-bzs/system-monitor/releases/latest)
or building from source:

- [Windows](docs/install/INSTALL-WINDOWS.md)
- [Linux](docs/install/INSTALL-LINUX.md) — bare binary, `.deb`, or AppImage
- [macOS](docs/install/INSTALL-MACOS.md) — note the unsigned-app first-launch steps

## Layout

```
cmd/system-monitor/   # main package — entry point
internal/
  ui/                 # Fyne app shell and the tabs
  monitor/            # gopsutil-backed metric collectors
  metrics/            # in-memory ring buffers for metric history
```

Design artifacts (per-tab wireframes, design system, product spec) live under
`docs/` and `.claude/`. Engineering standards are indexed in
[docs/conventions/README.md](docs/conventions/README.md).

## License

[MIT](LICENSE)
