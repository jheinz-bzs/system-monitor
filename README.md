# System Monitor

A native desktop system monitoring app built in **Go** with the **Fyne** UI
toolkit and **gopsutil** for system data. Targets developers and power users.

Metric history is held entirely in memory (ring buffers, ~1 minute at 1s
resolution) — there is no persistence layer.

## Requirements

- **Go 1.26+**
- **A C compiler** — Fyne uses CGO/OpenGL. On Windows, install the WinLibs
  mingw-w64 toolchain:
  ```powershell
  winget install --id BrechtSanders.WinLibs.POSIX.UCRT -e --scope user
  ```
  Restart your shell afterward so `gcc` is on `PATH`. `CGO_ENABLED=1` is
  required (the `Makefile` sets it automatically).

## Running

```sh
go run ./cmd/system-monitor      # the npm-start equivalent
# or, with the bundled mingw32-make:
make run
```

Other tasks: `make build`, `make vet`, `make tidy`, `make fmt`. See the
`Makefile` for the full list.

## Layout

```
cmd/system-monitor/   # main package — entry point
internal/
  ui/                 # Fyne app shell and (eventually) the 8 tabs
  monitor/            # gopsutil-backed metric collectors
  metrics/            # in-memory ring buffers for metric history
```

The app's eight tabs — Overview, CPU, Memory, Disk, Network, Processes, Ports,
Connections — are described in the design artifacts under `.claude/`.
