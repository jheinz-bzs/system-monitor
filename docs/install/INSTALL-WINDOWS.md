# Installing on Windows

## Option 1 — GitHub release (recommended)

1. Open the [latest release](https://github.com/josephheinz/system-monitor/releases/latest).
2. Download **`system-monitor-windows-amd64.exe`**.
3. (Optional) Verify the download against `checksums.txt`:

   ```powershell
   Get-FileHash .\system-monitor-windows-amd64.exe -Algorithm SHA256
   ```

4. Move the `.exe` wherever you like and double-click to run. No installer —
   it's a single self-contained binary.

> **SmartScreen**: the binary is unsigned, so Windows may show
> "Windows protected your PC". Click **More info → Run anyway**.

## Option 2 — Build from source

Requirements:

- **Go 1.26+** — https://go.dev/dl/
- **A C compiler** — Fyne needs CGO. Install [WinLibs mingw-w64](https://winlibs.com/)
  and put its `bin` directory on `PATH` (verify with `gcc --version`).
- **GNU Make** (optional) — ships with mingw-w64 as `mingw32-make`.

```powershell
git clone https://github.com/josephheinz/system-monitor.git
cd system-monitor
mingw32-make build-win  # generates bundled assets, then builds bin\system-monitor.exe
```

No make? Run the two steps directly:

```powershell
go run ./tools/genassets
$env:CGO_ENABLED = "1"; go build -o system-monitor.exe ./cmd/system-monitor
```

The asset-generation step is required on a fresh clone — a bare `go build`
fails without it.
