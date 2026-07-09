package ui

// Status bar (footer) content: the persistent bottom chrome summarizing live
// machine health and a few headline metrics, matching the design-system-05
// wireframe footer (dot + label health indicator on the left, headline readouts,
// process count and a poll-rate indicator on the right). It reuses the same
// ring-buffer sources the metric tabs read — no new data source — and redraws on
// the same poll tick as the active tab.

import (
	"image/color"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/josephheinz/system-monitor/internal/series"
	"github.com/josephheinz/system-monitor/internal/update"
)

// Status bar readout labels. The metric abbreviations are shorter than the panel
// titles (labelCPUPanel etc.), so they get their own consts.
const (
	labelStatusCPU  = "CPU "
	labelStatusMem  = "MEM "
	labelStatusNet  = "NET "
	labelStatusPoll = "s poll" // suffixed to the poll interval in seconds: "1s poll"

	// Self-update affordance (BZS253-71): a tappable link shown only when a newer
	// release is available, plus a status word for the install phases.
	labelUpdateLink   = "↑ Update"
	labelUpdating     = "updating…"
	labelUpdateFailed = "update failed"
)

// Status bar layout geometry.
const (
	statusItemGap = spaceXL // 16; gap between status-bar readout groups
	statusDotGap  = spaceSM // 4;  gap between a status dot and its label
)

// coloredDot is a filled status indicator circle of the given color, sized for a
// panel header or the status bar. It returns the circle (so a live caller can
// recolor it on refresh) alongside the laid-out object to place.
func coloredDot(col color.Color) (*canvas.Circle, fyne.CanvasObject) {
	dot := canvas.NewCircle(col)
	return dot, container.NewGridWrap(fyne.NewSize(overviewDotSize, overviewDotSize), dot)
}

// statusBarView is the live footer: the readout widgets plus the sources feeding
// them. refresh re-reads the sources each poll tick.
type statusBarView struct {
	cpu   series.Source // overall CPU %; nil when not wired
	mem   memSources    // memory used/total; zero when not wired
	net   netSources    // upload/download rates; zero when not wired
	swap  swapSources   // swap used/total; feeds health only; zero when not wired
	procs series.Source // process-count history; nil when not wired

	healthDot    *canvas.Circle
	healthDotObj fyne.CanvasObject
	healthLabel  *canvas.Text
	cpuText      *canvas.Text
	memText      *canvas.Text
	netText      *canvas.Text
	procsText    *canvas.Text

	updateStatus func() update.Snapshot // nil on a dev build → no update affordance
	updateLink   *jumpLink              // tappable when an update is available
	updateText   *canvas.Text           // new version / install-phase word
	updateGroup  *fyne.Container        // link + text; hidden unless an update is pending

	record *recordControl // session tracking toggle (BZS253-77); nil when not wired
}

// newStatusBarView builds the footer from the same sources the metric tabs read.
// An unwired source leaves its readout blank rather than crashing.
func newStatusBarView(src buildSources) *statusBarView {
	v := &statusBarView{
		cpu:          src.charts[tabCPU],
		mem:          src.mem,
		net:          src.net,
		swap:         src.swap,
		procs:        src.procCount,
		updateStatus: src.updateStatus,
		healthLabel:  newMeta(statusLabel(status.Healthy)),
		cpuText:      newMeta(""),
		memText:      newMeta(""),
		netText:      newMeta(""),
		procsText:    newMeta(""),
	}
	v.healthDot, v.healthDotObj = coloredDot(statusColor(status.Healthy))

	// The update affordance is built whether or not it's wired; it simply stays
	// hidden when there's no updater (dev build) or no update pending, taking no
	// space in the footer's box layout until refresh reveals it.
	v.updateLink = newJumpLink(labelUpdateLink, src.startUpdate)
	v.updateText = newMeta("")
	v.updateGroup = container.New(layout.NewCustomPaddedHBoxLayout(statusDotGap),
		vCenter(v.updateLink), vCenter(v.updateText))
	v.updateGroup.Hide()

	// Session tracking toggle (BZS253-77): built only when the composition root
	// wired both the state getter and the start/stop action.
	if src.recording != nil && src.toggleRecord != nil {
		v.record = newRecordControl(src.recording, src.toggleRecord)
	}

	v.refresh() // paint initial values before the first poll tick
	return v
}

// object assembles the footer row: the health indicator and headline readouts
// packed left, the process count and poll indicator pushed to the right edge by
// a spacer.
func (v *statusBarView) object() fyne.CanvasObject {
	health := container.New(layout.NewCustomPaddedHBoxLayout(statusDotGap),
		vCenter(v.healthDotObj), vCenter(v.healthLabel))

	_, pollDotObj := coloredDot(palette.Accent)
	poll := container.New(layout.NewCustomPaddedHBoxLayout(statusDotGap),
		vCenter(pollDotObj), vCenter(newMeta(pollLabel())))

	// Left group: health + live readouts. Right group (pushed by the spacer):
	// the tracking toggle, process count, and poll indicator.
	items := []fyne.CanvasObject{
		health,
		vCenter(v.cpuText),
		vCenter(v.memText),
		vCenter(v.netText),
		v.updateGroup,
		layout.NewSpacer(),
	}
	if v.record != nil {
		items = append(items, vCenter(v.record))
	}
	items = append(items, vCenter(v.procsText), poll)

	row := container.New(layout.NewCustomPaddedHBoxLayout(statusItemGap), items...)
	return newBar(statusBarHeight, row)
}

// refresh re-reads each wired source and repaints its readout, recoloring the
// health dot and relabeling its word to the current aggregate state. It touches
// the canvas, so a background poller must marshal it via fyne.Do.
func (v *statusBarView) refresh() {
	kind := v.health()
	v.healthDot.FillColor = statusColor(kind)
	v.healthDot.Refresh()
	v.healthLabel.Text = statusLabel(kind)
	v.healthLabel.Refresh()

	if v.cpu != nil {
		v.cpuText.Text = labelStatusCPU + formatWhole(latestSample(v.cpu.Values())) + labelUnitPercent
		v.cpuText.Refresh()
	}
	if v.mem.wired() {
		used := uint64(latestSample(v.mem.used.Values()))
		v.memText.Text = labelStatusMem + formatBytesShort(used) + labelTotalSlash + formatBytesShort(v.mem.total)
		v.memText.Refresh()
	}
	if v.net.wired() {
		down := formatRateText(latestSample(v.net.download.Values()))
		up := formatRateText(latestSample(v.net.upload.Values()))
		v.netText.Text = labelStatusNet + labelDownPrefix + down + labelRateGap + labelUpPrefix + up
		v.netText.Refresh()
	}
	if v.procs != nil {
		v.procsText.Text = formatWhole(latestSample(v.procs.Values())) + labelRateGap + labelUnitProcs
		v.procsText.Refresh()
	}
	v.refreshUpdate()
	if v.record != nil {
		v.record.refresh()
	}
}

// refreshUpdate reflects the self-update state into the footer affordance: a
// tappable link offering the new version when one is available, a quiet status
// word during install, and nothing at all when idle or unwired. The check runs
// once per launch, so a failed state simply persists rather than nagging.
func (v *statusBarView) refreshUpdate() {
	if v.updateStatus == nil {
		return // dev build: no updater wired
	}
	snap := v.updateStatus()
	switch snap.State {
	case update.StatusAvailable:
		v.updateLink.Show()
		v.updateText.Text = snap.NewVersion
		v.updateText.Refresh()
		v.updateGroup.Show()
	case update.StatusDownloading, update.StatusInstalling:
		v.updateLink.Hide() // not tappable mid-install
		v.updateText.Text = labelUpdating
		v.updateText.Refresh()
		v.updateGroup.Show()
	case update.StatusFailed:
		v.updateLink.Hide()
		v.updateText.Text = labelUpdateFailed
		v.updateText.Refresh()
		v.updateGroup.Show()
	default: // StatusIdle, StatusChecking
		v.updateGroup.Hide()
	}
}

// health is the aggregate state across the classified metrics (CPU, memory, and
// swap usage), using the same model as the Overview badge so the two agree.
// Unwired metrics are skipped, so a machine with none still reads healthy.
func (v *statusBarView) health() statusKind {
	var states []statusKind
	if v.cpu != nil {
		states = append(states, cpuHealth.classify(latestSample(v.cpu.Values())))
	}
	if v.mem.wired() {
		states = append(states, memHealth.classify(usagePercent(latestSample(v.mem.used.Values()), v.mem.total)))
	}
	if v.swap.wired() {
		states = append(states, swapHealth.classify(usagePercent(latestSample(v.swap.used.Values()), v.swap.total)))
	}
	return aggregateHealth(states)
}

// pollLabel renders the poll-rate indicator's text from the actual poll interval
// ("1s poll"), so it stays truthful if pollInterval changes.
func pollLabel() string {
	return strconv.Itoa(int(pollInterval/time.Second)) + labelStatusPoll
}
