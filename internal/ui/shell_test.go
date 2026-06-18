package ui

import "testing"

// noTabSelected marks "selectIndex was never called" in the nav tests, distinct
// from any real tab index.
const noTabSelected = -1

// wireProcessNav must route showProcesses to the Processes tab without touching
// its selection, and showProcess to the same tab plus its highlight hook — the
// CPU "→ all processes" link vs. a tapped CPU/Memory row.
func TestWireProcessNavSelectsAndHighlights(t *testing.T) {
	tabs := []tabDef{{id: tabCPU}, {id: tabProcesses}}
	selected := noTabSelected
	highlighted := PID(-1)
	selectors := map[tabID]func(PID){
		tabProcesses: func(p PID) { highlighted = p },
	}

	nav := &crossNav{}
	wireProcessNav(nav, tabs, func(i int) { selected = i }, selectors)

	nav.showProcesses()
	if selected != 1 {
		t.Errorf("showProcesses selected tab %d, want 1 (Processes)", selected)
	}
	if highlighted != -1 {
		t.Errorf("showProcesses highlighted %d, want no highlight", highlighted)
	}

	nav.showProcess(42)
	if selected != 1 {
		t.Errorf("showProcess selected tab %d, want 1 (Processes)", selected)
	}
	if highlighted != 42 {
		t.Errorf("showProcess highlighted %d, want 42", highlighted)
	}
}

// Without a Processes tab the navigator stays unwired, so its callers no-op
// instead of jumping to a tab that isn't there.
func TestWireProcessNavNoProcessesTab(t *testing.T) {
	tabs := []tabDef{{id: tabCPU}, {id: tabMemory}}
	selected := noTabSelected

	nav := &crossNav{}
	wireProcessNav(nav, tabs, func(i int) { selected = i }, nil)

	nav.showProcesses()
	nav.showProcess(42)
	if nav.selectProcess != nil {
		t.Error("navigator wired despite no Processes tab")
	}
	if selected != noTabSelected {
		t.Errorf("selectIndex called (tab %d) with no Processes tab", selected)
	}
}
