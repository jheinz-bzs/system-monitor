package ui

// scrollTable hosts a dataTable in a vertical scroll with viewport windowing,
// shared by every process table so they scroll identically. The table is built
// with sizeToRows() so its MinSize — and therefore the scrollbar — spans every
// data row; this host syncs the visible slice back to the table (setViewport)
// on every scroll and refresh, so only the on-screen rows are laid out per tick
// no matter how long the list. A short pane scrolls; a tall one shows the whole
// (top-N) list with room to spare.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// scrollTable couples a dataTable with the vertical scroll that hosts it.
type scrollTable struct {
	table  *dataTable
	scroll *container.Scroll
}

// newScrollTable wraps table in a vertical scroll and wires the viewport sync.
// The table should already be configured with sizeToRows().
func newScrollTable(table *dataTable) *scrollTable {
	st := &scrollTable{table: table, scroll: container.NewVScroll(table)}
	st.scroll.OnScrolled = func(fyne.Position) { st.syncViewport() }
	return st
}

// object returns the canvas object to place in a panel body.
func (st *scrollTable) object() fyne.CanvasObject { return st.scroll }

// refresh re-syncs the viewport and redraws. Called each poll tick; it touches
// the canvas, so callers on a background poller must marshal it onto the UI
// goroutine (fyne.Do).
func (st *scrollTable) refresh() {
	st.syncViewport()
	st.scroll.Refresh()
}

// syncViewport tells the table which vertical slice the scroll shows, then
// redraws that slice.
func (st *scrollTable) syncViewport() {
	st.table.setViewport(st.scroll.Offset.Y, st.scroll.Size().Height)
	st.table.Refresh()
}

// scrollToRow centers the given data row in the viewport (clamped to the
// scrollable range), then re-syncs the visible slice.
func (st *scrollTable) scrollToRow(row int) {
	rowH := st.table.rowPixelHeight()
	rowY := tableHeaderHeight + float32(row)*rowH
	target := rowY - (st.scroll.Size().Height-rowH)/2
	maxOffset := max(st.table.MinSize().Height-st.scroll.Size().Height, 0)
	st.scroll.Offset = fyne.NewPos(0, clamp32(target, 0, maxOffset))
	st.scroll.Refresh()
	st.syncViewport()
}
