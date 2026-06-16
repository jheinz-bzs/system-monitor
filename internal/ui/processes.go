package ui

// Processes tab content (BZS253-56), laid out to match
// tab-06-processes-treemap-sortable-table.html:
//
//	page head   — "Processes" title
//	top pane    — dominance treemap (placeholder until the treemap card lands)
//	bottom pane — the full sortable/filterable process table behind a toolbar
//	              with the live name filter and the Kill action
//
// The table is the generic dataTable in its scroll-hosted mode: sizeToRows
// makes the scrollbar span every (filtered) process and setViewport windows
// the rendering to the visible slice, so hundreds of rows stay cheap at the
// 1s refresh. Sort, filter, and selection state live in the
// allProcessTableSource adapter (processtable.go) and are re-applied inside
// every Snapshot — a poll tick can never reset them.
//
// The view reads process data through allProcessSource and terminates through
// processKiller only — never gopsutil or monitor types directly (the
// composition root adapts in app.go).

import (
	"fmt"
	"log"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Pane / panel / control text. The em-dash panel title, filter placeholder,
// and filter prefixes/options come verbatim from the wireframe.
const (
	labelProcessesPageTitle = "Processes"
	labelAllProcesses       = "All processes"
	labelDominanceMap       = "Dominance map — sized by CPU"
	labelKill               = "Kill"
	labelFilterPlaceholder  = "filter by name, user, pid…"
	labelUserFilterPrefix   = "user:"
	labelStatusFilterPrefix = "status:"
	filterOptionAll         = "all" // the user filter's no-filter choice
	filterOptionAny         = "any" // the status filter's no-filter choice
)

// Toolbar geometry. The filter width is an off-grid component dimension from
// the wireframe's toolbar input; the entry keeps its theme-derived height
// (forcing the wireframe's 28px would clip Fyne's entry chrome).
const (
	procFilterInputW = 220     // px; filter entry width
	procToolbarPad   = spaceMD // 8; toolbar inset within the table pane
)

// Pane weights: the wireframe gives the table pane a modestly larger share
// than the treemap, matching the CPU tab's processes-pane precedent.
const (
	treemapPaneWeight  = 1
	allProcsPaneWeight = 1.25
)

// processesView is the Processes tab: page head, the dominance-map placeholder
// pane, and the all-processes table with its filter/kill toolbar. Build with
// newProcessesView and drive live updates through refresh.
type processesView struct {
	adapter   *allProcessTableSource
	table     *dataTable
	scroll    *container.Scroll
	filter    *widget.Entry
	userSel   *widget.Select
	statusSel *widget.Select
	counts    *canvas.Text   // page-head "N TOTAL · M HIGH USAGE" readout
	kill      processKiller  // nil when termination isn't wired
	killBtn   *widget.Button // nil when kill is nil
}

// newProcessesView builds the Processes tab content. procs feeds the table;
// killer enables the Kill action and may be nil (the button is omitted —
// nil-collector degradation, matching the other tabs' fallbacks).
func newProcessesView(procs allProcessSource, killer processKiller) *processesView {
	v := &processesView{kill: killer}
	v.table, v.adapter = newAllProcessTable(procs, v.tapRow, v.tapHeader)
	v.scroll = container.NewVScroll(v.table)
	v.scroll.OnScrolled = func(fyne.Position) { v.syncViewport() }
	v.filter = newFilterEntry(v.changeTextFilter)
	v.userSel = newFilterSelect([]string{filterOptionAll}, filterOptionAll, v.pickUser)
	v.statusSel = newFilterSelect(statusFilterOptions(), filterOptionAny, v.pickStatus)
	if killer != nil {
		v.killBtn = newKillButton(v.killSelected)
	}
	v.syncSortMarkers()
	return v
}

// newFilterSelect builds one toolbar filter dropdown with its no-filter
// choice preselected.
func newFilterSelect(options []string, selected string, onChanged func(string)) *widget.Select {
	s := widget.NewSelect(options, onChanged)
	s.Selected = selected
	return s
}

// statusFilterOptions lists the status dropdown's choices: "any" plus the
// filterable states.
func statusFilterOptions() []string {
	opts := []string{filterOptionAny}
	for _, st := range procStatusFilterable {
		opts = append(opts, string(st))
	}
	return opts
}

// newFilterEntry builds the toolbar's live filter input: mono text with the
// wireframe placeholder, change events forwarded as they're typed.
func newFilterEntry(onChanged func(string)) *widget.Entry {
	e := widget.NewEntry()
	e.PlaceHolder = labelFilterPlaceholder
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.OnChanged = onChanged
	return e
}

// newKillButton builds the destructive Kill action, disabled until a row is
// selected.
func newKillButton(onTapped func()) *widget.Button {
	b := widget.NewButton(labelKill, onTapped)
	b.Importance = widget.DangerImportance
	b.Disable()
	return b
}

// object assembles the tab: page head pinned on top, then the treemap
// placeholder pane and the table pane splitting the remaining height.
func (v *processesView) object() fyne.CanvasObject {
	head := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0), v.pageHead())
	column := newWeightedVBox(tabPad,
		weightedPane{object: newPanel(labelDominanceMap, nil, layout.NewSpacer()), weight: treemapPaneWeight},
		weightedPane{object: v.tablePane(), weight: allProcsPaneWeight},
	)
	body := newTightBorder(head, nil, nil, nil, column)
	return container.New(
		layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
}

// pageHead is the wireframe's sm-pagehead row: the tab title beside the live
// "N TOTAL · M HIGH USAGE" readout (filled by syncReadout each tick).
func (v *processesView) pageHead() fyne.CanvasObject {
	v.counts = newPageSubtitle("")
	return container.New(layout.NewCustomPaddedHBoxLayout(spaceLG),
		vCenter(newHeading(labelProcessesPageTitle)),
		vCenter(v.counts))
}

// tablePane is the all-processes panel: the filter/kill toolbar pinned above
// the scroll-hosted table.
func (v *processesView) tablePane() fyne.CanvasObject {
	body := newTightBorder(v.toolbar(), nil, nil, nil, v.scroll)
	return newFlushPanel(labelAllProcesses, nil, body)
}

// toolbar is the row above the table: the free-text filter, the user and
// status dropdowns, then the Kill action on the right (omitted when
// termination isn't wired).
func (v *processesView) toolbar() fyne.CanvasObject {
	sizedFilter := container.NewGridWrap(
		fyne.NewSize(procFilterInputW, v.filter.MinSize().Height), v.filter)
	row := container.New(layout.NewCustomPaddedHBoxLayout(spaceMD),
		sizedFilter,
		vCenter(newFilterPrefix(labelUserFilterPrefix)), flatFocus(v.userSel),
		vCenter(newFilterPrefix(labelStatusFilterPrefix)), flatFocus(v.statusSel),
		layout.NewSpacer(),
	)
	if v.killBtn != nil {
		row.Add(vCenter(v.killBtn))
	}
	return container.New(layout.NewCustomPaddedLayout(
		procToolbarPad, procToolbarPad, procToolbarPad, procToolbarPad), row)
}

// newFilterPrefix renders a dropdown's muted "user:" / "status:" lead-in.
func newFilterPrefix(text string) *canvas.Text {
	return styledText(text, font.MonoRegular, theme.SizeNameCaptionText, palette.Text3)
}

// tapHeader sorts by the tapped column (tap again to flip direction), then
// re-tags the headers and redraws.
func (v *processesView) tapHeader(col int) {
	spec := procTableSpec()
	if col >= len(spec) {
		return
	}
	v.adapter.toggleSort(spec[col].sort)
	v.syncSortMarkers()
	v.table.Refresh()
}

// syncSortMarkers rewrites the column headers so the active sort column —
// and only it — carries its direction marker.
func (v *processesView) syncSortMarkers() {
	for i, s := range procTableSpec() {
		header := s.column.header
		if s.sort == v.adapter.sortCol {
			header += sortMarker(v.adapter.sortDir)
		}
		v.table.setColumnHeader(i, header)
	}
}

// tapRow selects the tapped row and repaints its highlight.
func (v *processesView) tapRow(row int) {
	v.adapter.selectRow(row)
	v.syncKillState()
	v.table.Refresh()
}

// changeTextFilter applies the typed free-text filter live.
func (v *processesView) changeTextFilter(text string) {
	v.adapter.setFilter(text)
	v.applyFilters()
}

// pickUser applies the "user:" dropdown choice; "all" clears it.
func (v *processesView) pickUser(opt string) {
	if opt == filterOptionAll {
		opt = ""
	}
	v.adapter.setUserFilter(opt)
	v.applyFilters()
}

// pickStatus applies the "status:" dropdown choice; "any" clears it.
func (v *processesView) pickStatus(opt string) {
	if opt == filterOptionAny {
		opt = ""
	}
	v.adapter.setStatusFilter(procStatus(opt))
	v.applyFilters()
}

// applyFilters redraws after any filter change. Filtering the selected row
// out clears the selection, so the Kill state re-syncs too.
func (v *processesView) applyFilters() {
	v.table.Refresh()
	v.scroll.Refresh() // content height tracks the filtered row count
	v.syncReadout()
	v.syncKillState()
}

// syncReadout updates the page-head counts and the user dropdown's choices
// from the last snapshot.
func (v *processesView) syncReadout() {
	total, high := v.adapter.counts()
	v.counts.Text = fmt.Sprintf("%d TOTAL · %d HIGH USAGE", total, high)
	v.counts.Refresh()
	v.syncUserOptions()
}

// syncUserOptions rebuilds the user dropdown's choices when the machine's
// user set changes, keeping the active choice listed even after its last
// process exits (so the control never shows a value it doesn't contain).
func (v *processesView) syncUserOptions() {
	opts := append([]string{filterOptionAll}, v.adapter.userOptions()...)
	if v.userSel.Selected != "" && !slices.Contains(opts, v.userSel.Selected) {
		opts = append(opts, v.userSel.Selected)
	}
	if slices.Equal(opts, v.userSel.Options) {
		return
	}
	v.userSel.SetOptions(opts)
}

// killSelected terminates the selected process. Failure is logged rather than
// surfaced — the row simply stays; the next tick reflects reality either way.
func (v *processesView) killSelected() {
	pid, ok := v.adapter.selectedPID()
	if !ok || v.kill == nil {
		return
	}
	if err := v.kill.kill(pid); err != nil {
		log.Printf("kill process %d: %v", pid, err)
	}
}

// syncKillState enables the Kill action exactly while a row is selected.
func (v *processesView) syncKillState() {
	if v.killBtn == nil {
		return
	}
	if _, ok := v.adapter.selectedPID(); ok {
		v.killBtn.Enable()
		return
	}
	v.killBtn.Disable()
}

// selectPID selects and highlights the process with the given PID, scrolling
// it into view. Cross-tab navigation (CPU/Ports → Processes) reaches this
// through the tab registry's tabContent.selectPID; wiring those callers is a
// later card.
func (v *processesView) selectPID(pid PID) {
	v.adapter.selectPID(pid)
	v.table.Refresh() // re-snapshot so the row index below is current
	if row := v.adapter.rowIndexOf(pid); row != noTableRow {
		v.scrollToRow(row)
	}
	v.syncKillState()
}

// scrollToRow centers the given data row in the scroll viewport (clamped to
// the scrollable range).
func (v *processesView) scrollToRow(row int) {
	rowY := tableHeaderHeight + float32(row)*processTableRowHeight
	target := rowY - (v.scroll.Size().Height-processTableRowHeight)/2
	maxOffset := max(v.table.MinSize().Height-v.scroll.Size().Height, 0)
	v.scroll.Offset = fyne.NewPos(0, clamp32(target, 0, maxOffset))
	v.scroll.Refresh()
	v.syncViewport()
}

// syncViewport tells the table which vertical slice of itself the scroll
// shows, then redraws that slice.
func (v *processesView) syncViewport() {
	v.table.setViewport(v.scroll.Offset.Y, v.scroll.Size().Height)
	v.table.Refresh()
}

// refresh redraws the live pane on each poll tick. It touches the canvas, so
// callers on a background poller must marshal it onto the UI goroutine
// (fyne.Do). Sort, filter, and selection live in the adapter and survive
// untouched; the scroll refresh keeps the content height tracking the row
// count; a selection that vanished with its process disables Kill.
func (v *processesView) refresh() {
	v.syncViewport()
	v.scroll.Refresh()
	v.syncReadout()
	v.syncKillState()
}
