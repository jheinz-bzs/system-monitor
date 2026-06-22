package ui

// Connections tab content (BZS253-61), laid out to match
// tab-08-connections-tcp-udp-table.html:
//
//	page head — "Connections" title beside the live "N ACTIVE  M ESTABLISHED"
//	            readout
//	one pane  — the active-connections panel: a filter/protocol toolbar pinned
//	            above the scroll-hosted table (Proto, local/remote address, State,
//	            owning process, PID, cross-nav link)
//
// The table is the generic dataTable in scroll-hosted mode (sizeToRows + viewport
// windowing), so hundreds of connections stay cheap at the 1s refresh — the same
// machinery the Ports and process tables use. Filter and protocol state live in
// the connsTableModel and are re-applied inside every Snapshot, so a poll tick
// never resets them. A tapped row resolves to its owning process through the same
// processNavigator seam the Ports tab uses.
//
// The view reads connection data through allConnsSource only — never gopsutil or
// monitor types (app.go adapts the ProcessCollector, resolving each connection's
// PID to a process name from the existing process snapshot).

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Page / panel text from the wireframe.
const (
	labelConnectionsPageTitle = "Connections"
	labelActiveConnections    = "Active connections"
)

// connsView is the Connections tab: page head plus the active-connections panel
// (filter toolbar above the scroll-hosted table). Build with newConnsView and
// drive live updates through refresh.
type connsView struct {
	model       *connsTableModel
	table       *dataTable
	tableScroll *scrollTable
	filter      *widget.Entry
	protoSel    *segmentedSelect
	counts      *canvas.Text     // page-head "N ACTIVE  M ESTABLISHED" readout
	nav         processNavigator // cross-tab navigation; nil when not wired
}

// newConnsView builds the Connections tab content. conns feeds the table; nav
// lands a tapped "process →" link on its owning process in the Processes tab.
func newConnsView(conns allConnsSource, nav processNavigator) *connsView {
	v := &connsView{model: newConnsTableModel(conns), nav: nav}
	v.table = newDataTable(v.model,
		tableColumns(v.model.cols...),
		rowHeight(processTableRowHeight),
		rowPool(portsTableRowPool),
		sizeToRows(),
		onRowTapped(v.tapRow),
	)
	v.tableScroll = newScrollTable(v.table)
	v.filter = newFilterEntry(labelConnsFilter, v.changeTextFilter)
	v.protoSel = newSegmentedSelect(0, v.pickProto, labelProtoSegAll, labelProtoSegTCP, labelProtoSegUDP)
	return v
}

// object assembles the tab: page head pinned on top, then the active-connections
// panel filling the remaining height, inset by the shared tab padding.
func (v *connsView) object() fyne.CanvasObject {
	head := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0), v.pageHead())
	body := newTightBorder(head, nil, nil, nil, v.tablePane())
	return container.New(
		layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
}

// pageHead is the wireframe's sm-pagehead row: the tab title beside the live
// "N ACTIVE  M ESTABLISHED" readout (filled by syncReadout each tick).
func (v *connsView) pageHead() fyne.CanvasObject {
	v.counts = newPageSubtitle("")
	return container.New(layout.NewCustomPaddedHBoxLayout(spaceLG),
		vCenter(newHeading(labelConnectionsPageTitle)),
		vCenter(v.counts))
}

// tablePane is the active-connections panel: the filter/protocol toolbar pinned
// above the scroll-hosted table.
func (v *connsView) tablePane() fyne.CanvasObject {
	body := newTightBorder(v.toolbar(), nil, nil, nil, v.tableScroll.object())
	return newFlushPanel(labelActiveConnections, nil, body)
}

// toolbar is the row above the table: the free-text filter beside the
// All/TCP/UDP protocol segments.
func (v *connsView) toolbar() fyne.CanvasObject {
	sizedFilter := container.NewGridWrap(
		fyne.NewSize(procFilterInputW, v.filter.MinSize().Height), v.filter)
	row := container.New(layout.NewCustomPaddedHBoxLayout(spaceMD),
		sizedFilter,
		vCenter(flatFocus(v.protoSel)),
		layout.NewSpacer(),
	)
	return container.New(layout.NewCustomPaddedLayout(
		procToolbarPad, procToolbarPad, procToolbarPad, procToolbarPad), row)
}

// tapRow follows a tapped "process →" row to the Processes tab, highlighting the
// process that owns the connection. Inert when navigation isn't wired. A row
// whose connection vanished between the snapshot and the tap is ignored.
func (v *connsView) tapRow(row int) {
	if v.nav == nil {
		return
	}
	if pid, ok := v.model.pidAt(row); ok {
		v.nav.showProcess(pid)
	}
}

// changeTextFilter applies the typed free-text filter live.
func (v *connsView) changeTextFilter(text string) {
	v.model.setFilter(text)
	v.applyFilters()
}

// pickProto applies the tapped protocol segment. The segment order is
// protoSegments (shared with the Ports tab).
func (v *connsView) pickProto(index int) {
	if index < 0 || index >= len(protoSegments) {
		return
	}
	v.model.setProtoFilter(protoSegments[index])
	v.applyFilters()
}

// applyFilters redraws after any filter change: the table re-snapshots and the
// scroll content height tracks the filtered row count.
func (v *connsView) applyFilters() {
	v.table.Refresh()
	v.tableScroll.scroll.Refresh()
}

// refresh redraws the live table on each poll tick and updates the page-head
// readout. It touches the canvas, so a background poller must marshal it onto the
// UI goroutine (fyne.Do). Filter and protocol state live in the model and survive
// untouched.
func (v *connsView) refresh() {
	v.tableScroll.refresh()
	v.syncReadout()
}

// syncReadout updates the page-head "N ACTIVE  M ESTABLISHED" counts from the
// last snapshot.
func (v *connsView) syncReadout() {
	v.counts.Text = fmt.Sprintf("%d %s   %d %s",
		v.model.activeCount(), labelConnsActive,
		v.model.establishedCount(), labelConnsEstablished)
	v.counts.Refresh()
}
