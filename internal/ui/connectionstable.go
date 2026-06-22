package ui

// Connections table model — wires the active TCP/UDP connection list into the
// generic dataTable widget, the same pull-on-render pattern the Ports and process
// tables use (BZS253-61).
//
// connsTableModel is the single TableSource behind the Connections tab table. It
// pulls the full connection list from one allConnsSource and shapes it per tick:
// tally the page-head counts (active total + established), apply the toolbar's
// text and protocol filters, sort for a stable order, then emit one tableCell per
// declarative column. Each row's owning PID is recorded in display order so a
// tapped row resolves to its process — reusing the Ports tab's jump-to-process
// seam.
//
// allConnsSource is the seam to the monitor layer: consumed here, implemented in
// app.go (the composition root), the only place that knows both
// monitor.ConnectionInfo and connRow and resolves a PID to a process name from
// the existing process snapshot (no separate gopsutil call). It mirrors the Ports
// tab's allPortsSource and reuses portProto/protoFilter unchanged.

import (
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

// Column header labels unique to the connections table. Proto/State/PID and the
// address column reuse the ports table's headers and widths (same vocabulary).
const (
	colHeaderLocalAddr  = "Local address"
	colHeaderRemoteAddr = "Remote address"
)

// connColStateW is the State pill column width (px). Wider than the ports State
// column because connection states ("ESTABLISHED") are longer than "LISTEN".
const connColStateW = 110

// Page-head readout labels: the live "N ACTIVE  M ESTABLISHED" counts.
const (
	labelConnsActive      = "ACTIVE"
	labelConnsEstablished = "ESTABLISHED"
)

// labelConnsFilter is the toolbar's free-text filter placeholder (wireframe).
const labelConnsFilter = "filter by address, process, pid…"

// connState is the display vocabulary for a connection's TCP/UDP state — the
// ui-side mirror of monitor.ConnState (app.go converts), kept here so the model
// stays free of cross-layer imports. The set is open (gopsutil reports many
// states); the consts below are only the ones the pill coloring distinguishes.
type connState string

const (
	connStateEstablished connState = "ESTABLISHED"
	connStateListen      connState = "LISTEN"
	connStateTimeWait    connState = "TIME_WAIT"
	connStateCloseWait   connState = "CLOSE_WAIT"
	connStateClose       connState = "CLOSE"
	connStateClosed      connState = "CLOSED"
)

// pill maps a connection state to its status-pill color: established/listening
// are healthy (green), an explicitly closed socket is critical (red), any other
// non-empty transitional state is a warning (yellow), and an empty state (UDP,
// which has none) is neutral.
func (s connState) pill() statusKind {
	switch s {
	case connStateEstablished, connStateListen:
		return status.Healthy
	case connStateClose, connStateClosed:
		return status.Critical
	case "":
		return status.Neutral
	default:
		return status.Warning
	}
}

// display renders the state for the State column: the em-dash placeholder when
// empty (UDP), otherwise the state string itself.
func (s connState) display() string {
	if s == "" {
		return labelNoProcess
	}
	return string(s)
}

// connRow is the display-layer shape for one active connection. app.go converts
// from monitor.ConnectionInfo, resolving process from the owning PID, so this
// type never appears in the monitor package.
type connRow struct {
	proto      portProto
	localAddr  string // "ip:port"
	remoteAddr string // "ip:port", empty when unbound
	state      connState
	process    string // resolved owning-process name; "" when unresolvable
	pid        PID
}

// allConnsSource is the connections table's data seam to the monitor layer: the
// full connection list, unordered (the model owns ordering). Implementations must
// return a fresh slice per call; the model filters and sorts it in place. Defined
// here at the consumer per idiomatic Go; app.go adapts the ProcessCollector to it.
type allConnsSource interface {
	allConns() []connRow
}

// allConnsSourceFunc adapts any func() []connRow to allConnsSource.
type allConnsSourceFunc func() []connRow

func (f allConnsSourceFunc) allConns() []connRow { return f() }

// connsColumns is the Connections tab's column set, in the wireframe's order: a
// Proto pill, local and remote addresses, a State pill, the owning process name,
// PID, then a trailing accent "process →" cross-nav link that flexes to absorb
// the pane's extra width. Reuses the ports table's Proto/PID/owner/link widths
// and the address width for both address columns (same "ip:port" shape); State
// gets its own wider width (connColStateW) since connection states run longer.
func connsColumns() []tableColumn {
	return []tableColumn{
		{header: colHeaderProto, width: portColProtoW, kind: columnPill},
		{header: colHeaderLocalAddr, width: portColAddrW, align: fyne.TextAlignLeading},
		{header: colHeaderRemoteAddr, width: portColAddrW, align: fyne.TextAlignLeading},
		{header: colHeaderState, width: connColStateW, kind: columnPill},
		{header: colHeaderOwner, width: portColOwnerW, align: fyne.TextAlignLeading, color: palette.Text},
		{header: colHeaderPID, width: procColPIDW, align: fyne.TextAlignTrailing},
		{header: "", width: portColLinkW, align: fyne.TextAlignTrailing, color: palette.Accent, flex: true},
	}
}

// connsTableModel is the TableSource behind the Connections tab table. It holds
// the data seam, the toolbar filter state, the page-head tallies, and the per-row
// PID bookkeeping for tap → process resolution.
type connsTableModel struct {
	src  allConnsSource
	cols []tableColumn

	filter string      // free-text filter (address, process name, or PID)
	proto  protoFilter // protocol segment

	total       int   // connection count from the UNFILTERED list (page head "ACTIVE")
	established int   // ESTABLISHED count from the UNFILTERED list (page head)
	rowPIDs     []PID // owning PID per row of the last snapshot, in display order
}

// newConnsTableModel builds the model showing every protocol, unfiltered.
func newConnsTableModel(src allConnsSource) *connsTableModel {
	return &connsTableModel{src: src, cols: connsColumns()}
}

// Snapshot shapes the live connection list into table cells, fresh on every
// Refresh: tally the unfiltered counts → filter → sort → record PIDs → one cell
// per column.
func (m *connsTableModel) Snapshot() [][]tableCell {
	rows := m.src.allConns()
	m.tally(rows)
	rows = m.filterRows(rows)
	sortConnRows(rows)
	m.recordPIDs(rows)

	cells := make([][]tableCell, len(rows))
	for i, r := range rows {
		cells[i] = []tableCell{
			{text: string(r.proto), pill: status.Neutral},
			{text: r.localAddr},
			{text: r.remoteAddr},
			{text: r.state.display(), pill: r.state.pill()},
			{text: processNameOr(r.process)},
			{text: strconv.Itoa(int(r.pid))},
			{text: jumpLinkText(r.process)},
		}
	}
	return cells
}

// tally counts the unfiltered list for the page head: every connection is
// "active", and those in ESTABLISHED are counted separately.
func (m *connsTableModel) tally(rows []connRow) {
	m.total = len(rows)
	m.established = 0
	for _, r := range rows {
		if r.state == connStateEstablished {
			m.established++
		}
	}
}

// filterRows applies the toolbar's protocol segment and free-text filter (address,
// process name, or PID; case-insensitive contains). Empty values pass everything
// through.
func (m *connsTableModel) filterRows(rows []connRow) []connRow {
	if m.proto == protoFilterAll && m.filter == "" {
		return rows
	}
	needle := strings.ToLower(m.filter)
	out := make([]connRow, 0, len(rows))
	for _, r := range rows {
		if !m.proto.matches(r.proto) {
			continue
		}
		if needle != "" && !connMatchesText(r, needle) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// connMatchesText reports whether a row matches the free-text needle on either
// address, its process name, or its PID.
func connMatchesText(r connRow, needle string) bool {
	return strings.Contains(strings.ToLower(r.localAddr), needle) ||
		strings.Contains(strings.ToLower(r.remoteAddr), needle) ||
		strings.Contains(strings.ToLower(r.process), needle) ||
		strings.Contains(strconv.Itoa(int(r.pid)), needle)
}

// sortConnRows orders rows for a stable display across ticks: by protocol, then
// local address, remote address, and PID. The wireframe marks no sort column, so
// the order only needs to be deterministic, not user-facing.
func sortConnRows(rows []connRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch {
		case a.proto != b.proto:
			return a.proto < b.proto
		case a.localAddr != b.localAddr:
			return a.localAddr < b.localAddr
		case a.remoteAddr != b.remoteAddr:
			return a.remoteAddr < b.remoteAddr
		default:
			return a.pid < b.pid
		}
	})
}

// recordPIDs caches each row's owning PID in display order for tap → process
// resolution (the Ports tab's jump-to-process seam).
func (m *connsTableModel) recordPIDs(rows []connRow) {
	m.rowPIDs = m.rowPIDs[:0]
	for _, r := range rows {
		m.rowPIDs = append(m.rowPIDs, r.pid)
	}
}

// pidAt returns the owning PID at display row index, false when out of range
// (the row vanished between the snapshot and the tap).
func (m *connsTableModel) pidAt(index int) (PID, bool) {
	return pidAtRow(m.rowPIDs, index)
}

// activeCount reports the total connections as of the last Snapshot — the page
// head's "N ACTIVE" readout, tallied before filtering.
func (m *connsTableModel) activeCount() int { return m.total }

// establishedCount reports the ESTABLISHED connections as of the last Snapshot —
// the page head's "M ESTABLISHED" readout, tallied before filtering.
func (m *connsTableModel) establishedCount() int { return m.established }

// setFilter sets the live free-text filter; the next Snapshot applies it.
func (m *connsTableModel) setFilter(f string) { m.filter = f }

// setProtoFilter restricts rows to one protocol segment; the next Snapshot
// applies it.
func (m *connsTableModel) setProtoFilter(f protoFilter) { m.proto = f }
