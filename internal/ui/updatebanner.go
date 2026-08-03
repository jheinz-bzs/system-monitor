package ui

// Update banner (BZS253-71): a dismissible accent-tinted bar shown above the tab
// content when a newer release is available, offering a one-click update. It
// complements the status-bar pill — the banner grabs attention on first
// detection; the footer pill is the quiet persistent affordance, and remains
// after the banner is dismissed. Like the status bar, the banner refreshes on
// the poll tick and reads the same update seam (updateStatus/startUpdate); it
// stays hidden — taking no space in the vertical stack — until an update is
// available and undismissed.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"

	"github.com/josephheinz/system-monitor/internal/update"
)

// Banner copy. The version tag is spliced between prefix and suffix so the
// single message reads "↑ New version v1.3.0 available". labelUpdateActionApt
// replaces labelUpdateAction on a package-manager-owned install, whose tap
// surfaces the apt command instead of self-updating (issue #68).
const (
	labelUpdateBannerPrefix = "↑ New version "
	labelUpdateBannerSuffix = " available"
	labelUpdateAction       = "Update"
	labelUpdateActionApt    = "Update via apt"
	labelUpdateDismiss      = "✕"
)

// Update-via-apt guidance (issue #68). A .deb install is apt-owned, so the app
// must not swap the binary itself; tapping the banner/pill instead surfaces the
// one-shot upgrade command — the apt repo at josephheinz.github.io/system-monitor
// is live and serves the same package.
const (
	labelAptGuideTitle   = "Update via apt"
	labelAptGuideBlurb   = "system-monitor is installed via apt. Run:"
	labelAptGuideCommand = "sudo apt update && sudo apt upgrade"
)

// updateActionLabel picks the banner's action-link text for the delivery mode:
// "Update" on a self-updating install, "Update via apt" on a dpkg-owned one
// whose upgrades come from apt rather than the in-app swap (issue #68).
func updateActionLabel(mode update.Mode) string {
	if mode == update.ModePackageManager {
		return labelUpdateActionApt
	}
	return labelUpdateAction
}

// showAptUpdateGuide surfaces the package-manager upgrade command for a
// dpkg-owned install. The tap handler runs on the UI goroutine, so like the
// kill confirm it may call dialog directly; the user runs the command in a
// terminal to pull the new version from the apt repo.
func showAptUpdateGuide(win fyne.Window) {
	dialog.ShowInformation(labelAptGuideTitle, labelAptGuideBlurb+"\n\n"+labelAptGuideCommand, win)
}

// updateBannerHeight is the banner's fixed height — a component dimension, so it
// carries its own literal-px const rather than a spacing-scale multiple. It
// matches titleBarHeight so the banner reads as part of the top chrome.
const updateBannerHeight = 40

// updateBannerView is the live update banner: the message text plus the update
// seam feeding it. refresh re-reads the seam each poll tick.
type updateBannerView struct {
	updateStatus func() update.Snapshot // nil on a dev build → banner never shows
	text         *canvas.Text           // the message; its version splice is rewritten on refresh
	root         *fyne.Container         // bar + divider; hidden unless an update is pending
	dismissed    bool                    // sticky for the session once the user closes it
}

// newUpdateBannerView builds the banner from the update seam. It starts hidden;
// refresh reveals it once a newer release is detected. The link triggers the
// same install path as the status-bar pill — a self-update, or the apt-guidance
// dialog on a package-manager-owned install (updateMode); the ✕ dismisses for
// the session.
func newUpdateBannerView(src buildSources) *updateBannerView {
	v := &updateBannerView{updateStatus: src.updateStatus}

	v.text = styledText("", font.MonoMedium, theme.SizeNameCaptionText, palette.Accent)
	action := newJumpLink(updateActionLabel(src.updateMode), src.startUpdate)
	dismiss := newJumpLink(labelUpdateDismiss, v.dismiss)

	row := container.New(layout.NewCustomPaddedHBoxLayout(statusItemGap),
		vCenter(v.text),
		layout.NewSpacer(),
		vCenter(action),
		vCenter(dismiss),
	)
	bar := newColoredBar(updateBannerHeight, palette.AccentDim, row)

	// Bundle the bar with its bottom divider so hiding the banner hides both and
	// leaves no orphan rule above the content.
	v.root = vStackTight(bar, hLine())
	v.root.Hide()
	return v
}

func (v *updateBannerView) object() fyne.CanvasObject { return v.root }

// dismiss hides the banner for the rest of the session; the footer pill carries
// the update affordance from then on.
func (v *updateBannerView) dismiss() {
	v.dismissed = true
	v.root.Hide()
}

// refresh shows the banner only while an update is available and undismissed.
// Once the install starts (or on a dev build with no seam), it steps aside and
// lets the status bar report progress.
func (v *updateBannerView) refresh() {
	if v.updateStatus == nil || v.dismissed {
		v.root.Hide()
		return
	}
	snap := v.updateStatus()
	if snap.State != update.StatusAvailable {
		v.root.Hide()
		return
	}
	v.text.Text = labelUpdateBannerPrefix + snap.NewVersion + labelUpdateBannerSuffix
	v.text.Refresh()
	v.root.Show()
}
