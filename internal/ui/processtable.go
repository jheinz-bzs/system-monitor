package ui

// Process table model — wires process data into the generic dataTable widget.
//
// All three process tables (the CPU and Memory tabs' top-N panels and the
// Processes tab's full sortable/filterable list) are one type, processTableModel,
// configured differently. The model pulls the full process list from a single
// allProcessSource and shapes it per its config: an optional filter pass, an
// optional top-N selection by a primary metric, the active display sort, and one
// tableCell per *declarative column* (each procColumn carries its own
// value-extractor). Adding or re-laying-out a table means declaring columns and
// flags, not writing a new Snapshot.
//
// allProcessSource and processKiller are the seams between the monitor layer and
// this model; they are consumed here and implemented in app.go (the composition
// root), the only place that knows both monitor.ProcessInfo and processRow. The
// generic dataTable widget (datatable.go) never sees processRow.

import (
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

// Process data constants.
const (
	topCPUProcessLimit    = 10 // CPU tab's top-N row count
	topMemProcessLimit    = 10 // Memory tab's top-N row count
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
	procColCheckW  = 36  // px; selection checkbox (tableCheckSize + both cell insets)
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
// ui-side mirror of monitor.ProcessState (app.go converts), kept separate so the
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

// PID is a typed process identifier, carried as a first-class value so
// cross-tab navigation links resolve without string parsing or type assertions.
type PID int32

// pidAtRow returns pids[index] guarded by bounds, false when index is out of
// range. Each process visualization records a PID per drawn element (table row
// or treemap block) in draw order, so a tapped element resolves to the process
// it represents through this one lookup.
func pidAtRow(pids []PID, index int) (PID, bool) {
	if index < 0 || index >= len(pids) {
		return 0, false
	}
	return pids[index], true
}

// processRow is the display-layer shape for one process. app.go selects and
// converts from monitor.ProcessInfo when building the model, so this type
// never appears in the monitor package.
type processRow struct {
	pid    PID
	name   string
	user   string
	cpu    float64 // 0..100, machine-wide scale
	mem    uint64  // resident set bytes
	status procStatus
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

// byteFraction returns part/whole as a 0..1 fraction, 0 when whole is zero
// (an unknown total must not divide by zero or fill a bar).
func byteFraction(part, whole uint64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}

// allProcessSource is every process table's data seam to the monitor layer: the
// complete process list, unordered (the model owns ordering — sort state must
// live UI-side to survive refreshes). Implementations must return a fresh slice
// per call; the model filters, selects, and sorts it in place. Defined here at
// the consumer per idiomatic Go; app.go adapts the concrete ProcessCollector to
// it.
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

// sortablePIDRows is the sort-state + per-row-PID bookkeeping the table model
// uses: the active sort column/direction (header-tap sorting) and the PID
// recorded per displayed row (tap → process resolution, for cross-nav or
// selection). The model embeds it; the header helpers read it.
type sortablePIDRows struct {
	sortCol procSortColumn
	sortDir sortDirection
	rowPIDs []PID // PID per row of the last snapshot, in display order
}

// applySort orders rows in place by the active column and direction.
func (s *sortablePIDRows) applySort(rows []processRow) {
	sortRows(rows, s.sortCol, s.sortDir)
}

// recordPIDs caches each row's PID in display order for tap → PID resolution.
func (s *sortablePIDRows) recordPIDs(rows []processRow) {
	s.rowPIDs = s.rowPIDs[:0]
	for _, r := range rows {
		s.rowPIDs = append(s.rowPIDs, r.pid)
	}
}

// pidAt returns the PID at display row index, false when out of range (the row
// vanished between the snapshot and the tap).
func (s *sortablePIDRows) pidAt(index int) (PID, bool) {
	return pidAtRow(s.rowPIDs, index)
}

// rowIndexOf returns pid's display row, noTableRow when absent.
func (s *sortablePIDRows) rowIndexOf(pid PID) int {
	for i, p := range s.rowPIDs {
		if p == pid {
			return i
		}
	}
	return noTableRow
}

// toggleSort makes col the active sort column at its default direction, or flips
// the direction when col already is active.
func (s *sortablePIDRows) toggleSort(col procSortColumn) {
	if s.sortCol == col {
		s.sortDir = oppositeDirection(s.sortDir)
		return
	}
	s.sortCol = col
	s.sortDir = defaultSortDirection(col)
}

// procColumn is one declarative table column: its rendering definition
// (embedded tableColumn), the sort key it taps to (when sortable), and a
// value-extractor that turns a processRow into the column's cell. The model
// formats every row by calling each column's cell, so the per-table differences
// (units, bars, pills) live entirely in these extractors — one generic Snapshot
// serves all three tables.
type procColumn struct {
	tableColumn
	sort     procSortColumn
	sortable bool
	cell     func(r processRow) tableCell
}

// columnDefs extracts the bare rendering declarations from a column set, in
// order — what newDataTable's tableColumns option wants.
func columnDefs(cols []procColumn) []tableColumn {
	out := make([]tableColumn, len(cols))
	for i, c := range cols {
		out[i] = c.tableColumn
	}
	return out
}

// Shared cell extractors. The identity trio plus CPU% and resident-memory text
// recur across tables, so they live as named functions; per-table columns that
// need table-level context (the Memory bars/percent need total) capture it in a
// closure built by memColumns.
func pidCell(r processRow) tableCell      { return tableCell{text: strconv.Itoa(int(r.pid))} }
func nameCell(r processRow) tableCell     { return tableCell{text: r.name} }
func userCell(r processRow) tableCell     { return tableCell{text: shortUsername(r.user)} }
func cpuPctCell(r processRow) tableCell   { return tableCell{text: formatPercent1(r.cpu)} }
func cpuBarCell(r processRow) tableCell   { return tableCell{frac: r.cpu / percentMax} }
func memBytesCell(r processRow) tableCell { return tableCell{text: formatBytesShort(r.mem)} }
func statusCell(r processRow) tableCell {
	return tableCell{text: string(r.status), pill: statusPillKind(r.status)}
}

// identityColumns returns the leading PID / Process / User columns every process
// table starts with, each sortable by its natural key.
func identityColumns() []procColumn {
	cols := processIdentityColumns()
	return []procColumn{
		{tableColumn: cols[0], sort: sortByPID, sortable: true, cell: pidCell},
		{tableColumn: cols[1], sort: sortByName, sortable: true, cell: nameCell},
		{tableColumn: cols[2], sort: sortByUser, sortable: true, cell: userCell},
	}
}

// cpuColumns is the CPU tab's column set: identity, CPU% with its inline bar
// gauge, then resident memory.
func cpuColumns() []procColumn {
	return append(identityColumns(),
		procColumn{tableColumn: tableColumn{header: colHeaderCPU, width: procColCPUW, align: fyne.TextAlignTrailing}, sort: sortByCPU, sortable: true, cell: cpuPctCell},
		procColumn{tableColumn: tableColumn{header: "", width: procColBarW, kind: columnBar}, cell: cpuBarCell},
		procColumn{tableColumn: tableColumn{header: colHeaderMem, width: procColMemW, align: fyne.TextAlignTrailing}, sort: sortByMem, sortable: true, cell: memBytesCell},
	)
}

// memColumns is the Memory tab's column set: identity, RSS with its inline bar
// gauge, then Mem%. total scales the bar (full at memBarFullScalePct of total)
// and the percentage, captured by the closures. Mem% is not separately
// sortable — its order is identical to RSS, so RSS owns the memory sort.
func memColumns(total uint64) []procColumn {
	memPct := func(r processRow) float64 { return byteFraction(r.mem, total) * percentMax }
	return append(identityColumns(),
		procColumn{tableColumn: tableColumn{header: colHeaderRSS, width: procColMemW, align: fyne.TextAlignTrailing}, sort: sortByMem, sortable: true, cell: memBytesCell},
		procColumn{tableColumn: tableColumn{header: "", width: procColBarW, kind: columnBar},
			cell: func(r processRow) tableCell { return tableCell{frac: min(memPct(r)/memBarFullScalePct, 1)} }},
		procColumn{tableColumn: tableColumn{header: colHeaderPctMem, width: procColPctW, align: fyne.TextAlignTrailing},
			cell: func(r processRow) tableCell { return tableCell{text: formatPercent1(memPct(r))} }},
	)
}

// allColumns is the Processes tab's full column set: the selection checkbox
// (its cells read the model's live selection through sel), identity, CPU%,
// Mem, and the status pill.
func allColumns(sel func(PID) bool) []procColumn {
	check := procColumn{
		tableColumn: tableColumn{width: procColCheckW, kind: columnCheck},
		cell:        func(r processRow) tableCell { return tableCell{checked: sel(r.pid)} },
	}
	return append(append([]procColumn{check}, identityColumns()...),
		procColumn{tableColumn: tableColumn{header: colHeaderCPU, width: procColCPUW, align: fyne.TextAlignTrailing}, sort: sortByCPU, sortable: true, cell: cpuPctCell},
		procColumn{tableColumn: tableColumn{header: colHeaderMem, width: procColMemW, align: fyne.TextAlignTrailing}, sort: sortByMem, sortable: true, cell: memBytesCell},
		procColumn{tableColumn: tableColumn{header: colHeaderStatus, width: procColStatusW, kind: columnPill}, sort: sortByStatus, sortable: true, cell: statusCell},
	)
}

// memBarFullScalePct is the Mem% value at which a memory-table bar fills its
// whole track, measured from the wireframe's fills (its 5.6%-of-total row
// fills 56% of the track). Percentage points, not a fraction.
const memBarFullScalePct = 10

// tapSortHeader handles a header tap: when the tapped column is sortable it
// becomes (or flips) the active sort, re-tags the markers, and repaints.
// Non-sortable columns (bar, derived) ignore the tap. Shared by all three
// process tables so header sorting behaves identically.
func tapSortHeader(table *dataTable, m *processTableModel, col int) {
	if col < 0 || col >= len(m.cols) || !m.cols[col].sortable {
		return
	}
	m.toggleSort(m.cols[col].sort)
	syncSortHeaders(table, m)
	table.Refresh()
}

// syncSortHeaders rewrites the column headers so the active sort column — and
// only it — carries its direction marker.
func syncSortHeaders(table *dataTable, m *processTableModel) {
	for i, c := range m.cols {
		header := c.header
		if c.sortable && c.sort == m.sortCol {
			header += sortMarker(m.sortDir)
		}
		table.setColumnHeader(i, header)
	}
}

// processTableModel is the single TableSource behind every process table. It
// pulls the full process list from one allProcessSource and shapes it per its
// configuration: an optional filter pass (Processes tab), an optional top-N
// selection by a primary metric (CPU/Memory tabs), the active display sort, and
// one tableCell per declarative column. Optional selection state (Processes tab)
// is re-resolved against each snapshot by PID. Embeds sortablePIDRows for the
// sort + PID bookkeeping shared with header taps and cross-nav.
type processTableModel struct {
	sortablePIDRows
	src  allProcessSource
	cols []procColumn

	// Top-N selection: limit 0 means the full list; metric is the column the
	// top-N is chosen by, before the display sort reorders that set.
	limit  int
	metric procSortColumn

	// Filtering (Processes tab only; filterable false disables the pass). Empty
	// string means "no filter" for each — the wireframe's "all" / "any" options.
	filterable   bool
	filter       string
	userFilter   string
	statusFilter procStatus

	// Page-head readout and user-filter options, tallied from the UNFILTERED
	// list each Snapshot so they describe the whole machine.
	total     int
	highUsage int
	users     []string

	// Selection (Processes tab only; selectable false disables it), tracked as
	// a set of PIDs — the first-class identifier — and re-resolved against each
	// snapshot: a selected process that disappears (exited or filtered out) is
	// pruned, so a mass kill can never hit a recycled PID. A set (not a single
	// PID) because the check column multi-selects for the mass-kill action.
	selectable bool
	selected   map[PID]struct{}
}

// newCPUTableSource builds the CPU tab's model: top-N by CPU, CPU% descending,
// no filter or selection (rows cross-navigate instead).
func newCPUTableSource(src allProcessSource) *processTableModel {
	return &processTableModel{
		sortablePIDRows: sortablePIDRows{sortCol: sortByCPU, sortDir: sortDescending},
		src:             src,
		cols:            cpuColumns(),
		limit:           topCPUProcessLimit,
		metric:          sortByCPU,
	}
}

// newMemTableSource builds the Memory tab's model: top-N by resident memory,
// RSS descending, with total scaling the Mem% column.
func newMemTableSource(src allProcessSource, total uint64) *processTableModel {
	return &processTableModel{
		sortablePIDRows: sortablePIDRows{sortCol: sortByMem, sortDir: sortDescending},
		src:             src,
		cols:            memColumns(total),
		limit:           topMemProcessLimit,
		metric:          sortByMem,
	}
}

// newAllProcessTableSource builds the Processes tab's model: the full list,
// filterable and selectable, CPU% descending by default. The check column's
// cells read the model's own selection set, so cols is wired after construction.
func newAllProcessTableSource(src allProcessSource) *processTableModel {
	m := &processTableModel{
		sortablePIDRows: sortablePIDRows{sortCol: sortByCPU, sortDir: sortDescending},
		src:             src,
		filterable:      true,
		selectable:      true,
		selected:        map[PID]struct{}{},
	}
	m.cols = allColumns(m.isSelected)
	return m
}

// Snapshot shapes the live process list into table cells, fresh on every
// Refresh: filter (if enabled) → top-N select (if limited) → display sort →
// record PIDs and re-resolve selection → one cell per column.
func (m *processTableModel) Snapshot() [][]tableCell {
	rows := m.src.allProcesses()
	if m.filterable {
		m.tally(rows)
		rows = filterRows(rows, m.filter, m.userFilter, m.statusFilter)
	}
	if m.limit > 0 {
		rows = topN(rows, m.metric, m.limit)
	}
	m.applySort(rows)
	if m.selectable {
		m.indexRows(rows)
	} else {
		m.recordPIDs(rows)
	}

	cells := make([][]tableCell, len(rows))
	for i, r := range rows {
		row := make([]tableCell, len(m.cols))
		for j, c := range m.cols {
			row[j] = c.cell(r)
		}
		cells[i] = row
	}
	return cells
}

// topN selects the n highest rows by metric (descending), then returns them for
// the caller to reorder by the active display sort.
func topN(rows []processRow, metric procSortColumn, n int) []processRow {
	sortRows(rows, metric, sortDescending)
	if n < len(rows) {
		rows = rows[:n]
	}
	return rows
}

// tally caches the page-head readout and the user-filter options from the
// unfiltered list, so both describe the whole machine regardless of the
// active filters.
func (m *processTableModel) tally(rows []processRow) {
	m.total = len(rows)
	m.highUsage = 0
	seen := make(map[string]struct{})
	users := m.users[:0]
	for _, r := range rows {
		if r.cpu >= highUsageCPUPct {
			m.highUsage++
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
	m.users = users
}

// counts reports the page-head readout: total processes and how many are at
// high CPU usage, as of the last Snapshot.
func (m *processTableModel) counts() (total, highUsage int) {
	return m.total, m.highUsage
}

// userOptions returns the distinct (short) usernames seen in the last
// Snapshot, sorted — the "user:" filter's choices. The slice is a copy.
func (m *processTableModel) userOptions() []string {
	out := make([]string, len(m.users))
	copy(out, m.users)
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

// indexRows records each row's PID and prunes the selection to PIDs still in
// the new row set. A selected process that disappeared — exited or filtered
// out — drops out, so a later mass kill can never hit a recycled PID.
func (m *processTableModel) indexRows(rows []processRow) {
	m.recordPIDs(rows)

	if len(m.selected) == 0 {
		return
	}
	present := make(map[PID]struct{}, len(m.rowPIDs))
	for _, pid := range m.rowPIDs {
		present[pid] = struct{}{}
	}
	for pid := range m.selected {
		if _, ok := present[pid]; !ok {
			delete(m.selected, pid)
		}
	}
}

// setFilter sets the live free-text filter; the next Snapshot applies it.
func (m *processTableModel) setFilter(f string) { m.filter = f }

// setUserFilter restricts rows to one (short) username; empty shows all.
func (m *processTableModel) setUserFilter(u string) { m.userFilter = u }

// setStatusFilter restricts rows to one status; empty shows any.
func (m *processTableModel) setStatusFilter(st procStatus) { m.statusFilter = st }

// isSelected reports whether pid is in the selection — the check column's
// cell state.
func (m *processTableModel) isSelected(pid PID) bool {
	_, ok := m.selected[pid]
	return ok
}

// toggleRow adds the process at row i of the last snapshot to the selection,
// or removes it when already selected — the whole row is the checkbox's
// hit-target.
func (m *processTableModel) toggleRow(i int) {
	pid, ok := m.pidAt(i)
	if !ok {
		return
	}
	if m.isSelected(pid) {
		delete(m.selected, pid)
		return
	}
	m.selected[pid] = struct{}{}
}

// clearSelection drops the whole selection.
func (m *processTableModel) clearSelection() {
	clear(m.selected)
}

// allVisibleSelected reports whether every row of the last snapshot is
// selected — the header checkbox's state (false when there are no rows).
func (m *processTableModel) allVisibleSelected() bool {
	return len(m.rowPIDs) > 0 && len(m.selected) == len(m.rowPIDs)
}

// toggleSelectAllVisible selects every row of the last snapshot, or clears the
// selection when all of them already are — the header checkbox's tap. The
// selection only ever holds visible PIDs (indexRows prunes), so the length
// comparison suffices.
func (m *processTableModel) toggleSelectAllVisible() {
	if m.allVisibleSelected() {
		m.clearSelection()
		return
	}
	for _, pid := range m.rowPIDs {
		m.selected[pid] = struct{}{}
	}
}

// selectPID makes pid the sole selection — cross-tab navigation lands on
// exactly the jumped-to process. Its row index resolves on the next Snapshot
// (refresh the table before reading rowIndexOf).
func (m *processTableModel) selectPID(pid PID) {
	clear(m.selected)
	m.selected[pid] = struct{}{}
}

// selectedPIDs returns the selected PIDs in display order (the kill loop's
// input; display order keeps it deterministic).
func (m *processTableModel) selectedPIDs() []PID {
	out := make([]PID, 0, len(m.selected))
	for _, pid := range m.rowPIDs {
		if !m.isSelected(pid) {
			continue
		}
		out = append(out, pid)
	}
	return out
}

// selectionCount reports how many processes are selected (the Kill label).
func (m *processTableModel) selectionCount() int { return len(m.selected) }

// newCPUProcessTable builds the CPU tab's top-processes table, sized to its rows
// so a short pane scrolls. onRowTap fires with the tapped data-row index (the
// CPU view resolves it to a PID via pidAt and jumps to the Processes tab);
// onHeaderTap drives header sorting. The returned model backs both.
func newCPUProcessTable(src allProcessSource, onRowTap, onHeaderTap func(int)) (*dataTable, *processTableModel) {
	m := newCPUTableSource(src)
	table := newDataTable(m,
		tableColumns(columnDefs(m.cols)...),
		rowHeight(processTableRowHeight),
		sizeToRows(),
		onRowTapped(onRowTap),
		onHeaderTapped(onHeaderTap),
	)
	return table, m
}

// newMemProcessTable builds the Memory tab's top-processes table, with total
// physical memory scaling the %Mem column. Sized to its rows so a short pane
// scrolls. onRowTap resolves to a PID for cross-nav; onHeaderTap drives header
// sorting. The returned model backs both.
func newMemProcessTable(src allProcessSource, total uint64, onRowTap, onHeaderTap func(int)) (*dataTable, *processTableModel) {
	m := newMemTableSource(src, total)
	table := newDataTable(m,
		tableColumns(columnDefs(m.cols)...),
		rowHeight(processTableRowHeight),
		sizeToRows(),
		onRowTapped(onRowTap),
		onHeaderTapped(onHeaderTap),
	)
	return table, m
}

// newAllProcessTable builds the Processes tab's full table: scroll-hosted
// (sizeToRows + the viewport pool) with tap-to-sort headers and tap-to-select
// rows.
func newAllProcessTable(src allProcessSource, onRowTap, onHeaderTap func(int)) (*dataTable, *processTableModel) {
	m := newAllProcessTableSource(src)
	table := newDataTable(m,
		tableColumns(columnDefs(m.cols)...),
		rowHeight(processTableRowHeight),
		rowPool(procTableRowPool),
		sizeToRows(),
		onRowTapped(onRowTap),
		onHeaderTapped(onHeaderTap),
	)
	return table, m
}

// treemapMetric selects which resource the Processes tab's dominance treemap
// sizes its blocks by — the metric behind the panel header's CPU/Mem toggle.
type treemapMetric int

const (
	treemapMetricCPU treemapMetric = iota
	treemapMetricMem
)

// processTreemapSource implements treemapSource over the full process list: it
// sizes the top treemapBlockLimit processes by the active metric (CPU% or
// resident memory), largest first, and colors them from the categorical palette
// so neighboring blocks stay distinguishable. setMetric switches the metric
// live; the next treemapBlocks reflects it (the toggle never resets the table's
// own sort, which lives in the processTableModel). Reusing sortRows keeps the
// ordering identical to the table's column sorts.
type processTreemapSource struct {
	src    allProcessSource
	metric treemapMetric
	pids   []PID // PID per block of the last treemapBlocks, for tap → PID mapping
}

// newProcessTreemapSource builds the adapter sizing by CPU% — the dominance
// map's default, matching the table's default order.
func newProcessTreemapSource(src allProcessSource) *processTreemapSource {
	return &processTreemapSource{src: src, metric: treemapMetricCPU}
}

// setMetric switches the sizing metric; the next treemapBlocks applies it.
func (s *processTreemapSource) setMetric(m treemapMetric) { s.metric = m }

// treemapBlocks returns the largest-N processes as treemap items, ordered
// largest-first (the squarified layout wants descending weights). A process
// with zero weight contributes no block; since the list is sorted descending,
// the first zero ends the run.
func (s *processTreemapSource) treemapBlocks() []treemapItem {
	rows := s.src.allProcesses()
	sortRows(rows, s.sortColumn(), sortDescending)
	if len(rows) > treemapBlockLimit {
		rows = rows[:treemapBlockLimit]
	}
	items := make([]treemapItem, 0, len(rows))
	s.pids = s.pids[:0]
	for i, r := range rows {
		weight := s.weight(r)
		if weight <= 0 {
			break
		}
		items = append(items, treemapItem{
			label:   r.name,
			weight:  weight,
			fill:    palette.Series[i%len(palette.Series)],
			tooltip: r.name, // hover shows the full name when the tile truncated it
		})
		s.pids = append(s.pids, r.pid)
	}
	return items
}

// pidAt returns the PID of the block at index in the last treemapBlocks result,
// false when index is out of range. The treemap records hit regions and this
// adapter records pids in the same pass, so a tapped block's index resolves to
// the process it represents.
func (s *processTreemapSource) pidAt(index int) (PID, bool) {
	return pidAtRow(s.pids, index)
}

// sortColumn maps the active metric onto the table's matching sort column, so
// the treemap and a same-metric table sort agree on order.
func (s *processTreemapSource) sortColumn() procSortColumn {
	if s.metric == treemapMetricMem {
		return sortByMem
	}
	return sortByCPU
}

// weight returns a row's size under the active metric: CPU percent or resident
// bytes, both as a positive relative weight.
func (s *processTreemapSource) weight(r processRow) float64 {
	if s.metric == treemapMetricMem {
		return float64(r.mem)
	}
	return r.cpu
}
