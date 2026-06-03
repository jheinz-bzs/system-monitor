package ui

// Custom sidebar navigation, built to match the design-system wireframe
// (tab-01-overview-sidebar-expanded). Fyne's AppTabs cannot express the design
// (surface background, Mono uppercase labels, the accent highlight, fixed item
// metrics, a per-item index badge), so the nav is a small custom widget.
//
// Per the wireframe:
//   - sidebar: surface (#161a21) background, 178px wide, 1px right divider
//   - item: 32px tall, top-aligned; layout is [icon] [LABEL] … [number]
//   - label: Mono 11 UPPERCASE; index number: Mono 10, right-aligned, text-3
//   - inactive: transparent bg, text-2 label + icon, text-3 number
//   - active: accent-dim fill, 1px accent-line left bar, text label, accent icon
//   - hover: surface-3 fill
//
// The number badge is shown to support future hotkeys (e.g. press 1–8).

import (
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

const (
	navWidth      = 178 // expanded sidebar width
	navItemHeight = 32
	navItemHPad   = 12 // horizontal inset inside an item
	navIconSize   = 16
	navIconGap    = 8  // gap between icon and label
	navBarWidth   = 1  // active-item left accent bar (--sm-accent-line)
	navItemGap    = 4  // vertical gap between items (--sm-1)
	navNumberSize = 10 // index badge text size
)

// navItem is one nav entry: an icon + uppercase label + right-aligned index
// number. It highlights on hover and shows the accent treatment when active.
type navItem struct {
	widget.BaseWidget
	label   string
	icon    fyne.Resource
	index   int // 1-based position, shown as the index badge
	onTap   func()
	active  bool
	hovered bool
}

func newNavItem(label string, icon fyne.Resource, index int, onTap func()) *navItem {
	n := &navItem{label: label, icon: icon, index: index, onTap: onTap}
	n.ExtendBaseWidget(n)
	return n
}

func (n *navItem) setActive(active bool) {
	if n.active != active {
		n.active = active
		n.Refresh()
	}
}

// Tapped implements fyne.Tappable.
func (n *navItem) Tapped(_ *fyne.PointEvent) {
	if n.onTap != nil {
		n.onTap()
	}
}

// MouseIn / MouseMoved / MouseOut implement desktop.Hoverable.
func (n *navItem) MouseIn(_ *desktop.MouseEvent)    { n.hovered = true; n.Refresh() }
func (n *navItem) MouseMoved(_ *desktop.MouseEvent) {}
func (n *navItem) MouseOut()                        { n.hovered = false; n.Refresh() }

// Cursor implements desktop.Cursorable — a pointer over nav items.
func (n *navItem) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (n *navItem) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bar := canvas.NewRectangle(colorAccentLine)

	// Pre-build the icon recolored for each state: text-2 when idle, accent
	// when active. The glyphs are Lucide line icons (stroke-drawn), so they're
	// recolored with colorizeStroke rather than theme.NewColoredResource, which
	// only rewrites fill colors (see colorize.go).
	iconInactive := colorizeStroke(n.icon, colorText2)
	iconActive := colorizeStroke(n.icon, colorAccent)
	icon := canvas.NewImageFromResource(iconInactive)
	icon.FillMode = canvas.ImageFillContain

	label := newColumnLabel(n.label) // Mono 11 UPPERCASE

	number := canvas.NewText(strconv.Itoa(n.index), colorText3)
	number.FontSource = font.MonoRegular
	number.TextSize = navNumberSize

	r := &navItemRenderer{
		item: n, bg: bg, bar: bar, icon: icon, label: label, number: number,
		iconInactive: iconInactive, iconActive: iconActive,
	}
	r.objects = []fyne.CanvasObject{bg, bar, icon, label, number}
	r.apply()
	return r
}

type navItemRenderer struct {
	item         *navItem
	bg           *canvas.Rectangle
	bar          *canvas.Rectangle
	icon         *canvas.Image
	iconInactive fyne.Resource
	iconActive   fyne.Resource
	label        *canvas.Text
	number       *canvas.Text
	objects      []fyne.CanvasObject
}

// apply sets per-state colors and the accent-bar visibility. The index number
// stays muted (text-3) in every state, per the wireframe.
func (r *navItemRenderer) apply() {
	switch {
	case r.item.active:
		r.bg.FillColor = colorAccentDim
		r.bar.Show()
		r.label.Color = colorText
		r.icon.Resource = r.iconActive
	case r.item.hovered:
		r.bg.FillColor = colorSurface3
		r.bar.Hide()
		r.label.Color = colorText2
		r.icon.Resource = r.iconInactive
	default:
		r.bg.FillColor = color.Transparent
		r.bar.Hide()
		r.label.Color = colorText2
		r.icon.Resource = r.iconInactive
	}
}

func (r *navItemRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	r.bar.Resize(fyne.NewSize(navBarWidth, size.Height))
	r.bar.Move(fyne.NewPos(0, 0))

	r.icon.Resize(fyne.NewSize(navIconSize, navIconSize))
	r.icon.Move(fyne.NewPos(navItemHPad, (size.Height-navIconSize)/2))

	lbl := r.label.MinSize()
	r.label.Resize(lbl)
	r.label.Move(fyne.NewPos(navItemHPad+navIconSize+navIconGap, (size.Height-lbl.Height)/2))

	num := r.number.MinSize()
	r.number.Resize(num)
	r.number.Move(fyne.NewPos(size.Width-navItemHPad-num.Width, (size.Height-num.Height)/2))
}

func (r *navItemRenderer) MinSize() fyne.Size {
	w := navItemHPad + navIconSize + navIconGap + r.label.MinSize().Width +
		navIconGap + r.number.MinSize().Width + navItemHPad
	return fyne.NewSize(w, navItemHeight)
}

func (r *navItemRenderer) Refresh() {
	r.apply()
	r.bg.Refresh()
	r.bar.Refresh()
	r.icon.Refresh()
	r.label.Refresh()
	r.number.Refresh()
}

func (r *navItemRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *navItemRenderer) Destroy()                     {}
