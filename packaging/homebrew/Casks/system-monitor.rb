# Homebrew cask for System Monitor (issue #67). This is the source-of-truth
# template in this repo; the release workflow
# (.github/workflows/release.yml → sync-homebrew-tap) copies it into the
# josephheinz/homebrew-tap repo and stamps the live values over the
# __VERSION__ / __SHA256__ placeholders before committing.
#
# A cask (not a formula): the app is a GUI bundle that lands in /Applications.
cask "system-monitor" do
  version "__VERSION__"
  sha256 "__SHA256__"

  url "https://github.com/josephheinz/system-monitor/releases/download/v#{version}/system-monitor-darwin-arm64.dmg"
  name "System Monitor"
  desc "Native desktop system monitor — CPU, memory, disk, network, processes."
  homepage "https://github.com/josephheinz/system-monitor"

  livecheck do
    url "https://github.com/josephheinz/system-monitor/releases/latest"
    strategy :github_latest
  end

  app "System Monitor.app"

  caveats <<~EOS
    System Monitor is ad-hoc signed but NOT notarized, so Gatekeeper blocks the
    first launch ("Apple could not verify…"). Remove the quarantine attribute:

      xattr -dr com.apple.quarantine "/Applications/System Monitor.app"

    or pass --no-quarantine when installing:

      brew install --cask --no-quarantine system-monitor

    On macOS 15 (Sequoia) and newer the old right-click → Open bypass no longer
    works — use one of the two methods above. See docs/install/INSTALL-MACOS.md.
  EOS
end
