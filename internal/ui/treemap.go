package ui

// A squarified treemap widget for the Processes tab's dominance map
// (docs/wireframe designs/tab-06-processes-treemap-sortable-table.html): one
// block per item, sized by the item's weight so the largest resource consumers
// fill the most area. Like lineChart and coreGrid it is a custom widget with a
// pooled renderer — every block's canvas objects are allocated once (up to
// treemapBlockLimit) and arrange() only repositions/recolors/hides them — and
// it re-reads its source on every Refresh, so it tracks the live poll tick.
// (Refresh touches the canvas, so a background poller must marshal the call
// onto the UI goroutine via fyne.Do.)
//
// The widget is decoupled from the process domain: it draws generic
// treemapItems (a label, a weight, a fill hue), so the squarified layout and
// the block chrome stay reusable. processTreemapSource (processtable.go) adapts
// the process list into items sized by the selected metric. Tapping a block
// reports its item index through onSelect — the consumer maps that back to its
// own identifier (the Processes tab resolves it to a PID and highlights the
// table row); a nil onSelect leaves the widget purely presentational.
//
// Block chrome follows the design-system treemap language
// (design-system-06-chart-language.html): a 2px gutter between blocks, each
// filled at ~20% of its hue with a 1px full-hue outline, distinct categorical
// hues so neighbors read apart. Labels are mono, truncated with an ellipsis to
// their block width. Blocks too small to letter at all are dropped — the
// remaining (larger) consumers re-tile and fill the pane — so every visible
// block is readable; the full process table below still lists everything.
// Hovering a block shows its tooltip (the disk treemap's full directory path),
// so a truncated label is always recoverable.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Treemap block chrome and geometry. The gutter and fill alpha come from the
// design-system treemap spec; the rest are component dimensions, so they keep
// their own literal-px consts rather than spacing-scale steps.
const (
	treemapGutter      = 2       // px; gap between adjacent blocks
	treemapFillAlpha   = 0x33    // ~20% — block fill over its full-hue outline
	treemapBlockStroke = 1       // px; full-hue block outline
	treemapLabelInset  = spaceSM // 4; label offset from the block's top-left
	treemapBlockLimit  = 24      // max blocks (top-N cap; keeps small tiles legible)
	treemapMinLabelW   = 24      // px; min inner width for a block to be kept/labeled
	treemapMinLabelH   = 15      // px; min height for a block to be kept/labeled
	treemapEllipsis    = "…"     // truncation marker for clipped labels
	treemapMinWidth    = 200     // px; widget MinSize floor
	treemapMinHeight   = 96      // px; widget MinSize floor
	treemapTipPad      = spaceSM // 4; inner padding of the hover tooltip
	treemapTipOffset   = 12      // px; tooltip offset from the cursor so it doesn't sit under it
	treemapBusyDots    = 32      // px; spinner size in the busy empty state
	treemapBusyGap     = spaceMD // 8; gap between spinner and its label
)

// noTreemapHover is the hoverIndex sentinel for "no block under the cursor".
const noTreemapHover = -1

// labelTreemapEmpty is the default muted placeholder shown when there is
// nothing to plot (no process data yet, or every candidate at zero usage).
// Sources for other domains override it via the treemap's emptyText field.
const labelTreemapEmpty = "no process data"

// treemapItem is one block: a label, a relative weight (its share of the area),
// the full-hue fill the block's chrome derives from, and an optional tooltip
// shown on hover (the disk treemap puts the directory's full path here; empty
// means no tooltip).
type treemapItem struct {
	label   string
	weight  float64
	fill    color.NRGBA
	tooltip string
}

// treemapSource is the data seam between a domain adapter and the widget,
// defined here at the consumer per idiomatic Go. Implementations return a fresh
// slice per call; treemapBlocks is invoked on every Refresh, like
// series.Source.Values on the line chart.
type treemapSource interface {
	treemapBlocks() []treemapItem
}

// treemap is the dominance-map widget. The zero value is not usable; build one
// with newTreemap.
type treemap struct {
	widget.BaseWidget

	src       treemapSource
	onSelect  func(index int) // tapped-block callback; nil leaves the widget presentational
	emptyText string          // placeholder when nothing is plotted; defaults to labelTreemapEmpty
	hits      []treemapHit    // placed blocks for hit-testing, recorded each arrange

	hoverIndex int              // source index of the hovered block, or noTreemapHover
	hoverPos   fyne.Position    // latest cursor position; the tooltip tracks it
	renderer   *treemapRenderer // captured in CreateRenderer to move the tooltip without re-arranging
}

// treemapHit is one placed block's screen rect, its source item index, and its
// tooltip text, recorded each arrange so a tap or hover can be mapped back to
// the block it fell on.
type treemapHit struct {
	x, y, w, h float32
	index      int
	tooltip    string
}

var (
	_ fyne.Tappable      = (*treemap)(nil)
	_ desktop.Cursorable = (*treemap)(nil)
	_ desktop.Hoverable  = (*treemap)(nil)
)

// newTreemap returns a treemap fed by src. Sources are re-read on every Refresh.
// onSelect fires with the tapped block's item index (its position in the source's
// last treemapBlocks result); pass nil for a presentational-only treemap.
func newTreemap(src treemapSource, onSelect func(index int)) *treemap {
	t := &treemap{src: src, onSelect: onSelect, emptyText: labelTreemapEmpty, hoverIndex: noTreemapHover}
	t.ExtendBaseWidget(t)
	return t
}

// hitAt returns the placed block under pos, or nil if pos falls on a gutter,
// the empty placeholder, or outside every block.
func (t *treemap) hitAt(pos fyne.Position) *treemapHit {
	for i := range t.hits {
		h := &t.hits[i]
		if pos.X >= h.x && pos.X < h.x+h.w && pos.Y >= h.y && pos.Y < h.y+h.h {
			return h
		}
	}
	return nil
}

// Tapped implements fyne.Tappable: report the block the tap fell on, ignoring
// taps on the gutters, the empty placeholder, or a widget with no callback.
func (t *treemap) Tapped(ev *fyne.PointEvent) {
	if t.onSelect == nil {
		return
	}
	if h := t.hitAt(ev.Position); h != nil {
		t.onSelect(h.index)
	}
}

// MouseIn / MouseMoved / MouseOut implement desktop.Hoverable, driving the path
// tooltip so it tracks the cursor in real time.
func (t *treemap) MouseIn(e *desktop.MouseEvent)    { t.hover(e.Position) }
func (t *treemap) MouseMoved(e *desktop.MouseEvent) { t.hover(e.Position) }
func (t *treemap) MouseOut() {
	if t.hoverIndex == noTreemapHover {
		return
	}
	t.hoverIndex = noTreemapHover
	t.Refresh() // hide the tooltip
}

// hover updates the tooltip for the cursor position. Entering a different block
// (or leaving the blocks) triggers a full Refresh so the tooltip's text and
// visibility rebuild; moving within the same block only repositions the tooltip
// objects, avoiding a re-read/re-squarify of the source on every pixel.
func (t *treemap) hover(pos fyne.Position) {
	idx := noTreemapHover
	if h := t.hitAt(pos); h != nil {
		idx = h.index
	}
	t.hoverPos = pos
	if idx != t.hoverIndex {
		t.hoverIndex = idx
		t.Refresh()
		return
	}
	if idx != noTreemapHover && t.renderer != nil {
		t.renderer.moveTooltip()
	}
}

// Cursor implements desktop.Cursorable — a pointer when blocks are selectable,
// signaling the map is interactive; the default arrow otherwise.
func (t *treemap) Cursor() desktop.Cursor {
	if t.onSelect == nil {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

// treemapBlock is one pooled block: the rounded tile rectangle and its label.
type treemapBlock struct {
	rect  *canvas.Rectangle
	label *canvas.Text
}

// newTreemapBlock builds a pooled block with the shared, per-frame-invariant
// chrome (stroke width, corner radius, label face); per-frame state — fill hue,
// size, position, label text, visibility — is set in arrange.
func newTreemapBlock() *treemapBlock {
	rect := canvas.NewRectangle(color.Transparent)
	rect.StrokeWidth = treemapBlockStroke
	rect.CornerRadius = theme.Size(theme.SizeNameInputRadius)
	return &treemapBlock{
		rect:  rect,
		label: styledText("", font.MonoRegular, theme.SizeNameCaptionText, palette.Text),
	}
}

func (t *treemap) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(palette.PlotBG)
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = palette.Border
	border.StrokeWidth = 1

	empty := newMeta(t.emptyText)
	empty.Hide()

	tipBG := canvas.NewRectangle(palette.Surface2)
	tipBG.StrokeColor = palette.BorderStrong
	tipBG.StrokeWidth = treemapBlockStroke
	tipBG.CornerRadius = theme.Size(theme.SizeNameInputRadius)
	tipBG.Hide()
	tip := styledText("", font.MonoRegular, theme.SizeNameCaptionText, palette.Text)
	tip.Hide()

	r := &treemapRenderer{tm: t, bg: bg, border: border, empty: empty, tipBG: tipBG, tip: tip}
	r.blocks = make([]*treemapBlock, treemapBlockLimit)
	for i := range r.blocks {
		r.blocks[i] = newTreemapBlock()
	}
	t.renderer = r // so hover can move the tooltip without a full re-arrange
	return r
}

type treemapRenderer struct {
	tm *treemap

	bg     *canvas.Rectangle
	border *canvas.Rectangle
	empty  *canvas.Text
	tipBG  *canvas.Rectangle // hover tooltip background
	tip    *canvas.Text      // hover tooltip text (the block's full path)
	blocks []*treemapBlock

	size fyne.Size
}

func (r *treemapRenderer) Layout(size fyne.Size) {
	r.size = size
	r.arrange()
}

func (r *treemapRenderer) Refresh() {
	r.arrange()
	canvas.Refresh(r.tm)
}

// arrange recomputes the whole scene for the current size and data: the
// plot-bg frame, then one block per squarified tile (fitTiles drops blocks too
// small to label). It reads the source once, hides every block up front, and
// shows only those that get a tile — so a tick with fewer (or zero) processes
// leaves no stale blocks behind.
func (r *treemapRenderer) arrange() {
	if r.size.Width <= 0 || r.size.Height <= 0 {
		return
	}
	r.bg.Resize(r.size)
	r.bg.Move(fyne.NewPos(0, 0))
	r.border.Resize(r.size)
	r.border.Move(fyne.NewPos(0, 0))

	items := r.tm.src.treemapBlocks()
	if len(items) > len(r.blocks) {
		items = items[:len(r.blocks)]
	}
	weights := make([]float64, len(items))
	for i, it := range items {
		weights[i] = it.weight
	}
	tiles := r.fitTiles(weights)

	r.tm.hits = r.tm.hits[:0]
	for _, b := range r.blocks {
		b.rect.Hide()
		b.label.Hide()
	}
	for _, tile := range tiles {
		r.arrangeBlock(r.blocks[tile.index], items[tile.index], tile)
	}
	r.syncEmpty(len(tiles))
	r.syncTooltip()
}

// fitTiles squarifies the largest prefix of weights (the biggest consumers)
// whose every tile is large enough to label, dropping the smallest blocks that
// would be unreadable. Because the layout always fills the rect, the survivors
// re-tile with no gaps. Weights must be in descending order; the search shrinks
// from the full set so the dominant processes are the ones kept. nil when even
// the single largest block can't be labeled (e.g. a degenerate pane).
func (r *treemapRenderer) fitTiles(weights []float64) []treemapTile {
	w, h := float64(r.size.Width), float64(r.size.Height)
	for n := len(weights); n >= 1; n-- {
		tiles := squarifyTreemap(weights[:n], w, h)
		if r.allLabelable(tiles) {
			return tiles
		}
	}
	return nil
}

// allLabelable reports whether every tile clears the label-size thresholds.
func (r *treemapRenderer) allLabelable(tiles []treemapTile) bool {
	for _, tile := range tiles {
		if !labelFits(float32(tile.w)-treemapGutter, float32(tile.h)-treemapGutter) {
			return false
		}
	}
	return len(tiles) > 0
}

// labelFits reports whether a block of inner size w×h (post-gutter) is big
// enough to carry a label. The single source of truth shared by the drop
// decision (allLabelable) and the label rendering (arrangeLabel).
func labelFits(w, h float32) bool {
	return w-2*treemapLabelInset >= treemapMinLabelW && h >= treemapMinLabelH
}

// arrangeBlock positions and colors one block for its tile, recording its rect
// for tap hit-testing. The tile is inset by half the gutter on every side, so
// two neighbors leave a full treemapGutter between them. A block whose inset
// collapses to nothing stays hidden and contributes no hit region.
func (r *treemapRenderer) arrangeBlock(b *treemapBlock, item treemapItem, tile treemapTile) {
	x := float32(tile.x) + treemapGutter/2
	y := float32(tile.y) + treemapGutter/2
	w := float32(tile.w) - treemapGutter
	h := float32(tile.h) - treemapGutter
	if w <= 0 || h <= 0 {
		return
	}

	b.rect.FillColor = withAlpha(item.fill, treemapFillAlpha)
	b.rect.StrokeColor = item.fill
	b.rect.Resize(fyne.NewSize(w, h))
	b.rect.Move(fyne.NewPos(x, y))
	b.rect.Show()

	r.tm.hits = append(r.tm.hits, treemapHit{x: x, y: y, w: w, h: h, index: tile.index, tooltip: item.tooltip})
	r.arrangeLabel(b.label, item.label, x, y, w, h)
}

// syncTooltip shows or hides the hover tooltip during a full arrange, reading
// the text from the freshly-recorded hits so it always matches what is drawn (a
// block that vanished on a data tick simply shows nothing), then positions it.
func (r *treemapRenderer) syncTooltip() {
	text := r.hoverTooltip()
	if text == "" {
		r.tipBG.Hide()
		r.tip.Hide()
		return
	}
	r.tip.Text = text
	r.tip.Refresh()
	r.tipBG.Show()
	r.tip.Show()
	r.positionTooltip()
}

// moveTooltip repositions the already-visible tooltip to follow the cursor
// without a full arrange — the cheap path taken while hovering within one
// block. A no-op when the hovered block carries no tooltip text.
func (r *treemapRenderer) moveTooltip() {
	if !r.tip.Visible() {
		return
	}
	r.positionTooltip()
	canvas.Refresh(r.tipBG)
	canvas.Refresh(r.tip)
}

// positionTooltip places the tooltip near the cursor (offset so it doesn't sit
// under the pointer), clamped to stay inside the widget. tip.Text must already
// be set.
func (r *treemapRenderer) positionTooltip() {
	sz := r.tip.MinSize()
	w := sz.Width + 2*treemapTipPad
	h := sz.Height + 2*treemapTipPad
	x := clampTip(r.tm.hoverPos.X+treemapTipOffset, w, r.size.Width)
	y := clampTip(r.tm.hoverPos.Y+treemapTipOffset, h, r.size.Height)

	r.tipBG.Resize(fyne.NewSize(w, h))
	r.tipBG.Move(fyne.NewPos(x, y))
	r.tip.Move(fyne.NewPos(x+treemapTipPad, y+treemapTipPad))
}

// hoverTooltip returns the tooltip text of the currently hovered block, or ""
// when nothing is hovered or the hovered block is no longer drawn.
func (r *treemapRenderer) hoverTooltip() string {
	if r.tm.hoverIndex == noTreemapHover {
		return ""
	}
	for _, h := range r.tm.hits {
		if h.index == r.tm.hoverIndex {
			return h.tooltip
		}
	}
	return ""
}

// clampTip keeps a tooltip edge of the given size within [0, max].
func clampTip(v, size, max float32) float32 {
	if v+size > max {
		v = max - size
	}
	if v < 0 {
		v = 0
	}
	return v
}

// arrangeLabel draws the process name in the block's top-left, truncated to the
// available width and hidden entirely when the block is too small to letter.
func (r *treemapRenderer) arrangeLabel(label *canvas.Text, text string, x, y, w, h float32) {
	if !labelFits(w, h) {
		label.Hide()
		return
	}
	fitted := truncateToWidth(label, text, w-2*treemapLabelInset)
	if fitted == "" {
		label.Hide()
		return
	}
	label.Text = fitted
	label.Refresh()
	sz := label.MinSize()
	label.Resize(sz)
	label.Move(fyne.NewPos(x+treemapLabelInset, y+treemapLabelInset))
	label.Show()
}

// syncEmpty shows the centered placeholder exactly when no block was drawn.
func (r *treemapRenderer) syncEmpty(tileCount int) {
	if tileCount > 0 {
		r.empty.Hide()
		return
	}
	sz := r.empty.MinSize()
	r.empty.Resize(sz)
	r.empty.Move(fyne.NewPos((r.size.Width-sz.Width)/2, (r.size.Height-sz.Height)/2))
	r.empty.Show()
}

// truncateToWidth returns text trimmed (with an ellipsis) to fit maxW when
// rendered by label, or text unchanged when it already fits. It exploits the
// monospace face's uniform glyph advance to find the fit in one measurement
// rather than a per-character search; returns "" when not even one glyph plus
// the ellipsis fits.
func truncateToWidth(label *canvas.Text, text string, maxW float32) string {
	if text == "" {
		return ""
	}
	label.Text = text
	full := label.MinSize().Width
	if full <= maxW {
		return text
	}
	runes := []rune(text)
	advance := full / float32(len(runes)) // uniform in a monospace face
	if advance <= 0 {
		return ""
	}
	fit := int(maxW/advance) - 1 // reserve one glyph's width for the ellipsis
	if fit < 1 {
		return ""
	}
	if fit > len(runes) {
		fit = len(runes)
	}
	return string(runes[:fit]) + treemapEllipsis
}

func (r *treemapRenderer) MinSize() fyne.Size {
	return fyne.NewSize(treemapMinWidth, treemapMinHeight)
}

// Objects assembles the draw order: plot fill, every block rect, every label
// (above all rects so a tile can't paint over its neighbor's name), then the
// frame and the empty-state text on top. Objects are reused across frames.
func (r *treemapRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, len(r.blocks)*2+5)
	objs = append(objs, r.bg)
	for _, b := range r.blocks {
		objs = append(objs, b.rect)
	}
	for _, b := range r.blocks {
		objs = append(objs, b.label)
	}
	// border and empty placeholder, then the tooltip on top of everything.
	return append(objs, r.border, r.empty, r.tipBG, r.tip)
}

func (r *treemapRenderer) Destroy() {}
