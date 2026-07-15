# System Monitor

A native desktop system monitoring app built in **Go** with the **Fyne** UI
toolkit and **gopsutil** for system data. Targets developers and power users.

Metric history is held entirely in memory (ring buffers, ~1 minute at 1s
resolution) — there is no persistence layer.

## Requirements

- **Go 1.26+**
- **A C compiler** — Fyne uses CGO/OpenGL. `CGO_ENABLED=1` is required (the
  `Makefile` and installer scripts set it automatically).

  - **Windows** — WinLibs mingw-w64:
    ```powershell
    winget install --id BrechtSanders.WinLibs.POSIX.UCRT -e --scope user
    ```
    Restart your shell afterward so `gcc` is on `PATH`.
  - **Linux (Debian/Ubuntu)** — `sudo apt-get install gcc libgl1-mesa-dev xorg-dev`
    (Fedora: `gcc mesa-libGL-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libxkbcommon-devel`;
    Arch: `gcc mesa libxcursor libxrandr libxinerama libxi`).
  - **macOS** — `xcode-select --install` (provides clang).

## Quick start (installer)

The installer detects your OS, installs the build deps above, generates the
bundled assets, and builds `bin/system-monitor`:

```sh
./scripts/install.sh                      # Linux / macOS
```
```powershell
powershell -ExecutionPolicy Bypass -File scripts\install-windows.ps1   # Windows
```

## Running

Bundled fonts/icons are compiled into `internal/ui/assets_gen.go` (which is
gitignored), so on a fresh clone generate them once before a bare `go` command:

```sh
go run ./tools/genassets         # generate assets (first time / after asset changes)
go run ./cmd/system-monitor      # the npm-start equivalent
```

The `make` targets do this step for you:

```sh
make run        # generate (if needed) + build and launch
```

Other tasks: `make build`, `make vet`, `make tidy`, `make fmt`, `make generate`.
See the `Makefile` for the full list. (On Windows the WinLibs toolchain ships
`mingw32-make`.)

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
  ui/                 # Fyne app shell and (eventually) the 8 tabs
  monitor/            # gopsutil-backed metric collectors
  metrics/            # in-memory ring buffers for metric history
```

The app's eight tabs — Overview, CPU, Memory, Disk, Network, Processes, Ports,
Connections — are described in the design artifacts under `.claude/`.
