# Installing on macOS

Release builds are **Apple Silicon (arm64) only** — M1 or newer. Intel Macs
must build from source.

## Option 1 — .dmg from GitHub releases (recommended)

1. Open the [latest release](https://github.com/josephheinz/system-monitor/releases/latest)
   and download **`system-monitor-darwin-arm64.dmg`**.
2. Open the `.dmg` and drag **System Monitor.app** into the **Applications**
   folder shortcut in the window.
3. Eject the disk image.

### ⚠️ First launch — the app is not signed

The app is **not notarized with an Apple Developer account**, so Gatekeeper
will block the first launch ("Apple could not verify…"). You have two ways
past it:

**A. Remove the quarantine flag in Terminal (most reliable):**

```sh
xattr -dr com.apple.quarantine "/Applications/System Monitor.app"
```

Then open the app normally. (`-d` deletes the `com.apple.quarantine`
attribute macOS stamps on downloads; `-r` applies it recursively through the
`.app` bundle.)

**B. Approve it in System Settings:**

1. Try to open the app once and dismiss the warning.
2. Go to **System Settings → Privacy & Security**, scroll down, and click
   **Open Anyway** next to the blocked-app message.
3. Confirm on the next prompt.

> On macOS 15 (Sequoia) and newer, the old right-click → Open trick no longer
> bypasses Gatekeeper — use one of the two methods above.

A bare binary (`system-monitor-darwin-arm64`) is also attached to each
release for terminal use; after `chmod +x` it needs the same
`xattr -d com.apple.quarantine` treatment.

## Option 2 — Build from source

Building locally avoids Gatekeeper entirely (no quarantine flag) and is the
only option for Intel Macs.

Requirements: **Go 1.26+** and the Xcode Command Line Tools (Fyne needs CGO):

```sh
xcode-select --install   # skip if already installed
brew install go          # or download from https://go.dev/dl/

git clone https://github.com/josephheinz/system-monitor.git
cd system-monitor
make run       # or `make build` → ./bin/system-monitor
```

The `make` targets run the required asset-generation step automatically; a
bare `go build` on a fresh clone fails until you run `go run ./tools/genassets`.
