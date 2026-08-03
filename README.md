# System Monitor

[![Release](https://img.shields.io/github/v/release/josephheinz/system-monitor)](https://github.com/josephheinz/system-monitor/releases/latest)
[![License: MIT](https://img.shields.io/github/license/josephheinz/system-monitor)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/josephheinz/system-monitor)](go.mod)
[![Release build](https://img.shields.io/github/actions/workflow/status/josephheinz/system-monitor/release.yml?label=release%20build)](https://github.com/josephheinz/system-monitor/actions/workflows/release.yml)
[![Repo size](https://img.shields.io/github/repo-size/josephheinz/system-monitor)](https://github.com/josephheinz/system-monitor)
[![Downloads](https://img.shields.io/github/downloads/josephheinz/system-monitor/total)](https://github.com/josephheinz/system-monitor/releases)
![Platforms](https://img.shields.io/badge/platforms-windows%20%7C%20linux%20%7C%20macos-blue)

A native desktop system monitoring app built in **Go** with the **Fyne** UI
toolkit and **gopsutil** for system data. Targets developers and power users.

Metric history is held entirely in memory (ring buffers, ~1 minute at 1s
resolution) — there is no persistence layer.

## Tabs

- **Overview** — 2×4 metric grid with sparklines
- **CPU** — overall + per-core charts, top processes
- **Memory** — stacked usage chart, top processes
- **Disk** — storage treemap, volumes, I/O chart
- **Network** — bandwidth stats and chart
- **Processes** — treemap + sortable table
- **Ports** — filterable table with cross-tab navigation to the owning process
- **Connections** — filterable table with state pills and cross-tab navigation
- **Settings** — app preferences

## Installation

Per-OS guides — each covers installing from the
[GitHub releases](https://github.com/josephheinz/system-monitor/releases/latest)
or building from source:

- [Windows](docs/install/INSTALL-WINDOWS.md)
- [Linux](docs/install/INSTALL-LINUX.md) — APT repository, bare binary, `.deb`, or AppImage
- [macOS](docs/install/INSTALL-MACOS.md) — note the unsigned-app first-launch steps

## Headless recording (servers)

`system-monitor-record` is a headless agent that runs the app's session-tracking
mode without a GUI: one CSV row per tick, same schema as the in-app record
button, so the file opens in the desktop app's **Recordings** tab. It needs no
Fyne, no cgo, and no display — grab it from the release assets
(`system-monitor-record-<os>-<arch>`) or build it yourself:

```sh
make build-record                                    # host platform
GOOS=linux GOARCH=amd64 make build-record            # cross-compile for a server
```

```sh
system-monitor-record                          # tracking-<timestamp>.csv in cwd, until Ctrl-C
system-monitor-record -out /var/lib/sysmon/run.csv -interval 5s -duration 1h
```

It runs in the foreground and stops on SIGINT/SIGTERM — let your service manager
supervise it, e.g. a systemd unit:

```ini
[Service]
ExecStart=/usr/local/bin/system-monitor-record -out /var/lib/sysmon/tracking.csv -interval 5s
Restart=always

[Install]
WantedBy=multi-user.target
```

## Layout

```
cmd/system-monitor/          # main package — entry point (GUI)
cmd/system-monitor-record/   # headless recording agent (servers)
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
