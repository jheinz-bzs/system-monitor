package ui

// Disk tab content (BZS253-52), laid out to match
// docs/pitch/4 _ Disk _ treemap _ volumes _ I_O.png:
//
//	page head   — "Disk" title, "N volumes · X total" subtitle
//	top row     — storage-by-directory treemap (left, fed by a background
//	              filesystem walk) + volumes panel (right): one semantically
//	              colored usage bar per mounted partition
//	bottom pane — I/O panel: the read/write/total line chart
//
// The view reads partition usage through the diskUsageSource seam and the
// directory snapshot through diskDirSource; app.go adapts the monitor
// collectors/scanner behind both.

import (
	"fmt"
	"image/color"
	"log"
	"os/exec"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/series"
)

// Pane / panel text. The em-dash panel titles come verbatim from the wireframe.
const (
	labelDiskPageTitle = "Disk"
	labelStorageByDir  = "Storage — by directory"
	labelVolumesPanel  = "Volumes"
	labelIOPanel       = "I/O"
	labelVolumesEmpty  = "no volume data"
	labelDirsEmpty     = "scanning…" // directory treemap, before the first walk lands
	labelRescan        = "rescan"    // storage header's manual re-walk control
	labelVolumeSizeSep = " / "       // "420G / 512G"
	fmtVolumeSubtitle  = "%d volumes · %s total"
	fmtVolumePercent   = "%d%% used" // "82% used"
)

// I/O chart legend labels and the categorical-palette indices for the read /
// write series (total is the emphasized accent line). c2 (cyan) reads as flow,
// c3 (violet) as its counterpart — the wireframe's two secondary I/O hues.
const (
	labelLegendTotal = "total"
	labelLegendRead  = "read"
	labelLegendWrite = "write"

	ioReadSeriesIndex  = 1 // c2
	ioWriteSeriesIndex = 2 // c3
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
// Rows are pooled on demand (one per partition) and the list scrolls
// vertically, so a machine with many volumes never stretches the pane.
const (
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

// diskDir is one scanned directory's size: a display label (its name under the
// scan root), its full path (shown as the treemap tile's hover tooltip), and
// byte total. app.go adapts monitor.DirSize into this.
type diskDir struct {
	label string
	path  string
	bytes uint64
}

// diskDirSource feeds the directory treemap a fresh snapshot of the currently
// selected volume's largest directories. Defined at the consumer per idiomatic
// Go; app.go's scan controller implements it over the background scanner.
type diskDirSource interface {
	dirs() []diskDir
}

// diskIOSources bundles the I/O chart's three series: total (emphasized) plus
// the read and write rates. The zero value means the disk I/O isn't wired and
// the I/O pane keeps its placeholder.
type diskIOSources struct {
	total series.Source
	read  series.Source
	write series.Source
}

// wired reports whether all three I/O series were adapted.
func (s diskIOSources) wired() bool {
	return s.total != nil && s.read != nil && s.write != nil
}

// diskDirTreemapSource adapts the selected volume's directory snapshot into
// treemap blocks: one block per directory sized by bytes, assigned categorical
// hues by position so neighbors read apart. The snapshot arrives already sorted
// largest-first (the treemap squarifier wants descending weights). It is the
// disk analogue of processTreemapSource.
type diskDirTreemapSource struct {
	src diskDirSource
}

func (s diskDirTreemapSource) treemapBlocks() []treemapItem {
	dirs := s.src.dirs()
	items := make([]treemapItem, len(dirs))
	for i, d := range dirs {
		items[i] = treemapItem{
			label:   d.label,
			weight:  float64(d.bytes),
			fill:    palette.Series[i%len(palette.Series)],
			tooltip: d.path,
		}
	}
	return items
}

// openDirAt returns the treemap's tapped-block callback: it resolves the block
// index against a fresh directory snapshot and reveals that directory in the OS
// file manager. The bounds guard covers the rare case where the snapshot shrank
// (a volume switch or crawl landing) between the arrange that placed the block
// and the tap.
func openDirAt(src diskDirSource) func(index int) {
	return func(index int) {
		dirs := src.dirs()
		if index < 0 || index >= len(dirs) {
			return
		}
		openDir(dirs[index].path)
	}
}

// openDir reveals path in the OS default file manager (Explorer on Windows,
// Finder on macOS, xdg-open elsewhere). It takes a native path directly rather
// than a file:// URL because Windows' URL handler won't reliably open folders
// with spaces (e.g. "C:\Program Files"). Fire-and-forget via Start: explorer
// exits non-zero even on success, so the exit code is meaningless here; only a
// failure to launch the command is logged.
func openDir(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("open %q: %v", path, err)
	}
}

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

// withCapacity returns the partitions backed by real storage (a non-zero
// total), dropping pseudo-filesystems. It is the single definition of "a
// volume the Disk tab shows" — shared by the subtitle, the volumes list, and
// the scan selector.
func withCapacity(parts []diskPartition) []diskPartition {
	out := make([]diskPartition, 0, len(parts))
	for _, p := range parts {
		if p.total > 0 {
			out = append(out, p)
		}
	}
	return out
}

// diskSubtitle composes the page-head subtitle from the partitions with real
// capacity: their count and summed total.
func diskSubtitle(parts []diskPartition) string {
	real := withCapacity(parts)
	var total uint64
	for _, p := range real {
		total += p.total
	}
	return fmt.Sprintf(fmtVolumeSubtitle, len(real), formatBytesShort(total))
}

// volumesList is the Volumes panel body: one usage bar per mounted partition,
// stacked top→bottom. Like coreGrid it is a pooled-renderer widget — rows are
// pooled on demand (one per partition) and arrange() repositions/recolors them —
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
	// Probe text for row-height math in MinSize: both of a row's text lines are
	// caption-sized, so one measurement fixes the whole row height.
	r.probe = styledText("0", font.MonoMedium, theme.SizeNameCaptionText, palette.Text)
	r.empty = newMeta(labelVolumesEmpty)
	r.empty.Hide()
	return r
}

type volumesListRenderer struct {
	list  *volumesList
	rows  []*volumeRow // pooled on demand, one per partition
	probe *canvas.Text // caption-height measurement for MinSize; never drawn
	empty *canvas.Text
	size  fyne.Size
}

// ensureRows grows the row pool to hold n rows. Existing rows are reused;
// the pool never shrinks (surplus rows are hidden by arrange).
func (r *volumesListRenderer) ensureRows(n int) {
	for len(r.rows) < n {
		r.rows = append(r.rows, newVolumeRow())
	}
}

func (r *volumesListRenderer) Layout(fyne.Size) {
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
	// Size comes from the widget, never a Layout-fed renderer field: Fyne
	// destroys renderers left unpainted for ~1 min and recreates them on
	// demand, and a recreated renderer gets no Layout call while the widget's
	// size is unchanged (the enclosing scroller's content resize is a no-op).
	// A renderer-cached size would stay zero and leave the list blank until an
	// external window resize; the widget's own size survives recreation.
	r.size = r.list.Size()
	if r.size.Width <= 0 || r.size.Height <= 0 {
		return
	}
	parts := r.visiblePartitions()
	r.ensureRows(len(parts))
	for _, row := range r.rows {
		row.hide()
	}
	y := float32(0)
	for i, p := range parts {
		y += r.arrangeRow(r.rows[i], p, y) + volumeRowGap
	}
	r.syncEmpty(len(parts))
}

// visiblePartitions returns the snapshot's partitions with real capacity.
func (r *volumesListRenderer) visiblePartitions() []diskPartition {
	return withCapacity(r.list.src.partitions())
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

// MinSize reports the full stacked height of every row, so the enclosing
// vertical scroller knows the true content extent. Row height mirrors
// arrangeRow exactly: a caption header line, the bar, and a caption percent
// line, separated by volumeLineGap.
func (r *volumesListRenderer) MinSize() fyne.Size {
	n := len(r.visiblePartitions())
	if n > 0 {
		captionH := r.probe.MinSize().Height
		rowH := 2*captionH + 2*volumeLineGap + volumeBarHeight
		return fyne.NewSize(volumeMinWidth, float32(n)*rowH+float32(n-1)*volumeRowGap)
	}
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

// diskView is the Disk tab: page head, the storage (directory treemap) /
// volumes top row, and the I/O chart pane. Build with newDiskView and drive
// live updates through refresh.
type diskView struct {
	volumes      *volumesList
	dirmap       *treemap   // directory treemap; nil when no scanner is wired
	io           *lineChart // I/O read/write/total chart; nil when disk I/O isn't wired
	sub          string     // page-head subtitle, snapshotted from the initial partitions
	mounts       []string   // selectable volume mounts, snapshotted for the scan selector
	selectVolume func(mount string)
	rescan       func() // triggers a fresh walk of the selected volume; nil leaves the control out
}

// newDiskView builds the Disk tab content. src feeds the volumes list and the
// page head. dirs feeds the storage directory treemap and may be nil (the
// storage panel keeps its placeholder). io feeds the I/O chart and may be
// unwired. selectVolume retargets the directory scan when the header's volume
// selector changes; nil leaves the selector out. rescan triggers a fresh walk of
// the selected volume; nil leaves the rescan control out.
func newDiskView(src diskUsageSource, dirs diskDirSource, io diskIOSources, selectVolume func(mount string), rescan func()) *diskView {
	v := &diskView{
		volumes:      newVolumesList(src),
		sub:          diskSubtitle(src.partitions()),
		selectVolume: selectVolume,
		rescan:       rescan,
	}
	if dirs != nil {
		v.dirmap = newTreemap(diskDirTreemapSource{src: dirs}, openDirAt(dirs))
		v.dirmap.emptyText = labelDirsEmpty
		v.dirmap.emptyBusy = true // no cached snapshot yet → show the scanning spinner
	}
	if io.wired() {
		v.io = newDiskIOChart(io)
	}
	for _, p := range withCapacity(src.partitions()) {
		v.mounts = append(v.mounts, p.mount)
	}
	return v
}

// newDiskIOChart builds the I/O line chart: an auto-scaled byte/sec Y axis over
// the history window, the emphasized total line plus the read and write series
// in their categorical hues.
func newDiskIOChart(io diskIOSources) *lineChart {
	chart := newLineChart(
		autoRange(),
		valueFormat(formatBytesAxis),
		window(metrics.HistoryCapacity),
		timeAxis(historySpan()),
	)
	chart.addSeries(io.total, emphasized())
	chart.addSeries(io.read, seriesColor(palette.Series[ioReadSeriesIndex]))
	chart.addSeries(io.write, seriesColor(palette.Series[ioWriteSeriesIndex]))
	return chart
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

// topRow pairs the storage-by-directory panel (the directory treemap, with the
// volume selector in its header) with the volumes panel. The directory panel
// keeps the larger share, per the wireframe.
func (v *diskView) topRow() fyne.CanvasObject {
	storage := newPanel(labelStorageByDir, v.storageHeader(), v.storageBody())
	// Vertical scroll so many volumes never stretch the pane past the window.
	volumes := newPanel(labelVolumesPanel, nil, container.NewVScroll(v.volumes))
	return newWeightedHBox(tabPad,
		weightedPane{object: storage, weight: storageDirWeight},
		weightedPane{object: volumes, weight: volumesPaneWeight},
	)
}

// storageBody is the directory treemap, or a spacer while no scanner is wired.
func (v *diskView) storageBody() fyne.CanvasObject {
	if v.dirmap == nil {
		return layout.NewSpacer()
	}
	return v.dirmap
}

// storageHeader is the storage panel's trailing controls: the volume selector
// (when more than one volume) beside the manual rescan link. Either may be
// absent; it returns just the present one, both in a row, or nil for a plain
// header.
func (v *diskView) storageHeader() fyne.CanvasObject {
	selector := v.volumeSelector()
	rescan := v.rescanLink()
	switch {
	case selector != nil && rescan != nil:
		return container.New(layout.NewCustomPaddedHBoxLayout(spaceLG),
			vCenter(selector), vCenter(rescan))
	case rescan != nil:
		return rescan
	default:
		return selector
	}
}

// rescanLink is the manual "rescan" control — a tappable header link that
// triggers a fresh walk of the selected volume. Omitted when no rescan hook is
// wired (no scanner), so the header degrades like the volume selector does.
func (v *diskView) rescanLink() fyne.CanvasObject {
	if v.rescan == nil {
		return nil
	}
	return newJumpLink(labelRescan, v.rescan)
}

// volumeSelector is the header control choosing which volume the directory
// treemap scans (the wireframe's "Macintosh HD" / "Data" segmented control,
// rendered as a dropdown so many mounts never overflow the header). The
// dropdown widths to its longest label and opens a scrolled list, so the
// natural width cap the segmented control needed is gone. It's omitted when
// there's nothing to retarget — no selectVolume hook or only one volume.
func (v *diskView) volumeSelector() fyne.CanvasObject {
	if v.selectVolume == nil || len(v.mounts) < 2 {
		return nil
	}
	labels := make([]string, 0, len(v.mounts))
	mountByLabel := make(map[string]string, len(v.mounts))
	for i, m := range v.mounts {
		l := volumeLabel(m)
		if _, dup := mountByLabel[l]; dup {
			// volumeLabel strips trailing separators, so "/data" and "/data/"
			// would share a label and the first match would select the wrong
			// mount. Disambiguate the duplicate with its index so every label
			// maps to exactly one mount.
			l = fmt.Sprintf("%s [%d]", l, i+1)
		}
		mountByLabel[l] = m
		labels = append(labels, l)
	}
	sel := widget.NewSelect(labels, func(chosen string) {
		if m, ok := mountByLabel[chosen]; ok {
			v.selectVolume(m)
		}
	})
	// Seed the first volume so the dropdown and the shown treemap agree at
	// startup (scanRoots order). Set the field directly — the newFilterSelect
	// pattern — so seeding doesn't fire the callback at build time.
	sel.Selected = labels[0]
	// widget.Select sizes itself to the selected label only, so a short
	// default volume would clip longer mounts in the popup. Floor the control
	// at the widest option (label text plus the renderer's chrome) with a
	// transparent spacer so every mount name fits.
	widest := float32(0)
	th := sel.Theme()
	for _, l := range labels {
		if w := fyne.MeasureText(l, th.Size(theme.SizeNameText), fyne.TextStyle{}).Width; w > widest {
			widest = w
		}
	}
	chrome := th.Size(theme.SizeNameInnerPadding)*4 + th.Size(theme.SizeNameInlineIcon)
	floor := canvas.NewRectangle(color.Transparent)
	floor.SetMinSize(fyne.NewSize(widest+chrome, 1))
	return flatFocus(container.NewStack(floor, sel))
}

// volumeLabel renders a mount path as a compact selector label, dropping a
// trailing path separator so "C:\" reads as "C:".
func volumeLabel(mount string) string {
	if trimmed := strings.TrimRight(mount, `/\`); trimmed != "" {
		return trimmed
	}
	return mount
}

// ioPane is the I/O panel: the read/write/total line chart with its legend, or
// a placeholder while disk I/O isn't wired. Titled with the live history window.
func (v *diskView) ioPane() fyne.CanvasObject {
	if v.io == nil {
		return newPanel(historyTitle(labelIOPanel), nil, layout.NewSpacer())
	}
	legend := newLegend(
		legendEntry{label: labelLegendTotal, col: palette.Accent},
		legendEntry{label: labelLegendRead, col: palette.Series[ioReadSeriesIndex]},
		legendEntry{label: labelLegendWrite, col: palette.Series[ioWriteSeriesIndex]},
	)
	return newPanel(historyTitle(labelIOPanel), legend, v.io)
}

// refresh redraws the live panes — the volumes list, the directory treemap
// (which re-reads the latest scan snapshot), and the I/O chart. It touches the
// canvas, so a background poller must marshal it onto the UI goroutine
// (fyne.Do).
func (v *diskView) refresh() {
	v.volumes.Refresh()
	if v.dirmap != nil {
		v.dirmap.Refresh()
	}
	if v.io != nil {
		v.io.Refresh()
	}
}
