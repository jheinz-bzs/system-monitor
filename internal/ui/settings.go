package ui

// Settings tab (BZS253-72): a single panel of preference rows, styled from the
// design-system tokens like every other tab. Each control writes straight
// through to the persisted settings; the effects are applied at startup
// (app.go / theme.go), so every row is documented "next launch". The tab is
// static — no live data — so it ships no refresh callback.

import (
	"image/color"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const (
	labelSettingsPageTitle = "Settings"
	labelSettingsPanel     = "Preferences"

	labelSettingStartTab = "Start tab"
	labelSettingPoll     = "Poll interval"
	labelSettingMemCap   = "Memory cap"
	labelSettingTheme    = "Appearance"

	helpSettingStartTab = "Tab shown on launch"
	helpSettingPoll     = "Sampling cadence · next launch"
	helpSettingMemCap   = "GC soft heap limit · next launch"
	helpSettingTheme    = "Color palette · next launch"

	labelMemCapChip = "Enabled"
)

// Settings-form geometry. The label column is a fixed min width so every row's
// control aligns to the same x; it's a component dimension, not on the spacing
// scale, so it carries its own literal-px const (it must clear the longest
// helper caption above).
const (
	settingRowGap     = space2XL // 24; vertical gap between setting rows
	settingLabelGap   = spaceXL  // 16; gap from the label column to its control
	settingLabelWidth = 200      // px; label-column min width (fits the longest caption)
)

// Poll-interval segment labels, aligned by index with pollSecondsAllowed.
var pollIntervalLabels = []string{"1s", "2s", "5s"}

// Appearance segment labels, aligned by index with themeChoice (dark, light).
var themeLabels = []string{"Dark", "Light"}

// settingsView is the Settings tab. It holds only the assembled root; controls
// capture the settings store directly and need no further state.
type settingsView struct {
	prefs settings
	root  fyne.CanvasObject
}

// newSettingsView builds the Settings tab from the persisted preferences.
func newSettingsView(prefs settings) *settingsView {
	v := &settingsView{prefs: prefs}

	form := container.New(layout.NewCustomPaddedVBoxLayout(settingRowGap),
		settingRow(labelSettingStartTab, helpSettingStartTab, v.startTabControl()),
		settingRow(labelSettingPoll, helpSettingPoll, v.pollControl()),
		settingRow(labelSettingMemCap, helpSettingMemCap, v.memCapControl()),
		settingRow(labelSettingTheme, helpSettingTheme, v.themeControl()),
	)
	panel := newPanel(labelSettingsPanel, nil, form)

	head := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0),
		container.NewHBox(vCenter(newHeading(labelSettingsPageTitle))))
	body := newTightBorder(head, nil, nil, nil, panel)
	v.root = container.New(layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
	return v
}

func (v *settingsView) object() fyne.CanvasObject { return v.root }

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
// segment persists as the poll interval (applied next launch).
func (v *settingsView) pollControl() fyne.CanvasObject {
	// A stored value outside the set yields index -1; seed the first segment.
	active := max(slices.Index(pollSecondsAllowed, int(v.prefs.pollInterval().Seconds())), 0)
	return newSegmentedSelect(active, func(i int) {
		v.prefs.setPollSeconds(pollSecondsAllowed[i])
	}, pollIntervalLabels...)
}

// memCapControl toggles the GC soft memory limit (applied next launch).
func (v *settingsView) memCapControl() fyne.CanvasObject {
	return newToggleChip(labelMemCapChip, palette.Series[0], v.prefs.memoryCapEnabled(),
		func(on bool) { v.prefs.setMemoryCapEnabled(on) })
}

// themeControl is a segmented Dark/Light selector; the choice persists as the
// startup palette (applied next launch).
func (v *settingsView) themeControl() fyne.CanvasObject {
	return newSegmentedSelect(int(v.prefs.theme()), func(i int) {
		v.prefs.setTheme(themeChoice(i))
	}, themeLabels...)
}
