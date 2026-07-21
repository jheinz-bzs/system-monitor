package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// newTestProcessesView builds the view against fixed rows with a recording
// killer and no window (kills run unconfirmed), rendered so taps resolve.
func newTestProcessesView(t *testing.T, killed *[]PID) *processesView {
	t.Helper()
	v := newProcessesView(fixedProcs(testRows()),
		processKillerFunc(func(pid PID) error {
			*killed = append(*killed, pid)
			return nil
		}), nil)
	w := test.NewWindow(v.object())
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(1100, 700))
	v.refresh()
	return v
}

// Row taps accumulate checked rows, the Kill label carries the count, and
// killing fires once per selected PID in display order.
func TestProcessesViewMassKill(t *testing.T) {
	var killed []PID
	v := newTestProcessesView(t, &killed)

	v.tapRow(0) // PID 20 (CPU desc: 20, 30, 10)
	v.tapRow(2) // PID 10
	if v.killBtn.Text != "Kill (2)" {
		t.Errorf("kill label = %q, want %q", v.killBtn.Text, "Kill (2)")
	}

	v.killSelected()
	if !pidsEqual(killed, []PID{20, 10}) {
		t.Errorf("killed = %v, want [20 10] (display order)", killed)
	}
}

// The check column's header tap is the select-all-visible / clear-all toggle,
// and the Kill action tracks it: enabled with the count, disabled when empty.
func TestProcessesViewHeaderSelectAll(t *testing.T) {
	var killed []PID
	v := newTestProcessesView(t, &killed)

	v.tapHeader(0) // check column header → select all visible
	if !v.adapter.allVisibleSelected() {
		t.Fatal("header tap did not select all visible rows")
	}
	if v.killBtn.Text != "Kill (3)" || v.killBtn.Disabled() {
		t.Errorf("kill button = %q disabled=%v, want enabled Kill (3)", v.killBtn.Text, v.killBtn.Disabled())
	}

	v.tapHeader(0) // all selected → clear
	if v.adapter.selectionCount() != 0 {
		t.Errorf("selectionCount = %d after clear-all, want 0", v.adapter.selectionCount())
	}
	if v.killBtn.Text != labelKill || !v.killBtn.Disabled() {
		t.Errorf("kill button = %q disabled=%v, want disabled %q", v.killBtn.Text, v.killBtn.Disabled(), labelKill)
	}
}

// A header tap on the check column must not disturb the sort; a tap on a
// sortable column still sorts.
func TestProcessesViewCheckHeaderDoesNotSort(t *testing.T) {
	var killed []PID
	v := newTestProcessesView(t, &killed)

	v.tapHeader(0)
	if v.adapter.sortCol != sortByCPU || v.adapter.sortDir != sortDescending {
		t.Errorf("sort after check-header tap = %v/%v, want CPU desc unchanged",
			v.adapter.sortCol, v.adapter.sortDir)
	}
}
