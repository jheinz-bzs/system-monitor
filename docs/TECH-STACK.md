# Tech Stack

## Languages & runtimes
- Go 1.26.3 — module `github.com/josephheinz/system-monitor`
- Requires CGO (`CGO_ENABLED=1`) — Fyne uses OpenGL. On Windows, install the
  WinLibs mingw-w64 toolchain so `gcc` is on `PATH`:
  `winget install --id BrechtSanders.WinLibs.POSIX.UCRT -e --scope user`

## Frameworks
- Fyne v2.7.4 — native desktop UI toolkit (renders its own widgets, not HTML/CSS)

## Data layer
- No database, no file persistence for metrics
- gopsutil v4 (`github.com/shirou/gopsutil/v4`) — system metric collection
- In-memory ring buffers hold ~1 minute of history at 1s resolution

## Test frameworks
- Standard `testing` package (testify v1.11.1 available as a transitive dep)
- Run: `go test ./...`

## Build & deploy
- Local dev: `make run` (or `go run ./cmd/system-monitor`)
- Build: `make build` → `bin/system-monitor`
- Static analysis: `make vet` · Format: `make fmt` · Sync modules: `make tidy`
- Deploy: distributed as a standalone native binary (no server/deploy pipeline)

## Notable libraries
- fyne.io/fyne/v2 — UI toolkit (app shell, widgets, canvas, charts)
- github.com/shirou/gopsutil/v4 — CPU, memory, disk, network, process, port data
- fyne.io/systray — system tray integration (transitive)
