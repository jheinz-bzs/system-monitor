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

  # The app is ad-hoc signed but not notarized (no paid Developer ID yet), so
  # Gatekeeper would block the first launch of a quarantined copy. Homebrew
  # quarantines every cask download unconditionally and the old
  # --no-quarantine flag/env var is a no-op for installs (Homebrew/brew#23362),
  # so strip the attribute here — same effect, no user step required.
  postflight do
    system_command "/usr/bin/xattr",
                   args: ["-dr", "com.apple.quarantine", "#{appdir}/System Monitor.app"]
  end

  caveats <<~EOS
    System Monitor is ad-hoc signed but NOT notarized. The cask clears the
    quarantine attribute on install, so the first launch should just work.

    If you installed from the .dmg instead (or ever see "Apple could not
    verify…"), clear quarantine manually:

      xattr -dr com.apple.quarantine "/Applications/System Monitor.app"

    or approve it in System Settings → Privacy & Security → Open Anyway.
    See docs/install/INSTALL-MACOS.md.
  EOS
end
