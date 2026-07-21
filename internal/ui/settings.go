package ui

// Settings tab (BZS253-72): a single panel of preference rows, styled from the
// design-system tokens like every other tab. Each control writes straight
// through to the persisted settings, then fires its applyHooks entry so the
// change takes effect immediately — no relaunch. Only the rows whose meaning
// is inherently launch-time (start tab, auto-update install) stay launch-
// scoped. The tab is static — no live data — so it ships no refresh callback.

import (
	"image/color"
	"slices"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const (
	labelSettingsPageTitle = "Settings"
	labelSettingsPanel     = "Preferences"
	labelSystemPanel       = "System"

	labelSettingStartTab   = "Start tab"
	labelSettingPoll       = "Poll interval"
	labelSettingMemCap     = "Memory cap"
	labelSettingTheme      = "Appearance"
	labelSettingAutoUpdate = "Auto-update"
	labelSettingTray       = "Minimize to tray"
	labelSettingCPUAlert   = "CPU alert"
	labelSettingMemAlert   = "Memory alert"
	labelSettingDiskAlert  = "Disk alert"

	helpSettingStartTab   = "Tab shown on launch"
	helpSettingPoll       = "Sampling cadence"
	helpSettingMemCap     = "GC soft heap limit"
	helpSettingTheme      = "Color palette"
	helpSettingAutoUpdate = "Install updates on next launch · off by default"
	helpSettingTray       = "Close hides to the tray · off by default"
	// Shared by the CPU and memory alert rows (disk names its volume choice).
	helpSettingAlert     = "Notify above the threshold · off by default"
	helpSettingDiskAlert = "Busiest volume above the threshold · off by default"

	// System-section row labels (BZS253-74): static, read-once machine facts.
	labelSysHostname = "Hostname"
	labelSysOS       = "OS"
	labelSysKernel   = "Kernel"
	labelSysBootTime = "Boot time"
	labelSysUptime   = "Uptime"
	labelSysCores    = "Logical cores"
	labelSysUsers    = "Logged-in users"
	labelSysVersion  = "Version"

	// labelToggleEnabled is the on/off chip caption shared by the boolean
	// settings (memory cap, auto-update).
	labelToggleEnabled = "Enabled"

	// labelUnknownValue stands in for a machine fact the host lookup couldn't
	// fill — the shared em-dash, matching the tables' unresolved-value convention.
	labelUnknownValue = glyphDash
	usersJoinSep      = ", "

	// labelPercentSuffix trails each alert threshold entry, naming its unit.
	labelPercentSuffix = "%"
)

// Alert threshold range (percent): the inclusive bounds the entry accepts, so a
// typed value can't store a negative or above-100 threshold that never fires
// (or fires every tick). defaultAlertThreshold sits inside this range.
const (
	alertThresholdMin = 0
	alertThresholdMax = 100
)

// Settings-form geometry. The label column is a fixed min width so every row's
// control aligns to the same x; it's a component dimension, not on the spacing
// scale, so it carries its own literal-px const (it must clear the longest
// helper caption above).
const (
	settingRowGap     = space2XL // 24; vertical gap between setting rows
	settingLabelGap   = spaceXL  // 16; gap from the label column to its control
	settingLabelWidth = 200      // px; label-column min width (fits the longest caption)
	alertEntryWidth   = 64       // px; alert threshold numeric input (fits "100")
)

// Poll-interval segment labels, aligned by index with pollSecondsAllowed.
var pollIntervalLabels = []string{"1s", "2s", "5s"}

// Appearance segment labels, aligned by index with themeChoice (dark, light).
var themeLabels = []string{"Dark", "Light"}

// applyHooks are the composition root's live-appliers for the Settings rows
// whose effect lives outside this tab: the theme palette, the poller cadence,
// the GC memory cap, and the tray behavior. Each control persists its
// preference first, then calls its hook so the change takes effect immediately
// — no relaunch. Defined here at the consumer (idiomatic Go); app.go populates
// it because only the composition root holds the window, poller, and runtime.
// A nil hooks struct or nil field just persists (tests, or an effect that
// isn't wired on this platform).
type applyHooks struct {
	theme  func(themeChoice)
	poll   func(time.Duration)
	memCap func(enabled bool)
	tray   func(enabled bool)
}

// settingsView is the Settings tab. It holds the assembled root; controls
// capture the settings store and the live-apply hooks directly.
type settingsView struct {
	prefs settings
	apply *applyHooks
	root  fyne.CanvasObject
}

// systemInfo is the static machine description shown in the Settings "System"
// section (BZS253-74). Read once at startup and rendered directly — no ring
// buffer, no per-tick refresh. The app.go composition root maps
// monitor.HostSummary (+ core count + build version) into it, so this package
// never imports the monitor concretes.
type systemInfo struct {
	hostname string
	os       string // OS product + platform version, pre-joined
	kernel   string
	bootTime time.Time
	uptime   time.Duration
	cores    int
	users    []string // distinct logged-in usernames; empty omits the row
	version  string
}

// newSettingsView builds the Settings tab: the preferences form plus a
// read-only System section (BZS253-74) describing the monitored machine.
func newSettingsView(prefs settings, sys systemInfo, apply *applyHooks) *settingsView {
	v := &settingsView{prefs: prefs, apply: apply}

	form := container.New(layout.NewCustomPaddedVBoxLayout(settingRowGap),
		settingRow(labelSettingStartTab, helpSettingStartTab, v.startTabControl()),
		settingRow(labelSettingPoll, helpSettingPoll, v.pollControl()),
		settingRow(labelSettingMemCap, helpSettingMemCap, v.memCapControl()),
		settingRow(labelSettingTheme, helpSettingTheme, v.themeControl()),
		settingRow(labelSettingAutoUpdate, helpSettingAutoUpdate, v.autoUpdateControl()),
		settingRow(labelSettingTray, helpSettingTray, v.trayControl()),
		settingRow(labelSettingCPUAlert, helpSettingAlert, v.alertControl(thresholdCPU)),
		settingRow(labelSettingMemAlert, helpSettingAlert, v.alertControl(thresholdMemory)),
		settingRow(labelSettingDiskAlert, helpSettingDiskAlert, v.alertControl(thresholdDisk)),
	)
	panels := container.New(layout.NewCustomPaddedVBoxLayout(tabPad),
		newPanel(labelSettingsPanel, nil, form),
		newPanel(labelSystemPanel, nil, systemInfoRows(sys)),
	)

	head := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0),
		container.NewHBox(vCenter(newHeading(labelSettingsPageTitle))))
	// Scroll the panels rather than let them grow the window: a VScroll reports a
	// small min height, so more preference rows never force the window taller —
	// they scroll once they'd overflow the tab's height. The heading stays fixed
	// above the scrolled region.
	body := newTightBorder(head, nil, nil, nil, container.NewVScroll(panels))
	v.root = container.New(layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
	return v
}

func (v *settingsView) object() fyne.CanvasObject { return v.root }

// systemInfoRows lays out the System section's label→value rows. The users row
// is omitted entirely when the (degradable) users lookup returned nothing, so a
// failed lookup shows no row rather than an empty value (BZS253-74).
func systemInfoRows(sys systemInfo) fyne.CanvasObject {
	rows := []fyne.CanvasObject{
		infoRow(labelSysHostname, orDash(sys.hostname)),
		infoRow(labelSysOS, orDash(sys.os)),
		infoRow(labelSysKernel, orDash(sys.kernel)),
		infoRow(labelSysBootTime, formatBootTime(sys.bootTime)),
		infoRow(labelSysUptime, formatUptime(sys.uptime)),
		infoRow(labelSysCores, strconv.Itoa(sys.cores)),
	}
	if len(sys.users) > 0 {
		rows = append(rows, infoRow(labelSysUsers, strings.Join(sys.users, usersJoinSep)))
	}
	rows = append(rows, infoRow(labelSysVersion, orDash(sys.version)))
	return container.New(layout.NewCustomPaddedVBoxLayout(settingRowGap), rows...)
}

// infoRow is a static label→value row: the same fixed-width label column as
// settingRow (so both panels' values align to one x), beside a plain mono value.
// Unlike settingRow it carries no helper caption — these are read-only facts,
// not controls.
func infoRow(name, value string) fyne.CanvasObject {
	sizer := canvas.NewRectangle(color.Transparent)
	sizer.SetMinSize(fyne.NewSize(settingLabelWidth, 0))
	labelCol := container.NewStack(sizer, newColumnLabel(name))
	return container.New(layout.NewCustomPaddedHBoxLayout(settingLabelGap),
		vCenter(labelCol), vCenter(newTableText(value)))
}

// orDash shows v, or the unknown-value dash when the host lookup left it empty.
func orDash(v string) string {
	if v == "" {
		return labelUnknownValue
	}
	return v
}

// formatBootTime renders the boot timestamp, or the dash when it is unknown
// (zero) — e.g. gopsutil returned no boot time.
func formatBootTime(t time.Time) string {
	if t.IsZero() {
		return labelUnknownValue
	}
	return t.Format(bootTimeLayout)
}

// settingRow is one preference row: a fixed-width label column (uppercase name
// over a muted helper caption) beside its control. The control sits in an HBox,
// so it hugs its own width instead of stretching across the panel, and the
// fixed-width label column aligns every row's control to the same x. A
// transparent sizer enforces the column's min width while letting the row's
// height follow the two-line label.
func settingRow(name, help string, control fyne.CanvasObject) fyne.CanvasObject {
	sizer := canvas.NewRectangle(color.Transparent)
	sizer.SetMinSize(fyne.NewSize(settingLabelWidth, 0))
	labelCol := container.NewStack(sizer, vStackTight(newColumnLabel(name), newMeta(help)))
	return container.New(layout.NewCustomPaddedHBoxLayout(settingLabelGap),
		vCenter(labelCol), vCenter(control))
}

// startTabControl is a dropdown of the tab names; selecting one persists it as
// the launch tab. The current value seeds the selection.
func (v *settingsView) startTabControl() fyne.CanvasObject {
	tabs := tabDefs()
	names := make([]string, len(tabs))
	byName := make(map[string]tabID, len(tabs))
	for i, t := range tabs {
		names[i] = t.name
		byName[t.name] = t.id
	}

	sel := widget.NewSelect(names, func(chosen string) {
		if id, ok := byName[chosen]; ok {
			v.prefs.setStartTab(id)
		}
	})
	sel.SetSelected(tabName(tabs, v.prefs.startTab()))
	// Quiet the accent focus flash widget.Select paints while focused
	// (see flatFocus in theme.go).
	return flatFocus(sel)
}

// tabName resolves a tabID to its display name through indexOfTab (the single
// home for tabID→position), falling back to the first tab's name when the id
// isn't present (indexOfTab returns 0), so the dropdown always shows a valid
// label.
func tabName(tabs []tabDef, id tabID) string {
	i, _ := indexOfTab(tabs, id)
	return tabs[i].name
}

// pollControl is a segmented selector over the allowed cadences; the chosen
// segment persists as the poll interval and re-paces the running poller.
func (v *settingsView) pollControl() fyne.CanvasObject {
	// A stored value outside the set yields index -1; seed the first segment.
	active := max(slices.Index(pollSecondsAllowed, int(v.prefs.pollInterval().Seconds())), 0)
	return newSegmentedSelect(active, func(i int) {
		v.prefs.setPollSeconds(pollSecondsAllowed[i])
		if v.apply != nil && v.apply.poll != nil {
			// Read the duration back through the getter — the one authoritative
			// seconds→Duration conversion (prefs.go).
			v.apply.poll(v.prefs.pollInterval())
		}
	}, pollIntervalLabels...)
}

// memCapControl toggles the GC soft memory limit, applied immediately.
func (v *settingsView) memCapControl() fyne.CanvasObject {
	return newToggleChip(labelToggleEnabled, palette.Series[0], v.prefs.memoryCapEnabled(),
		func(on bool) {
			v.prefs.setMemoryCapEnabled(on)
			if v.apply != nil && v.apply.memCap != nil {
				v.apply.memCap(on)
			}
		})
}

// autoUpdateControl toggles opt-in auto-install (applied next launch). Off by
// default; enabling it is the user's consent to self-update without a click
// (ADR-010).
func (v *settingsView) autoUpdateControl() fyne.CanvasObject {
	return newToggleChip(labelToggleEnabled, palette.Series[0], v.prefs.autoUpdateEnabled(),
		func(on bool) { v.prefs.setAutoUpdateEnabled(on) })
}

// trayControl toggles minimize-to-tray-on-close, applied immediately. Off by
// default so quit-on-close stays unchanged until the user opts in (BZS253-76).
func (v *settingsView) trayControl() fyne.CanvasObject {
	return newToggleChip(labelToggleEnabled, palette.Series[0], v.prefs.minimizeToTrayEnabled(),
		func(on bool) {
			v.prefs.setMinimizeToTrayEnabled(on)
			if v.apply != nil && v.apply.tray != nil {
				v.apply.tray(on)
			}
		})
}

// alertControl is one metric's threshold-alert row control (BZS253-75): an
// enable chip beside a numeric percentage entry and its "%" suffix. Both write
// straight through to settings; the watcher reads them live each tick
// (threshold.go), so changes apply immediately.
func (v *settingsView) alertControl(m thresholdMetric) fyne.CanvasObject {
	toggle := newToggleChip(labelToggleEnabled, palette.Series[0], v.prefs.alertEnabled(m),
		func(on bool) { v.prefs.setAlertEnabled(m, on) })
	entry := newThresholdEntry(v.prefs.alertThreshold(m), func(pct int) {
		v.prefs.setAlertThreshold(m, pct)
	})
	sized := container.NewGridWrap(fyne.NewSize(alertEntryWidth, entry.MinSize().Height), entry)
	return container.New(layout.NewCustomPaddedHBoxLayout(settingLabelGap),
		vCenter(toggle), vCenter(flatFocus(sized)), vCenter(newTableText(labelPercentSuffix)))
}

// newThresholdEntry is the small numeric input for an alert threshold. It seeds
// the current value and persists any integer in [alertThresholdMin,
// alertThresholdMax] through onChange; a non-numeric or out-of-range value is
// silently ignored (never stored), so a bad entry can't wedge the watcher. No
// Validator is set on purpose — that would paint Fyne's ✓/✗ status icon in the
// field, which we don't want here.
func newThresholdEntry(value int, onChange func(pct int)) *widget.Entry {
	e := widget.NewEntry()
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.SetText(strconv.Itoa(value))
	e.OnChanged = func(s string) {
		if pct, ok := parseThreshold(s); ok {
			onChange(pct)
		}
	}
	return e
}

// parseThreshold reads a threshold entry's text as an in-range percentage.
func parseThreshold(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < alertThresholdMin || n > alertThresholdMax {
		return 0, false
	}
	return n, true
}

// themeControl is a segmented Dark/Light selector; the choice persists and
// swaps the live palette (the composition root rebuilds the widget tree, since
// palette colors are baked in at construction).
func (v *settingsView) themeControl() fyne.CanvasObject {
	return newSegmentedSelect(int(v.prefs.theme()), func(i int) {
		v.prefs.setTheme(themeChoice(i))
		if v.apply != nil && v.apply.theme != nil {
			v.apply.theme(themeChoice(i))
		}
	}, themeLabels...)
}
