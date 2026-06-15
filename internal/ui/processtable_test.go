package ui

import "testing"

// fixedProcs adapts a literal row set to allProcessSource. Like the production
// adapter in app.go, it returns a fresh slice per call (Snapshot sorts the
// returned slice in place).
func fixedProcs(rows []processRow) allProcessSource {
	return allProcessSourceFunc(func() []processRow {
		out := make([]processRow, len(rows))
		copy(out, rows)
		return out
	})
}

// snapshotPIDs runs a Snapshot and returns the resulting row order as PIDs.
func snapshotPIDs(m *processTableModel) []PID {
	m.Snapshot()
	out := make([]PID, len(m.rowPIDs))
	copy(out, m.rowPIDs)
	return out
}

func pidsEqual(a, b []PID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func testRows() []processRow {
	return []processRow{
		{pid: 10, name: "alpha", user: "root", cpu: 5, mem: 100, status: statusSleeping},
		{pid: 20, name: "Bravo", user: "you", cpu: 30, mem: 50, status: statusRunning},
		{pid: 30, name: "charlie", user: "you", cpu: 30, mem: 200, status: statusStopped},
	}
}

func TestAllProcessSnapshotDefaultsToCPUDescending(t *testing.T) {
	s := newAllProcessTableSource(fixedProcs(testRows()))

	// 20 and 30 tie on CPU; ascending-PID tiebreak keeps 20 first.
	want := []PID{20, 30, 10}
	if got := snapshotPIDs(s); !pidsEqual(got, want) {
		t.Errorf("default order = %v, want %v (CPU%% desc, PID tiebreak)", got, want)
	}
}

func TestAllProcessFilterIsCaseInsensitive(t *testing.T) {
	s := newAllProcessTableSource(fixedProcs(testRows()))
	s.setFilter("BRA")

	want := []PID{20}
	if got := snapshotPIDs(s); !pidsEqual(got, want) {
		t.Errorf("filtered rows = %v, want %v ('BRA' should match 'Bravo')", got, want)
	}
}

func TestToggleSortFlipsAndSwitchesColumns(t *testing.T) {
	s := newAllProcessTableSource(fixedProcs(testRows()))

	s.toggleSort(sortByCPU) // already active → flip to ascending
	want := []PID{10, 20, 30}
	if got := snapshotPIDs(s); !pidsEqual(got, want) {
		t.Errorf("after flip, order = %v, want %v (CPU%% asc)", got, want)
	}

	s.toggleSort(sortByName) // new column → its default (ascending)
	want = []PID{10, 20, 30} // alpha, Bravo, charlie — case-insensitive
	if got := snapshotPIDs(s); !pidsEqual(got, want) {
		t.Errorf("by name, order = %v, want %v (name asc, case-insensitive)", got, want)
	}

	s.toggleSort(sortByMem) // usage column defaults descending
	want = []PID{30, 10, 20}
	if got := snapshotPIDs(s); !pidsEqual(got, want) {
		t.Errorf("by memory, order = %v, want %v (mem desc)", got, want)
	}
}

func TestSelectionFollowsPIDAcrossResort(t *testing.T) {
	s := newAllProcessTableSource(fixedProcs(testRows()))
	s.Snapshot()            // CPU desc: 20, 30, 10
	s.selectRow(2)          // selects PID 10
	s.toggleSort(sortByPID) // re-sort: 10, 20, 30
	s.Snapshot()

	if s.highlightedRow() != 0 {
		t.Errorf("highlightedRow = %d, want 0 (PID 10 moved to the top)", s.highlightedRow())
	}
	if pid, ok := s.selectedPID(); !ok || pid != 10 {
		t.Errorf("selectedPID = %d,%v, want 10,true", pid, ok)
	}
}

// Tapping a row selects it; tapping the same row again clears the selection
// (toggle-to-deselect), while tapping a different row moves the selection.
func TestToggleRowDeselects(t *testing.T) {
	s := newAllProcessTableSource(fixedProcs(testRows()))
	s.Snapshot() // CPU desc: 20, 30, 10

	s.toggleRow(0) // select PID 20
	if pid, ok := s.selectedPID(); !ok || pid != 20 {
		t.Fatalf("after first tap: selectedPID = %d,%v, want 20,true", pid, ok)
	}

	s.toggleRow(0) // same row → deselect
	if _, ok := s.selectedPID(); ok {
		t.Error("second tap on the selected row left it selected; want cleared")
	}
	if s.highlightedRow() != noTableRow {
		t.Errorf("highlightedRow = %d, want noTableRow after deselect", s.highlightedRow())
	}

	s.toggleRow(0) // re-select 20
	s.toggleRow(1) // different row → move selection to PID 30
	if pid, ok := s.selectedPID(); !ok || pid != 30 {
		t.Errorf("after tapping a different row: selectedPID = %d,%v, want 30,true", pid, ok)
	}
}

func TestSelectionClearsWhenProcessDisappears(t *testing.T) {
	rows := testRows()
	s := newAllProcessTableSource(fixedProcs(rows[:2]))
	s.Snapshot()
	s.selectRow(0)

	gone := newAllProcessTableSource(fixedProcs(rows[:1]))
	gone.selected, gone.hasSelected = s.selected, s.hasSelected
	gone.Snapshot()

	if _, ok := gone.selectedPID(); ok {
		t.Error("selection survived its process disappearing; want it cleared")
	}
	if gone.highlightedRow() != noTableRow {
		t.Errorf("highlightedRow = %d, want noTableRow", gone.highlightedRow())
	}
}

func TestSelectionClearsWhenFilteredOut(t *testing.T) {
	s := newAllProcessTableSource(fixedProcs(testRows()))
	s.Snapshot()
	s.selectRow(0) // PID 20, "Bravo"
	s.setFilter("alpha")
	s.Snapshot()

	if _, ok := s.selectedPID(); ok {
		t.Error("selection survived being filtered out; want it cleared")
	}
}

func TestTextFilterMatchesUserAndPID(t *testing.T) {
	s := newAllProcessTableSource(fixedProcs(testRows()))

	s.setFilter("ROOT") // matches user, case-insensitive
	if got := snapshotPIDs(s); !pidsEqual(got, []PID{10}) {
		t.Errorf("user-text filter rows = %v, want [10]", got)
	}

	s.setFilter("3") // matches PID 30 (and nothing else: no name/user has a 3)
	if got := snapshotPIDs(s); !pidsEqual(got, []PID{30}) {
		t.Errorf("pid-text filter rows = %v, want [30]", got)
	}
}

func TestUserAndStatusFiltersCombine(t *testing.T) {
	s := newAllProcessTableSource(fixedProcs(testRows()))

	s.setUserFilter("you")
	if got := snapshotPIDs(s); !pidsEqual(got, []PID{20, 30}) {
		t.Errorf("user filter rows = %v, want [20 30]", got)
	}

	s.setStatusFilter(statusStopped)
	if got := snapshotPIDs(s); !pidsEqual(got, []PID{30}) {
		t.Errorf("user+status filter rows = %v, want [30]", got)
	}

	s.setUserFilter("")
	s.setStatusFilter("")
	if got := snapshotPIDs(s); len(got) != 3 {
		t.Errorf("cleared filters rows = %v, want all 3", got)
	}
}

func TestTallyCountsWholeMachineDespiteFilters(t *testing.T) {
	s := newAllProcessTableSource(fixedProcs(testRows()))
	s.setFilter("alpha")
	s.Snapshot()

	total, high := s.counts()
	if total != 3 {
		t.Errorf("total = %d, want 3 (unfiltered)", total)
	}
	if high != 3 {
		// alpha sits exactly at the 5%% threshold; at-or-above counts.
		t.Errorf("highUsage = %d, want 3 (cpu 5, 30, 30 all >= %d)", high, highUsageCPUPct)
	}
	if users := s.userOptions(); len(users) != 2 || users[0] != "root" || users[1] != "you" {
		t.Errorf("userOptions = %v, want [root you]", users)
	}
}

// The CPU and Memory top-process tables must record a PID per row each
// Snapshot so a tapped row resolves to its process for cross-tab navigation;
// out-of-range indices (a row that vanished before the tap) resolve to false.
func TestTopProcessTablesResolveRowPID(t *testing.T) {
	rows := []processRow{
		{pid: 3412, name: "chrome", cpu: 42, mem: 5 << 30},
		{pid: 540, name: "postgres", cpu: 3, mem: 1 << 28},
	}

	cpu := newCPUTableSource(fixedProcs(rows))
	cpu.Snapshot()
	assertPIDAt(t, "cpu", cpu.pidAt, 0, 3412)
	assertPIDAt(t, "cpu", cpu.pidAt, 1, 540)
	if _, ok := cpu.pidAt(2); ok {
		t.Error("cpu pidAt(2) resolved past the last row")
	}

	mem := newMemTableSource(fixedProcs(rows), testTotalMem)
	mem.Snapshot()
	assertPIDAt(t, "mem", mem.pidAt, 0, 3412)
	assertPIDAt(t, "mem", mem.pidAt, 1, 540)
	if _, ok := mem.pidAt(-1); ok {
		t.Error("mem pidAt(-1) resolved a negative index")
	}
}

// The top-N tables (CPU/Memory) keep their top-N selection but re-order that set
// by the active sort column — a header tap re-sorts the displayed rows.
func TestTopProcessTableHeaderResort(t *testing.T) {
	rows := []processRow{
		{pid: 10, name: "zeta", cpu: 90, mem: 100},
		{pid: 20, name: "alpha", cpu: 10, mem: 300},
	}
	cpu := newCPUTableSource(fixedProcs(rows))

	// Default order is CPU% descending.
	cpu.Snapshot()
	if got := rowPIDsCopy(cpu.rowPIDs); !pidsEqual(got, []PID{10, 20}) {
		t.Errorf("default order = %v, want [10 20] (CPU%% desc)", got)
	}

	// Re-sorting by name reorders the same top-N set, ascending.
	cpu.toggleSort(sortByName)
	cpu.Snapshot()
	if got := rowPIDsCopy(cpu.rowPIDs); !pidsEqual(got, []PID{20, 10}) {
		t.Errorf("by-name order = %v, want [20 10] (alpha, zeta)", got)
	}
}

// rowPIDsCopy returns an independent copy of a recorded row-PID slice.
func rowPIDsCopy(pids []PID) []PID {
	out := make([]PID, len(pids))
	copy(out, pids)
	return out
}

// syncSortHeaders must mark only the active, sortable column. The Memory table's
// Mem% column shares RSS's ordering and is non-sortable, so RSS owns the marker.
func TestSyncSortHeadersMarksActiveSortableColumn(t *testing.T) {
	src := newMemTableSource(fixedProcs(nil), testTotalMem)
	table := newDataTable(src, tableColumns(columnDefs(src.cols)...))

	syncSortHeaders(table, src) // default: sortByMem desc

	const rssCol, barCol, pctCol = 3, 4, 5
	if got := table.cols[rssCol].header; got != colHeaderRSS+sortDescMarker {
		t.Errorf("RSS header = %q, want %q", got, colHeaderRSS+sortDescMarker)
	}
	if got := table.cols[pctCol].header; got != colHeaderPctMem {
		t.Errorf("Mem%% header = %q, want unmarked %q", got, colHeaderPctMem)
	}

	// Tapping the non-sortable bar column must not change the sort.
	tapSortHeader(table, src, barCol)
	if src.sortCol != sortByMem || src.sortDir != sortDescending {
		t.Errorf("sort after bar-column tap = %v/%v, want sortByMem/desc (unchanged)", src.sortCol, src.sortDir)
	}

	// Tapping RSS flips it to ascending and re-marks.
	tapSortHeader(table, src, rssCol)
	if src.sortDir != sortAscending {
		t.Errorf("RSS re-tap dir = %v, want ascending", src.sortDir)
	}
	if got := table.cols[rssCol].header; got != colHeaderRSS+sortAscMarker {
		t.Errorf("RSS header after flip = %q, want %q", got, colHeaderRSS+sortAscMarker)
	}
}

func assertPIDAt(t *testing.T, label string, pidAt func(int) (PID, bool), row int, want PID) {
	t.Helper()
	got, ok := pidAt(row)
	if !ok || got != want {
		t.Errorf("%s pidAt(%d) = %d,%v, want %d,true", label, row, got, ok, want)
	}
}

func TestSelectPIDResolvesOnNextSnapshot(t *testing.T) {
	s := newAllProcessTableSource(fixedProcs(testRows()))
	s.selectPID(30)
	s.Snapshot() // CPU desc: 20, 30, 10

	if got := s.rowIndexOf(30); got != 1 {
		t.Errorf("rowIndexOf(30) = %d, want 1", got)
	}
	if s.highlightedRow() != 1 {
		t.Errorf("highlightedRow = %d, want 1", s.highlightedRow())
	}
}
