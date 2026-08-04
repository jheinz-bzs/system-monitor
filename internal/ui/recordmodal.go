package ui

// Session-start modal (issue #82): tapping the status-bar record toggle opens a
// Fyne dialog before a session begins, so the compact-output and top-processes
// sidecar options — previously reachable only from the headless recording
// binary's flags — are available in the GUI, and the save location is shown and
// editable before anything is created. The modal lives entirely in the UI layer:
// it translates the choices into a recorder spec and a path and hands them to a
// start callback the composition root wires, so the recorder's data path stays
// Fyne-free. Cancel (or dismissing the dialog) leaves the session idle.

import (
	"errors"
	"log"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/ncruces/zenity"

	"github.com/josephheinz/system-monitor/internal/recorder"
	"github.com/josephheinz/system-monitor/internal/recorder/columns"
)

// Record modal copy. The top-processes hint mirrors the headless binary's
// --processes help text: 0 leaves the sidecar off.
const (
	recordModalTitle     = "Record tracking session"
	recordModalSave      = "Start recording"
	recordModalCancel    = "Cancel"
	labelRecordCompact   = "Compact output"
	helpRecordCompact    = "gzip-compressed .csv.gz"
	labelRecordProcesses = "Top processes per tick"
	helpRecordProcesses  = "N busiest by CPU; 0 = off"
	// helpRecordProcessesUnavailable replaces the top-processes caption when the
	// process collector failed to start, so the disabled field explains why the
	// sidecar can't be recorded (issue #89) rather than silently dropping it.
	helpRecordProcessesUnavailable = "process collector unavailable; sidecar off"
	recordTopDefault               = "0"
	labelRecordLocation            = "Save to"
	helpRecordLocation             = "editable; Browse… to pick"
	labelRecordBrowse              = "Browse…"
)

// showRecordModal presents the session-start choices — compact output, a
// top-processes count, and the save location — and hands the confirmed spec and
// path to onStart. It runs on the UI goroutine (a record-toggle tap); the
// confirm callback is a Fyne dialog callback, so the recorder build and file
// creation happen there too. When processesAvailable is false the top-processes
// field is disabled and its caption warns — the sidecar can't be recorded, so
// the choice is surfaced rather than silently dropped (issue #89). The Browse
// button reuses the native save dialog the toggle used directly before this
// modal, prefilled with the current location; zenity blocks, so it runs on its
// own goroutine with the result marshalled back via fyne.Do.
func showRecordModal(win fyne.Window, processesAvailable bool, onStart func(recorder.OptionsSpec, string)) {
	defaultName := columns.FileName(time.Now())

	pathEntry := widget.NewEntry()
	pathEntry.SetText(defaultName)

	compact := widget.NewCheck(labelRecordCompact, nil)
	// Keep the displayed filename truthful when compact is toggled: rewrite the
	// .gz suffix only while the name is still the untouched default, so a
	// user-typed name is left alone.
	compact.OnChanged = func(on bool) {
		current := pathEntry.Text
		if current == defaultName || current == columns.CompactFilePath(defaultName) {
			if on {
				pathEntry.SetText(columns.CompactFilePath(defaultName))
			} else {
				pathEntry.SetText(defaultName)
			}
		}
	}

	topField := widget.NewEntry()
	topField.SetText(recordTopDefault)
	topHelp := helpRecordProcesses
	if !processesAvailable {
		topHelp = helpRecordProcessesUnavailable
		topField.Disable()
	}

	browse := widget.NewButton(labelRecordBrowse, func() {
		current := pathEntry.Text
		go func() {
			picked, err := zenity.SelectFileSave(
				zenity.Title(recordDialogTitle),
				zenity.ConfirmOverwrite(),
				zenity.Filename(current),
				zenity.FileFilters{{Name: recordFilterName, Patterns: []string{recordFilterPattern}}},
			)
			if err != nil {
				if !errors.Is(err, zenity.ErrCanceled) {
					log.Printf("record location dialog: %v", err)
				}
				return
			}
			fyne.Do(func() { pathEntry.SetText(picked) })
		}()
	})
	location := container.NewBorder(nil, nil, nil, browse, pathEntry)

	// Rows reuse the Settings tab's label-column layout so the modal reads like
	// the rest of the app: a fixed-width uppercase label over a muted caption,
	// beside the control.
	rows := container.New(layout.NewCustomPaddedVBoxLayout(settingRowGap),
		settingRow(labelRecordCompact, helpRecordCompact, compact),
		settingRow(labelRecordProcesses, topHelp, topField),
		settingRow(labelRecordLocation, helpRecordLocation, location),
	)

	d := dialog.NewCustomConfirm(recordModalTitle, recordModalSave, recordModalCancel, rows,
		func(ok bool) {
			if !ok {
				return // cancelled: stay idle
			}
			loc := strings.TrimSpace(pathEntry.Text)
			if !columns.ValidPath(loc) {
				return // a cleared location writes nothing; stay idle
			}
			onStart(recorder.OptionsSpec{
				Compact: compact.Checked,
				TopN:    recorder.ParseTopN(topField.Text),
			}, loc)
		}, win)
	d.Show()
}
