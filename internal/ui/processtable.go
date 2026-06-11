package ui

// Process table adapters — wire process data into the generic dataTable widget.
//
// This file is the process-domain analog of cpu.go's relationship to lineChart:
// it knows about processRow, formats cell values, and declares the wireframes'
// process tables — the CPU tab's top-N (PID / Process / User / CPU% / bar /
// Mem, newProcessTable), the Memory tab's top-N (PID / Process / User / RSS /
// bar / %Mem, newMemProcessTable), and the Processes tab's full sortable/
// filterable table (newAllProcessTable). The generic dataTable widget
// (datatable.go) never sees processRow.
//
// processSource, memProcessSource, allProcessSource, and processKiller are the
// seams between the monitor layer and these adapters; they are consumed here
// and implemented in app.go (the composition root), which is the only place
// that knows both monitor.ProcessInfo and processRow.

import (
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

// Process data constants.
const (
	topCPUProcessLimit    = 10 // rows returned by topByCPU
	topMemProcessLimit    = 10 // rows returned by topByMemory
	processTableRowHeight = 29 // px; passed as rowHeight() option to dataTable
)

// procTableRowPool sizes the all-processes table's renderer pool: comfortably
// more rows than the tallest realistic viewport shows at once (a full-height
// 4K pane is ~45 rows at 29px, plus the windowing overdraw).
const procTableRowPool = 48

// Process table column widths (px), taken from the CPU tab wireframe's table
// and shared by the Memory tab's table (the same design-system component; the
// flex name column absorbs the wider pane). All are off-grid component
// dimensions, not spacing-scale steps.
const (
	procColPIDW    = 74  // px; PID
	procColNameW   = 195 // px; process name
	procColUserW   = 74  // px; owning user
	procColCPUW    = 87  // px; CPU% value — right-aligned numeric
	procColBarW    = 90  // px; inline mini-bar
	procColMemW    = 74  // px; resident memory — right-aligned numeric
	procColPctW    = 74  // px; %Mem value — right-aligned numeric
	procColStatusW = 110 // px; status pill (widest pill + cell padding)
)

// Column header labels. Named constants so changes propagate from one place.
const (
	colHeaderPID     = "PID"
	colHeaderProcess = "Process"
	colHeaderUser    = "User"
	colHeaderCPU     = "CPU%"
	colHeaderMem     = "Mem"
	colHeaderRSS     = "RSS"
	colHeaderPctMem  = "Mem%"
	colHeaderStatus  = "Status"
)

// procStatus is the display vocabulary for a process's coarse state — the
// ui-side mirror of monitor.ProcState (app.go converts), kept separate so the
// adapters stay free of cross-layer imports. Empty means unknown.
type procStatus string

const (
	statusRunning  procStatus = "running"
	statusSleeping procStatus = "sleeping"
	statusStopped  procStatus = "stopped"
)

// procStatusFilterable lists the states the "status:" filter offers, in menu
// order.
var procStatusFilterable = []procStatus{statusRunning, statusSleeping, statusStopped}

// statusPillKind maps a process status onto the design-system pill roles the
// wireframe assigns: running green, stopped yellow, everything else neutral.
func statusPillKind(s procStatus) statusKind {
	switch s {
	case statusRunning:
		return status.Healthy
	case statusStopped:
		return status.Warning
	default:
		return status.Neutral
	}
}

// highUsageCPUPct is the CPU% at or above which a process counts as "high
// usage" in the page head's readout — the wireframe's cut, where the 6
// high-usage processes of 187 are those at 5%+ (the treemap's named tiles).
const highUsageCPUPct = 5

// sortDescMarker / sortAscMarker tag the column a table is sorted by. The
// wireframes draw ▾ (U+25BE), which the bundled IBM Plex fonts lack; the
// arrows are the closest covered glyphs.
const (
	sortDescMarker = " ↓"
	sortAscMarker  = " ↑"
)

// sortMarker returns the header marker for a sort direction.
func sortMarker(d sortDirection) string {
	if d == sortAscending {
		return sortAscMarker
	}
	return sortDescMarker
}

// processSource is the data seam between the monitor layer and this adapter.
// Defined here at the consumer (ui package) per idiomatic Go; app.go adapts
// the concrete ProcessCollector to this interface without creating a cross-
// layer import.
type processSource interface {
	topByCPU(n int) []processRow
}

// processSourceFunc adapts any func(int)[]processRow to processSource.
type processSourceFunc func(n int) []processRow

func (f processSourceFunc) topByCPU(n int) []processRow { return f(n) }

// PID is a typed process identifier, carried as a first-class value so
// cross-tab navigation links resolve without string parsing or type assertions.
type PID int32

// processRow is the display-layer shape for one process. app.go selects and
// converts from monitor.ProcessInfo when building the adapter, so this type
// never appears in the monitor package.
type processRow struct {
	pid    PID
	name   string
	user   string
	cpu    float64 // 0..100, machine-wide scale
	mem    uint64  // resident set bytes
	status procStatus
}

// processTableSource implements TableSource for a processSource. It formats
// each processRow into the six wireframe columns; the CPU value feeds both the
// numeric column and the bar column's fill fraction.
type processTableSource struct {
	src processSource
}

// Snapshot calls topByCPU on every Refresh to return a live snapshot, exactly
// as series.Source.Values() is called on every lineChart arrange().
func (s *processTableSource) Snapshot() [][]tableCell {
	rows := s.src.topByCPU(topCPUProcessLimit)
	cells := make([][]tableCell, len(rows))
	for i, r := range rows {
		cells[i] = append(
			processIdentityCells(r),
			tableCell{text: formatPercent1(r.cpu)},
			tableCell{frac: r.cpu / percentMax},
			tableCell{text: formatBytesShort(r.mem)},
		)
	}
	return cells
}

// processIdentityCells are the leading PID / Process / User cells every
// process table's rows start with (shared by all three Snapshot adapters).
func processIdentityCells(r processRow) []tableCell {
	return []tableCell{
		{text: strconv.Itoa(int(r.pid))},
		{text: r.name},
		{text: shortUsername(r.user)},
	}
}

// processIdentityColumns are the leading PID / Process / User column
// declarations every process table starts with. Returned fresh per call so a
// caller appending its value columns can't mutate a shared backing array.
func processIdentityColumns() []tableColumn {
	return []tableColumn{
		{header: colHeaderPID, width: procColPIDW, align: fyne.TextAlignLeading},
		{header: colHeaderProcess, width: procColNameW, align: fyne.TextAlignLeading, color: palette.Text, flex: true},
		{header: colHeaderUser, width: procColUserW, align: fyne.TextAlignLeading},
	}
}

// shortUsername strips the Windows "DOMAIN\" qualifier so the User column
// shows the bare account name the wireframe's narrow column expects.
func shortUsername(user string) string {
	if i := strings.LastIndexByte(user, '\\'); i >= 0 {
		return user[i+1:]
	}
	return user
}

// formatPercent1 renders a percentage for the tables' numeric columns (CPU%,
// %Mem): one decimal, no unit suffix ("42.0"), matching the wireframe cells.
func formatPercent1(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// newProcessTable builds a *dataTable configured for the process view: the six
// wireframe columns fed by src. The name column renders in primary text; the
// rest keep the table-data default (text-2).
func newProcessTable(src processSource) *dataTable {
	return newDataTable(
		&processTableSource{src: src},
		tableColumns(append(
			processIdentityColumns(),
			tableColumn{header: colHeaderCPU, width: procColCPUW, align: fyne.TextAlignTrailing},
			tableColumn{header: "", width: procColBarW, kind: columnBar},
			tableColumn{header: colHeaderMem, width: procColMemW, align: fyne.TextAlignTrailing},
		)...),
		rowHeight(processTableRowHeight),
	)
}

// memProcessSource is the Memory tab's data seam to the monitor layer: the
// top-by-memory analog of processSource. Defined here at the consumer per
// idiomatic Go; app.go adapts the concrete ProcessCollector to it.
type memProcessSource interface {
	topByMemory(n int) []processRow
}

// memProcessSourceFunc adapts any func(int)[]processRow to memProcessSource.
type memProcessSourceFunc func(n int) []processRow

func (f memProcessSourceFunc) topByMemory(n int) []processRow { return f(n) }

// memBarFullScalePct is the Mem% value at which a memory-table bar fills its
// whole track, measured from the wireframe's fills (its 5.6%-of-total row
// fills 56% of the track). Percentage points, not a fraction.
const memBarFullScalePct = 10

// memProcessTableSource implements TableSource for the Memory tab's table. It
// formats each processRow into the wireframe's columns; total (physical memory
// bytes) scales the Mem% column and the bars.
type memProcessTableSource struct {
	src   memProcessSource
	total uint64
}

// Snapshot calls topByMemory on every Refresh to return a live snapshot. The
// bars fill linearly with each row's share of physical memory, reaching a full
// track at memBarFullScalePct (the wireframe's scale — against the raw 0..100%
// domain every bar would sit near-empty).
func (s *memProcessTableSource) Snapshot() [][]tableCell {
	rows := s.src.topByMemory(topMemProcessLimit)
	cells := make([][]tableCell, len(rows))
	for i, r := range rows {
		pct := byteFraction(r.mem, s.total) * percentMax
		cells[i] = append(
			processIdentityCells(r),
			tableCell{text: formatBytesShort(r.mem)},
			tableCell{frac: min(pct/memBarFullScalePct, 1)},
			tableCell{text: formatPercent1(pct)},
		)
	}
	return cells
}

// byteFraction returns part/whole as a 0..1 fraction, 0 when whole is zero
// (an unknown total must not divide by zero or fill a bar).
func byteFraction(part, whole uint64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}

// newMemProcessTable builds a *dataTable configured for the Memory tab's
// top-processes pane: the wireframe's columns fed by src, with total physical
// memory scaling the %Mem column. minVisibleRows keeps every top-N row
// reachable when the table sits in a scroll container.
func newMemProcessTable(src memProcessSource, total uint64) *dataTable {
	return newDataTable(
		&memProcessTableSource{src: src, total: total},
		tableColumns(append(
			processIdentityColumns(),
			tableColumn{header: colHeaderRSS + sortDescMarker, width: procColMemW, align: fyne.TextAlignTrailing},
			tableColumn{header: "", width: procColBarW, kind: columnBar},
			tableColumn{header: colHeaderPctMem, width: procColPctW, align: fyne.TextAlignTrailing},
		)...),
		rowHeight(processTableRowHeight),
		minVisibleRows(topMemProcessLimit),
	)
}

// allProcessSource is the Processes tab's data seam to the monitor layer: the
// complete process list, unordered (this adapter owns ordering — sort state
// must live UI-side to survive refreshes). Implementations must return a
// fresh slice per call; the adapter filters and sorts it in place. Defined
// here at the consumer per idiomatic Go; app.go adapts the concrete
// ProcessCollector to it.
type allProcessSource interface {
	allProcesses() []processRow
}

// allProcessSourceFunc adapts any func()[]processRow to allProcessSource.
type allProcessSourceFunc func() []processRow

func (f allProcessSourceFunc) allProcesses() []processRow { return f() }

// processKiller is the Processes tab's termination seam to the monitor layer.
// Defined here at the consumer; app.go adapts ProcessCollector.Terminate to it
// so the UI never touches gopsutil.
type processKiller interface {
	kill(pid PID) error
}

// processKillerFunc adapts any func(PID) error to processKiller.
type processKillerFunc func(pid PID) error

func (f processKillerFunc) kill(pid PID) error { return f(pid) }

// procSortColumn identifies which process-table column orders the rows.
type procSortColumn int

const (
	sortByPID procSortColumn = iota
	sortByName
	sortByUser
	sortByCPU
	sortByMem
	sortByStatus
)

// sortDirection is the order applied to the active sort column.
type sortDirection int

const (
	sortAscending sortDirection = iota
	sortDescending
)

// procColumnSpec pairs one all-processes column declaration with its sort
// key. procTableSpec is the single authoritative home of the table's column
// order: newAllProcessTable builds its columns from it and the view's
// tap-to-sort and sort-marker logic read it, so the three can't drift.
type procColumnSpec struct {
	column tableColumn
	sort   procSortColumn
}

// procTableSpec returns the all-processes table's columns in table order.
func procTableSpec() []procColumnSpec {
	identity := processIdentityColumns()
	return []procColumnSpec{
		{column: identity[0], sort: sortByPID},
		{column: identity[1], sort: sortByName},
		{column: identity[2], sort: sortByUser},
		{column: tableColumn{header: colHeaderCPU, width: procColCPUW, align: fyne.TextAlignTrailing}, sort: sortByCPU},
		{column: tableColumn{header: colHeaderMem, width: procColMemW, align: fyne.TextAlignTrailing}, sort: sortByMem},
		{column: tableColumn{header: colHeaderStatus, width: procColStatusW, kind: columnPill}, sort: sortByStatus},
	}
}

// defaultSortDirection is the direction a column sorts on first tap: usage
// columns show hottest-first, identity columns read naturally ascending.
func defaultSortDirection(col procSortColumn) sortDirection {
	if col == sortByCPU || col == sortByMem {
		return sortDescending
	}
	return sortAscending
}

// oppositeDirection flips a sort direction (second tap on the same column).
func oppositeDirection(d sortDirection) sortDirection {
	if d == sortAscending {
		return sortDescending
	}
	return sortAscending
}

// allProcessTableSource implements TableSource over the full process list,
// applying the live name filter, the active sort, and the row selection on
// every Snapshot — so all three survive each poll tick by construction.
type allProcessTableSource struct {
	src allProcessSource

	sortCol procSortColumn
	sortDir sortDirection

	// The toolbar's three filters: free text over name/user/PID, an exact
	// (short) username, and a status. Empty string means "no filter" for all
	// three — the wireframe's "all" / "any" options.
	filter       string
	userFilter   string
	statusFilter procStatus

	// Page-head readout and filter options, tallied from the UNFILTERED list
	// each Snapshot so they describe the whole machine.
	total     int
	highUsage int
	users     []string

	// Selection is tracked by PID (the first-class identifier), then
	// re-resolved to a row index against each snapshot. hasSelected
	// disambiguates "none" — PID 0 is a real pseudo-process on Windows.
	selected    PID
	hasSelected bool
	rowPIDs     []PID // PID per row of the last snapshot (tap → PID mapping)
	selRow      int   // selected PID's row in the last snapshot; noTableRow when none
}

// newAllProcessTableSource builds the adapter with the card's default order:
// CPU% descending.
func newAllProcessTableSource(src allProcessSource) *allProcessTableSource {
	return &allProcessTableSource{
		src:     src,
		sortCol: sortByCPU,
		sortDir: sortDescending,
		selRow:  noTableRow,
	}
}

// Snapshot returns the filtered, sorted process list as table cells, fresh on
// every Refresh.
func (s *allProcessTableSource) Snapshot() [][]tableCell {
	all := s.src.allProcesses()
	s.tally(all)
	rows := filterRows(all, s.filter, s.userFilter, s.statusFilter)
	sortRows(rows, s.sortCol, s.sortDir)
	s.indexRows(rows)

	cells := make([][]tableCell, len(rows))
	for i, r := range rows {
		cells[i] = append(processIdentityCells(r),
			tableCell{text: formatPercent1(r.cpu)},
			tableCell{text: formatBytesShort(r.mem)},
			tableCell{text: string(r.status), pill: statusPillKind(r.status)},
		)
	}
	return cells
}

// highlightedRow implements tableRowHighlighter: the selected row's index in
// the last snapshot.
func (s *allProcessTableSource) highlightedRow() int { return s.selRow }

// tally caches the page-head readout and the user-filter options from the
// unfiltered list, so both describe the whole machine regardless of the
// active filters.
func (s *allProcessTableSource) tally(rows []processRow) {
	s.total = len(rows)
	s.highUsage = 0
	seen := make(map[string]struct{})
	users := s.users[:0]
	for _, r := range rows {
		if r.cpu >= highUsageCPUPct {
			s.highUsage++
		}
		u := shortUsername(r.user)
		if u == "" {
			continue // permission-restricted rows have no name to filter on
		}
		if _, ok := seen[u]; !ok {
			seen[u] = struct{}{}
			users = append(users, u)
		}
	}
	sort.Strings(users)
	s.users = users
}

// counts reports the page-head readout: total processes and how many are at
// high CPU usage, as of the last Snapshot.
func (s *allProcessTableSource) counts() (total, highUsage int) {
	return s.total, s.highUsage
}

// userOptions returns the distinct (short) usernames seen in the last
// Snapshot, sorted — the "user:" filter's choices. The slice is a copy.
func (s *allProcessTableSource) userOptions() []string {
	out := make([]string, len(s.users))
	copy(out, s.users)
	return out
}

// filterRows applies the toolbar's three filters: free text matched against
// name, user, and PID (case-insensitive contains — the wireframe's "filter by
// name, user, pid…"), plus the exact user and status selections. Empty values
// pass everything through.
func filterRows(rows []processRow, text, user string, st procStatus) []processRow {
	if text == "" && user == "" && st == "" {
		return rows
	}
	needle := strings.ToLower(text)
	out := make([]processRow, 0, len(rows))
	for _, r := range rows {
		if user != "" && shortUsername(r.user) != user {
			continue
		}
		if st != "" && r.status != st {
			continue
		}
		if needle != "" && !matchesText(r, needle) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// matchesText reports whether the row matches the free-text needle on any of
// name, user, or PID.
func matchesText(r processRow, needle string) bool {
	return strings.Contains(strings.ToLower(r.name), needle) ||
		strings.Contains(strings.ToLower(shortUsername(r.user)), needle) ||
		strings.Contains(strconv.Itoa(int(r.pid)), needle)
}

// sortRows orders rows by the given column and direction, breaking ties by
// ascending PID so equal-valued rows hold a stable order between ticks
// (process enumeration order is not guaranteed stable).
func sortRows(rows []processRow, col procSortColumn, dir sortDirection) {
	less := rowLess(col)
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if dir == sortDescending {
			a, b = b, a
		}
		switch {
		case less(a, b):
			return true
		case less(b, a):
			return false
		default:
			return rows[i].pid < rows[j].pid
		}
	})
}

// rowLess returns the ascending comparison for one sortable column. Name and
// user compare case-insensitively so capitalization doesn't split the order.
func rowLess(col procSortColumn) func(a, b processRow) bool {
	switch col {
	case sortByName:
		return func(a, b processRow) bool { return strings.ToLower(a.name) < strings.ToLower(b.name) }
	case sortByUser:
		return func(a, b processRow) bool { return strings.ToLower(a.user) < strings.ToLower(b.user) }
	case sortByCPU:
		return func(a, b processRow) bool { return a.cpu < b.cpu }
	case sortByMem:
		return func(a, b processRow) bool { return a.mem < b.mem }
	case sortByStatus:
		return func(a, b processRow) bool { return a.status < b.status }
	default:
		return func(a, b processRow) bool { return a.pid < b.pid }
	}
}

// indexRows records each row's PID and re-resolves the selection against the
// new row set. A selected process that disappeared — exited or filtered out —
// clears the selection, so a later kill can never hit a recycled PID.
func (s *allProcessTableSource) indexRows(rows []processRow) {
	s.rowPIDs = s.rowPIDs[:0]
	for _, r := range rows {
		s.rowPIDs = append(s.rowPIDs, r.pid)
	}

	s.selRow = noTableRow
	if !s.hasSelected {
		return
	}
	if i := s.rowIndexOf(s.selected); i != noTableRow {
		s.selRow = i
		return
	}
	s.selected, s.hasSelected = 0, false
}

// setFilter sets the live free-text filter; the next Snapshot applies it.
func (s *allProcessTableSource) setFilter(f string) { s.filter = f }

// setUserFilter restricts rows to one (short) username; empty shows all.
func (s *allProcessTableSource) setUserFilter(u string) { s.userFilter = u }

// setStatusFilter restricts rows to one status; empty shows any.
func (s *allProcessTableSource) setStatusFilter(st procStatus) { s.statusFilter = st }

// toggleSort makes col the active sort column at its default direction, or
// flips the direction when col already is active.
func (s *allProcessTableSource) toggleSort(col procSortColumn) {
	if s.sortCol == col {
		s.sortDir = oppositeDirection(s.sortDir)
		return
	}
	s.sortCol = col
	s.sortDir = defaultSortDirection(col)
}

// selectRow selects the process at row i of the last snapshot.
func (s *allProcessTableSource) selectRow(i int) {
	if i < 0 || i >= len(s.rowPIDs) {
		return
	}
	s.selected, s.hasSelected = s.rowPIDs[i], true
	s.selRow = i
}

// selectPID selects the given PID. Its row index resolves on the next
// Snapshot (refresh the table before reading rowIndexOf).
func (s *allProcessTableSource) selectPID(pid PID) {
	s.selected, s.hasSelected = pid, true
}

// selectedPID returns the selected PID, false when nothing is selected.
func (s *allProcessTableSource) selectedPID() (PID, bool) {
	return s.selected, s.hasSelected
}

// rowIndexOf returns pid's row in the last snapshot, noTableRow when absent.
func (s *allProcessTableSource) rowIndexOf(pid PID) int {
	for i, p := range s.rowPIDs {
		if p == pid {
			return i
		}
	}
	return noTableRow
}

// newAllProcessTable builds the Processes tab's full table: the card's five
// columns fed by src, scroll-hosted (sizeToRows + the viewport pool) with
// tap-to-sort headers and tap-to-select rows.
func newAllProcessTable(src allProcessSource, onRowTap, onHeaderTap func(int)) (*dataTable, *allProcessTableSource) {
	adapter := newAllProcessTableSource(src)
	spec := procTableSpec()
	cols := make([]tableColumn, len(spec))
	for i, s := range spec {
		cols[i] = s.column
	}
	table := newDataTable(adapter,
		tableColumns(cols...),
		rowHeight(processTableRowHeight),
		rowPool(procTableRowPool),
		sizeToRows(),
		onRowTapped(onRowTap),
		onHeaderTapped(onHeaderTap),
	)
	return table, adapter
}
