package ui

// Ports tab content (BZS253-59), laid out to match
// tab-07-ports-table-cross-nav.html:
//
//	page head — "Ports" title beside the live "N LISTENING" readout
//	one pane  — the listening-ports panel: a filter/protocol toolbar pinned above
//	            the scroll-hosted table (Proto, Port, State, owning process, PID,
//	            local address)
//
// The table is the generic dataTable in its scroll-hosted mode (sizeToRows +
// viewport windowing), so hundreds of ports stay cheap at the 1s refresh — the
// same machinery the process tables use. Filter and protocol state live in the
// portsTableModel and are re-applied inside every Snapshot, so a poll tick never
// resets them. Rows carry the hover/pointer affordance and a tap hook; resolving
// a tapped row to its owning process is the seam BZS253-60's "jump to process"
// navigation wires onto.
//
// The view reads port data through allPortsSource only — never gopsutil or
// monitor types (app.go adapts the ProcessCollector, resolving each port's PID to
// a process name from the existing process snapshot).

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Page / panel / toolbar text. The panel title, filter placeholder, protocol
// segments, and the listening-only indicator come from the wireframe.
const (
	labelPortsPageTitle = "Ports"
	labelListeningPorts = "Listening ports"
	labelPortsFilter    = "filter by port, process…"
	labelListeningOnly  = "listening only"
	labelProtoSegAll    = "All"
	labelProtoSegTCP    = "TCP"
	labelProtoSegUDP    = "UDP"
)

// protoSegments lists the protocol-filter segments in the toggle's order, so a
// tapped segment index maps to a protoFilter without a magic-number switch.
var protoSegments = []protoFilter{protoFilterAll, protoFilterTCP, protoFilterUDP}

// portsView is the Ports tab: page head plus the listening-ports panel (filter
// toolbar above the scroll-hosted table). Build with newPortsView and drive live
// updates through refresh.
type portsView struct {
	model       *portsTableModel
	table       *dataTable
	tableScroll *scrollTable
	filter      *widget.Entry
	protoSel    *segmentedSelect
	counts      *canvas.Text     // page-head "N LISTENING" readout
	nav         processNavigator // cross-tab navigation; nil when not wired
}

// newPortsView builds the Ports tab content. ports feeds the table; nav lands a
// tapped "process →" link on its owning process in the Processes tab.
func newPortsView(ports allPortsSource, nav processNavigator) *portsView {
	v := &portsView{model: newPortsTableModel(ports), nav: nav}
	v.table = newDataTable(v.model,
		tableColumns(v.model.cols...),
		rowHeight(processTableRowHeight),
		rowPool(portsTableRowPool),
		sizeToRows(),
		onRowTapped(v.tapRow),
	)
	v.tableScroll = newScrollTable(v.table)
	v.filter = newFilterEntry(labelPortsFilter, v.changeTextFilter)
	v.protoSel = newSegmentedSelect(0, v.pickProto, labelProtoSegAll, labelProtoSegTCP, labelProtoSegUDP)
	return v
}

// object assembles the tab: page head pinned on top, then the listening-ports
// panel filling the remaining height, inset by the shared tab padding.
func (v *portsView) object() fyne.CanvasObject {
	head := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0), v.pageHead())
	body := newTightBorder(head, nil, nil, nil, v.tablePane())
	return container.New(
		layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
}

// pageHead is the wireframe's sm-pagehead row: the tab title beside the live
// "N LISTENING" readout (filled by syncReadout each tick).
func (v *portsView) pageHead() fyne.CanvasObject {
	v.counts = newPageSubtitle("")
	return container.New(layout.NewCustomPaddedHBoxLayout(spaceLG),
		vCenter(newHeading(labelPortsPageTitle)),
		vCenter(v.counts))
}

// tablePane is the listening-ports panel: the filter/protocol toolbar pinned
// above the scroll-hosted table.
func (v *portsView) tablePane() fyne.CanvasObject {
	body := newTightBorder(v.toolbar(), nil, nil, nil, v.tableScroll.object())
	return newFlushPanel(labelListeningPorts, nil, body)
}

// toolbar is the row above the table: the free-text filter, the All/TCP/UDP
// protocol segments, then the muted "listening only" indicator (the table only
// ever lists listening ports).
func (v *portsView) toolbar() fyne.CanvasObject {
	sizedFilter := container.NewGridWrap(
		fyne.NewSize(procFilterInputW, v.filter.MinSize().Height), v.filter)
	row := container.New(layout.NewCustomPaddedHBoxLayout(spaceMD),
		sizedFilter,
		vCenter(flatFocus(v.protoSel)),
		vCenter(newFilterPrefix(labelListeningOnly)),
		layout.NewSpacer(),
	)
	return container.New(layout.NewCustomPaddedLayout(
		procToolbarPad, procToolbarPad, procToolbarPad, procToolbarPad), row)
}

// tapRow follows a tapped "process →" row to the Processes tab, highlighting the
// process that owns the port. Inert when navigation isn't wired. A row whose port
// vanished between the snapshot and the tap (index out of range) is ignored.
func (v *portsView) tapRow(row int) {
	if v.nav == nil {
		return
	}
	if pid, ok := v.model.pidAt(row); ok {
		v.nav.showProcess(pid)
	}
}

// changeTextFilter applies the typed free-text filter live.
func (v *portsView) changeTextFilter(text string) {
	v.model.setFilter(text)
	v.applyFilters()
}

// pickProto applies the tapped protocol segment. The segment order is
// protoSegments.
func (v *portsView) pickProto(index int) {
	if index < 0 || index >= len(protoSegments) {
		return
	}
	v.model.setProtoFilter(protoSegments[index])
	v.applyFilters()
}

// applyFilters redraws after any filter change: the table re-snapshots and the
// scroll content height tracks the filtered row count.
func (v *portsView) applyFilters() {
	v.table.Refresh()
	v.tableScroll.scroll.Refresh()
}

// refresh redraws the live table on each poll tick and updates the page-head
// readout. It touches the canvas, so a background poller must marshal it onto the
// UI goroutine (fyne.Do). Filter and protocol state live in the model and survive
// untouched.
func (v *portsView) refresh() {
	v.tableScroll.refresh()
	v.syncReadout()
}

// syncReadout updates the page-head listening-port count from the last snapshot.
func (v *portsView) syncReadout() {
	v.counts.Text = fmt.Sprintf("%d LISTENING", v.model.listeningCount())
	v.counts.Refresh()
}
