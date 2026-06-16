package ui

// Disk tab content (BZS253-52), laid out to match
// docs/pitch/4 _ Disk _ treemap _ volumes _ I_O.png:
//
//	page head   — "Disk" title, "N volumes · X total" subtitle
//	top row     — storage-by-directory panel (left, reserved for the directory
//	              treemap) + volumes panel (right): one semantically colored
//	              usage bar per mounted partition
//	bottom pane — I/O panel (reserved for the read/write/total line chart)
//
// Only the volumes bars are live today; the directory treemap (a filesystem
// walk) and the I/O chart (its own card) are reserved empty panels so the
// layout already holds their space. The view reads partition usage through the
// diskUsageSource seam only — app.go adapts DiskCollector.Usage().

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Pane / panel text. The em-dash panel titles come verbatim from the wireframe.
const (
	labelDiskPageTitle = "Disk"
	labelStorageByDir  = "Storage — by directory"
	labelVolumesPanel  = "Volumes"
	labelIOPanel       = "I/O"
	labelVolumesEmpty  = "no volume data"
	labelVolumeSizeSep = " / "      // "420G / 512G"
	fmtVolumeSubtitle  = "%d volumes · %s total"
	fmtVolumePercent   = "%d%% used" // "82% used"
)

// Disk-usage warning thresholds: a partition at or above diskWarnFraction shows
// the warning hue, at or above diskCritFraction the critical hue (the card's
// >80% low-space indicator). Below, the bar uses the neutral accent (the
// wireframe colors a normal volume with the accent, not green). Fractions of
// total capacity, not percent.
const (
	diskWarnFraction = 0.80
	diskCritFraction = 0.90
)

// Disk-tab pane weights, from the wireframe's flex-grow ratios: the top row is
// 1.2× the I/O pane's height; within it the directory panel is 1.7× the volumes
// panel's width.
const (
	diskTopWeight     = 1.2
	diskBottomWeight  = 1
	storageDirWeight  = 1.7
	volumesPaneWeight = 1
)

// Volumes-list geometry, from the wireframe's volumes panel. Off-grid bar size
// keeps its own literal-px const; the row/line gaps are exact spacing steps.
const (
	volumeRowLimit  = 12       // max pooled rows (partitions are few; caps the pool)
	volumeBarHeight = 6        // px; usage bar track/fill height
	volumeRowGap    = space2XL // 24; vertical gap between volume rows
	volumeLineGap   = spaceSM  // 4; gap between a row's header, bar, and percent
	volumeMinWidth  = 240      // px; panel floor so name + sizes stay readable
)

// diskPartition is the UI's per-partition usage shape: a mount path and its byte
// totals. app.go adapts monitor.PartitionUsage into this so the view never
// imports the monitor layer.
type diskPartition struct {
	mount string
	total uint64
	used  uint64
}

// diskUsageSource feeds the volumes list a fresh snapshot per call, mirroring
// allProcessSource. Defined at the consumer per idiomatic Go.
type diskUsageSource interface {
	partitions() []diskPartition
}

// diskUsageSourceFunc adapts a plain func to a diskUsageSource.
type diskUsageSourceFunc func() []diskPartition

func (f diskUsageSourceFunc) partitions() []diskPartition { return f() }

// diskUsageColor maps a used fraction onto the bar hue: critical at or above
// diskCritFraction, warning at or above diskWarnFraction, the neutral accent
// below.
func diskUsageColor(frac float64) color.NRGBA {
	switch {
	case frac >= diskCritFraction:
		return withAlpha(palette.Red, 0xff)
	case frac >= diskWarnFraction:
		return withAlpha(palette.Yellow, 0xff)
	default:
		return withAlpha(palette.Accent, 0xff)
	}
}

// diskPercentColor colors the "N% used" caption: the bar's warning/critical hue
// when over the threshold (so low space pops), the muted meta color otherwise.
func diskPercentColor(frac float64) color.Color {
	if frac >= diskWarnFraction {
		return diskUsageColor(frac)
	}
	return palette.Text3
}

// diskSizeText renders a partition's used/total in the table's compact byte
// style ("420G / 512G").
func diskSizeText(p diskPartition) string {
	return formatBytesShort(p.used) + labelVolumeSizeSep + formatBytesShort(p.total)
}

// diskPercentText renders the used fraction as a whole-percent caption.
func diskPercentText(frac float64) string {
	return fmt.Sprintf(fmtVolumePercent, int(frac*100))
}

// diskSubtitle composes the page-head subtitle from the partitions with real
// capacity: their count and summed total. Pseudo-filesystems (no total) are
// excluded, matching the volumes the list shows.
func diskSubtitle(parts []diskPartition) string {
	var count int
	var total uint64
	for _, p := range parts {
		if p.total == 0 {
			continue
		}
		count++
		total += p.total
	}
	return fmt.Sprintf(fmtVolumeSubtitle, count, formatBytesShort(total))
}

// volumesList is the Volumes panel body: one usage bar per mounted partition,
// stacked top→bottom. Like coreGrid it is a pooled-renderer widget — rows are
// allocated once up to volumeRowLimit and arrange() repositions/recolors them —
// and re-reads its source on every Refresh, tracking the poll tick. Partitions
// without a real total (pseudo-filesystems) contribute no row.
type volumesList struct {
	widget.BaseWidget

	src diskUsageSource
}

// newVolumesList returns a list fed by src. The source is re-read on every
// Refresh.
func newVolumesList(src diskUsageSource) *volumesList {
	v := &volumesList{src: src}
	v.ExtendBaseWidget(v)
	return v
}

// volumeRow is one partition's pooled canvas objects: the mount name (left) and
// used/total sizes (right) on the header line, the bar track/fill beneath, and
// the percent caption under that.
type volumeRow struct {
	name    *canvas.Text
	sizes   *canvas.Text
	track   *canvas.Rectangle
	fill    *canvas.Rectangle
	percent *canvas.Text
}

// newVolumeRow builds a row's per-frame-invariant chrome; text, fill hue, sizes,
// and positions are set in arrange.
func newVolumeRow() *volumeRow {
	return &volumeRow{
		name:    styledText("", font.MonoMedium, theme.SizeNameCaptionText, palette.Text),
		sizes:   styledText("", font.MonoRegular, theme.SizeNameCaptionText, palette.Text2),
		track:   newBarRect(palette.Surface3),
		fill:    newBarRect(palette.Accent),
		percent: styledText("", font.MonoRegular, theme.SizeNameCaptionText, palette.Text3),
	}
}

func (row *volumeRow) show() {
	row.track.Show()
	row.fill.Show()
	row.name.Show()
	row.sizes.Show()
	row.percent.Show()
}

func (row *volumeRow) hide() {
	row.track.Hide()
	row.fill.Hide()
	row.name.Hide()
	row.sizes.Hide()
	row.percent.Hide()
}

func (v *volumesList) CreateRenderer() fyne.WidgetRenderer {
	r := &volumesListRenderer{list: v}
	r.rows = make([]*volumeRow, volumeRowLimit)
	for i := range r.rows {
		r.rows[i] = newVolumeRow()
	}
	r.empty = newMeta(labelVolumesEmpty)
	r.empty.Hide()
	return r
}

type volumesListRenderer struct {
	list  *volumesList
	rows  []*volumeRow
	empty *canvas.Text
	size  fyne.Size
}

func (r *volumesListRenderer) Layout(size fyne.Size) {
	r.size = size
	r.arrange()
}

func (r *volumesListRenderer) Refresh() {
	r.arrange()
	canvas.Refresh(r.list)
}

// arrange recomputes the rows for the current size and data: read the snapshot,
// hide every pooled row, then stack one row per partition with real capacity
// from the top. It reads the source once and shows only the rows it fills, so a
// removed volume leaves no stale row behind.
func (r *volumesListRenderer) arrange() {
	if r.size.Width <= 0 || r.size.Height <= 0 {
		return
	}
	parts := r.visiblePartitions()
	for _, row := range r.rows {
		row.hide()
	}
	y := float32(0)
	for i, p := range parts {
		y += r.arrangeRow(r.rows[i], p, y) + volumeRowGap
	}
	r.syncEmpty(len(parts))
}

// visiblePartitions returns the snapshot's partitions with real capacity, capped
// at the pool size.
func (r *volumesListRenderer) visiblePartitions() []diskPartition {
	parts := r.list.src.partitions()
	out := make([]diskPartition, 0, len(parts))
	for _, p := range parts {
		if p.total == 0 {
			continue
		}
		out = append(out, p)
		if len(out) == volumeRowLimit {
			break
		}
	}
	return out
}

// arrangeRow lays one partition out at vertical offset y across the full width,
// colors its bar and caption by usage, and returns the height it consumed so the
// caller can stack the next row.
func (r *volumesListRenderer) arrangeRow(row *volumeRow, p diskPartition, y float32) float32 {
	frac := float64(p.used) / float64(p.total)
	w := r.size.Width

	row.name.Text = p.mount
	row.name.Refresh()
	row.sizes.Text = diskSizeText(p)
	row.sizes.Refresh()
	row.percent.Text = diskPercentText(frac)
	row.percent.Color = diskPercentColor(frac)
	row.percent.Refresh()

	nameSize := row.name.MinSize()
	sizesSize := row.sizes.MinSize()
	headerH := max(nameSize.Height, sizesSize.Height)
	row.name.Resize(nameSize)
	row.name.Move(fyne.NewPos(0, y+(headerH-nameSize.Height)/2))
	row.sizes.Resize(sizesSize)
	row.sizes.Move(fyne.NewPos(w-sizesSize.Width, y+(headerH-sizesSize.Height)/2))

	barY := y + headerH + volumeLineGap
	row.track.Resize(fyne.NewSize(w, volumeBarHeight))
	row.track.Move(fyne.NewPos(0, barY))
	row.fill.FillColor = diskUsageColor(frac)
	row.fill.Resize(fyne.NewSize(w*clamp32(float32(frac), 0, 1), volumeBarHeight))
	row.fill.Move(fyne.NewPos(0, barY))

	pctY := barY + volumeBarHeight + volumeLineGap
	pctSize := row.percent.MinSize()
	row.percent.Resize(pctSize)
	row.percent.Move(fyne.NewPos(w-pctSize.Width, pctY))

	row.show()
	return pctY + pctSize.Height - y
}

// syncEmpty shows the centered placeholder exactly when no volume was drawn.
func (r *volumesListRenderer) syncEmpty(count int) {
	if count > 0 {
		r.empty.Hide()
		return
	}
	sz := r.empty.MinSize()
	r.empty.Resize(sz)
	r.empty.Move(fyne.NewPos((r.size.Width-sz.Width)/2, (r.size.Height-sz.Height)/2))
	r.empty.Show()
}

func (r *volumesListRenderer) MinSize() fyne.Size {
	return fyne.NewSize(volumeMinWidth, 0)
}

// Objects lists each row's track under its fill, with the texts on top; the
// empty placeholder last. Objects are reused across frames.
func (r *volumesListRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, len(r.rows)*5+1)
	for _, row := range r.rows {
		objs = append(objs, row.track, row.fill, row.name, row.sizes, row.percent)
	}
	return append(objs, r.empty)
}

func (r *volumesListRenderer) Destroy() {}

// diskView is the Disk tab: page head, the storage/volumes top row, and the
// reserved I/O pane. Build with newDiskView and drive live updates through
// refresh.
type diskView struct {
	volumes *volumesList
	sub     string // page-head subtitle, snapshotted from the initial partitions
}

// newDiskView builds the Disk tab content from the adapted partition source.
func newDiskView(src diskUsageSource) *diskView {
	return &diskView{
		volumes: newVolumesList(src),
		sub:     diskSubtitle(src.partitions()),
	}
}

// object assembles the tab: page head pinned on top, then the top row and the
// I/O pane splitting the remaining height by the wireframe's weights, inset by
// the shared tab padding.
func (v *diskView) object() fyne.CanvasObject {
	head := container.New(layout.NewCustomPaddedLayout(0, tabPad, 0, 0), v.pageHead())
	column := newWeightedVBox(tabPad,
		weightedPane{object: v.topRow(), weight: diskTopWeight},
		weightedPane{object: v.ioPane(), weight: diskBottomWeight},
	)
	body := newTightBorder(head, nil, nil, nil, column)
	return container.New(layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), body)
}

// pageHead is the wireframe's sm-pagehead row: the tab title and the volume
// count / total-capacity subtitle.
func (v *diskView) pageHead() fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedHBoxLayout(spaceLG),
		vCenter(newHeading(labelDiskPageTitle)),
		vCenter(newPageSubtitle(v.sub)))
}

// topRow pairs the storage-by-directory panel (reserved for the directory
// treemap) with the volumes panel. The directory panel keeps the larger share,
// per the wireframe.
func (v *diskView) topRow() fyne.CanvasObject {
	storage := newPanel(labelStorageByDir, nil, layout.NewSpacer())
	volumes := newPanel(labelVolumesPanel, nil, v.volumes)
	return newWeightedHBox(tabPad,
		weightedPane{object: storage, weight: storageDirWeight},
		weightedPane{object: volumes, weight: volumesPaneWeight},
	)
}

// ioPane is the reserved I/O panel — empty until the read/write/total line chart
// lands. Titled with the live history window so the header is already truthful.
func (v *diskView) ioPane() fyne.CanvasObject {
	return newPanel(historyTitle(labelIOPanel), nil, layout.NewSpacer())
}

// refresh redraws the volumes list. It touches the canvas, so a background
// poller must marshal it onto the UI goroutine (fyne.Do).
func (v *diskView) refresh() { v.volumes.Refresh() }
