# Windows: install the WinLibs mingw-w64 toolchain (gcc for CGO), then build.
$ErrorActionPreference = 'Stop'

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go is not installed. Get it from https://go.dev/dl/ then re-run."
    exit 1
}

if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
    Write-Host "Installing WinLibs mingw-w64 toolchain via winget..."
    winget install --id BrechtSanders.WinLibs.POSIX.UCRT -e --scope user
    Write-Warning "Restart your shell so 'gcc' is on PATH, then re-run this script."
    exit 1
}

$repo = Resolve-Path (Join-Path $PSScriptRoot '..')
Push-Location $repo
try {
    $env:CGO_ENABLED = '1'
    go run ./tools/genassets
    go build -o bin/system-monitor.exe ./cmd/system-monitor
    Write-Host "Built .\bin\system-monitor.exe"
} finally {
    Pop-Location
}
