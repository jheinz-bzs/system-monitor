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
    System Monitor is ad-hoc signed but NOT notarized, so the first launch
    may be blocked ("Apple could not verify…"). Either approve it in
    System Settings → Privacy & Security → Open Anyway, or clear the
    quarantine attribute:

      xattr -dr com.apple.quarantine "/Applications/System Monitor.app"

    To skip the quarantine step entirely at install time, set the env var
    (the --no-quarantine flag was removed from brew in Homebrew 4.0):

      HOMEBREW_CASK_OPTS="--no-quarantine" brew install --cask system-monitor

    On macOS 15 (Sequoia) and newer the old right-click → Open bypass no
    longer works — use one of the two methods above. See docs/install/INSTALL-MACOS.md.
  EOS
end
