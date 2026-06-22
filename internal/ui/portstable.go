package ui

// Ports table model — wires listening-port data into the generic dataTable
// widget, the same pull-on-render pattern the process tables use.
//
// portsTableModel is the single TableSource behind the Ports tab table. It pulls
// the full listening-port list from one allPortsSource and shapes it per tick:
// tally the unfiltered count (page-head readout), apply the toolbar's text and
// protocol filters, sort by port ascending (the wireframe's default order), then
// emit one tableCell per declarative column. Each row's owning PID is recorded in
// display order so a tapped row resolves to its process — the seam BZS253-60
// wires its "jump to process" navigation onto.
//
// allPortsSource is the seam to the monitor layer: it is consumed here and
// implemented in app.go (the composition root), the only place that knows both
// monitor.PortInfo and portRow — and the only place that resolves a port's PID to
// a process name by cross-referencing the existing process snapshot (no separate
// gopsutil call). The generic dataTable never sees portRow.

import (
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

// Column header labels for the ports table.
const (
	colHeaderProto = "Proto"
	colHeaderPort  = "Port"
	colHeaderState = "State"
	colHeaderOwner = "Owning process"
	colHeaderAddr  = "Local address"
)

// Ports table column widths (px), from the tab-07 wireframe. The owning-process
// column flexes to absorb the pane's extra width; the rest are fixed off-grid
// component dimensions, not spacing-scale steps.
const (
	portColProtoW = 70  // px; protocol pill
	portColPortW  = 80  // px; port number
	portColStateW = 90  // px; LISTEN state pill
	portColOwnerW = 200 // px; owning process name
	portColAddrW  = 210 // px; "ip:port" local address (wide enough for IPv6)
	portColLinkW  = 160 // px; trailing "process →" cross-nav link (flex)
	// The owning-PID column reuses procColPIDW — the same wireframe PID-column
	// width the process tables use (it also shares colHeaderPID). Only the
	// alignment differs: trailing here (numeric), leading in the process tables.
)

// portsTableRowPool sizes the table's renderer pool: comfortably more rows than
// the tallest realistic viewport shows at once, matching procTableRowPool's
// rationale (a full-height 4K pane is ~45 rows at 29px, plus windowing overdraw).
const portsTableRowPool = 48

// labelListenState is the state every listening port reports. PortInfo carries
// only sockets that are listening (TCP in LISTEN, peerless UDP), so the column is
// uniformly this — shown to match the wireframe's State column.
const labelListenState = "LISTEN"

// labelNoProcess fills the owning-process column when the PID could not be
// resolved to a name (permission-restricted or already exited) — a dash rather
// than a blank cell, so an unresolved owner reads as "unknown", not "missing".
const labelNoProcess = "—"

// labelJumpSuffix is the trailing arrow on the cross-nav "process →" link cell
// (the wireframe's jump-to-process affordance). The link is display-only for now;
// BZS253-60 wires the navigation onto the row tap.
const labelJumpSuffix = " →"

// portProto is the display vocabulary for a port's transport protocol — the
// ui-side mirror of monitor.Protocol (app.go converts), kept here so the model
// stays free of cross-layer imports.
type portProto string

const (
	portProtoTCP portProto = "TCP"
	portProtoUDP portProto = "UDP"
)

// protoFilter selects which protocols the table shows — the toolbar's
// All/TCP/UDP segmented control. Ordered to match the segment indices in ports.go.
type protoFilter int

const (
	protoFilterAll protoFilter = iota
	protoFilterTCP
	protoFilterUDP
)

// matches reports whether a row's protocol passes the active protocol filter.
func (f protoFilter) matches(p portProto) bool {
	switch f {
	case protoFilterTCP:
		return p == portProtoTCP
	case protoFilterUDP:
		return p == portProtoUDP
	default:
		return true
	}
}

// portRow is the display-layer shape for one listening port. app.go converts
// from monitor.PortInfo when building the model, resolving process from the
// owning PID, so this type never appears in the monitor package.
type portRow struct {
	proto     portProto
	port      uint32
	process   string // resolved owning-process name; "" when unresolvable
	pid       PID
	localAddr string // "ip:port"
}

// allPortsSource is the ports table's data seam to the monitor layer: the full
// listening-port list, unordered (the model owns ordering). Implementations must
// return a fresh slice per call; the model filters and sorts it in place. Defined
// here at the consumer per idiomatic Go; app.go adapts the concrete
// ProcessCollector to it.
type allPortsSource interface {
	allPorts() []portRow
}

// allPortsSourceFunc adapts any func() []portRow to allPortsSource.
type allPortsSourceFunc func() []portRow

func (f allPortsSourceFunc) allPorts() []portRow { return f() }

// portsColumns is the Ports tab's column set, in the wireframe's order: a Proto
// pill, Port (the default-sort column, marked ascending), a LISTEN State pill,
// the owning process name, PID, local address, then a trailing accent "process →"
// cross-nav link. The link column flexes (absorbs the pane's extra width) so the
// fixed identity/address columns pack left next to the owner and the
// right-aligned link hugs the right edge — long addresses can't reach it. The
// Proto and State pills and the trailing link reuse the generic dataTable's
// pill/colored-text column kinds.
func portsColumns() []tableColumn {
	return []tableColumn{
		{header: colHeaderProto, width: portColProtoW, kind: columnPill},
		{header: colHeaderPort + sortAscMarker, width: portColPortW, align: fyne.TextAlignTrailing},
		{header: colHeaderState, width: portColStateW, kind: columnPill},
		{header: colHeaderOwner, width: portColOwnerW, align: fyne.TextAlignLeading, color: palette.Text},
		{header: colHeaderPID, width: procColPIDW, align: fyne.TextAlignTrailing},
		{header: colHeaderAddr, width: portColAddrW, align: fyne.TextAlignLeading},
		{header: "", width: portColLinkW, align: fyne.TextAlignTrailing, color: palette.Accent, flex: true},
	}
}

// portsTableModel is the TableSource behind the Ports tab table. It holds the
// data seam, the toolbar filter state, the page-head tally, and the per-row PID
// bookkeeping for tap → process resolution.
type portsTableModel struct {
	src  allPortsSource
	cols []tableColumn

	filter string      // free-text filter (port number or process name)
	proto  protoFilter // protocol segment

	total   int   // listening-port count from the UNFILTERED list, for the page head
	rowPIDs []PID // owning PID per row of the last snapshot, in display order
}

// newPortsTableModel builds the model showing every protocol, unfiltered.
func newPortsTableModel(src allPortsSource) *portsTableModel {
	return &portsTableModel{src: src, cols: portsColumns()}
}

// Snapshot shapes the live port list into table cells, fresh on every Refresh:
// tally the unfiltered count → filter → sort by port ascending → record PIDs →
// one cell per column.
func (m *portsTableModel) Snapshot() [][]tableCell {
	rows := m.src.allPorts()
	m.total = len(rows)
	rows = m.filterRows(rows)
	sortPortRows(rows)
	m.recordPIDs(rows)

	cells := make([][]tableCell, len(rows))
	for i, r := range rows {
		cells[i] = []tableCell{
			{text: string(r.proto), pill: status.Neutral},
			{text: strconv.Itoa(int(r.port))},
			{text: labelListenState, pill: status.Healthy},
			{text: processNameOr(r.process)},
			{text: strconv.Itoa(int(r.pid))},
			{text: r.localAddr},
			{text: jumpLinkText(r.process)},
		}
	}
	return cells
}

// processNameOr returns the process name, or the unresolved-owner dash when it
// is empty.
func processNameOr(name string) string {
	if name == "" {
		return labelNoProcess
	}
	return name
}

// jumpLinkText renders the trailing cross-nav link cell — "<process> →" — or an
// empty cell when the owner is unresolved (nothing to jump to).
func jumpLinkText(name string) string {
	if name == "" {
		return ""
	}
	return name + labelJumpSuffix
}

// filterRows applies the toolbar's protocol segment and free-text filter (port
// number or process name, case-insensitive contains). Empty values pass
// everything through.
func (m *portsTableModel) filterRows(rows []portRow) []portRow {
	if m.proto == protoFilterAll && m.filter == "" {
		return rows
	}
	needle := strings.ToLower(m.filter)
	out := make([]portRow, 0, len(rows))
	for _, r := range rows {
		if !m.proto.matches(r.proto) {
			continue
		}
		if needle != "" && !portMatchesText(r, needle) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// portMatchesText reports whether a row matches the free-text needle on its port
// number or process name.
func portMatchesText(r portRow, needle string) bool {
	return strings.Contains(strconv.Itoa(int(r.port)), needle) ||
		strings.Contains(strings.ToLower(r.process), needle)
}

// sortPortRows orders rows by ascending port (the wireframe's default), breaking
// ties by protocol then PID so equal-port rows hold a stable order between ticks.
func sortPortRows(rows []portRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch {
		case a.port != b.port:
			return a.port < b.port
		case a.proto != b.proto:
			return a.proto < b.proto
		default:
			return a.pid < b.pid
		}
	})
}

// recordPIDs caches each row's owning PID in display order for tap → process
// resolution (BZS253-60's jump-to-process).
func (m *portsTableModel) recordPIDs(rows []portRow) {
	m.rowPIDs = m.rowPIDs[:0]
	for _, r := range rows {
		m.rowPIDs = append(m.rowPIDs, r.pid)
	}
}

// pidAt returns the owning PID at display row index, false when out of range
// (the row vanished between the snapshot and the tap).
func (m *portsTableModel) pidAt(index int) (PID, bool) {
	return pidAtRow(m.rowPIDs, index)
}

// listeningCount reports the total listening ports as of the last Snapshot — the
// page head's "N LISTENING" readout, tallied before filtering so it describes the
// whole machine.
func (m *portsTableModel) listeningCount() int { return m.total }

// setFilter sets the live free-text filter; the next Snapshot applies it.
func (m *portsTableModel) setFilter(f string) { m.filter = f }

// setProtoFilter restricts rows to one protocol segment; the next Snapshot
// applies it.
func (m *portsTableModel) setProtoFilter(f protoFilter) { m.proto = f }
