package ui

// tightBorderLayout is a zero-padding variant of Fyne's BorderLayout: top /
// bottom / left / right objects are pinned to the edges at their MinSize and
// the remaining objects fill the center — with NO padding inserted between
// regions. Fyne's stock layout.NewBorderLayout always inserts theme.Padding()
// between regions, which would leave gaps between the shell's bars, sidebar,
// and content; this keeps them flush so the explicit 1px dividers read cleanly.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

var _ fyne.Layout = (*tightBorderLayout)(nil)

type tightBorderLayout struct {
	top, bottom, left, right fyne.CanvasObject
}

// visible reports whether an edge object is present (non-nil) and shown.
func visible(o fyne.CanvasObject) bool {
	return o != nil && o.Visible()
}

// isEdge reports whether c is one of the pinned edge objects, so the center
// pass can skip it.
func (b *tightBorderLayout) isEdge(c fyne.CanvasObject) bool {
	return c == b.top || c == b.bottom || c == b.left || c == b.right
}

// newTightBorder builds a container using tightBorderLayout. Any of the edge
// objects may be nil; center fills whatever remains.
func newTightBorder(top, bottom, left, right, center fyne.CanvasObject) *fyne.Container {
	l := &tightBorderLayout{top: top, bottom: bottom, left: left, right: right}
	objects := make([]fyne.CanvasObject, 0, 5)
	for _, o := range []fyne.CanvasObject{top, bottom, left, right, center} {
		if o != nil {
			objects = append(objects, o)
		}
	}
	return container.New(l, objects...)
}

func (b *tightBorderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var topH, bottomH, leftW, rightW float32
	if visible(b.top) {
		topH = b.top.MinSize().Height
		b.top.Resize(fyne.NewSize(size.Width, topH))
		b.top.Move(fyne.NewPos(0, 0))
	}
	if visible(b.bottom) {
		bottomH = b.bottom.MinSize().Height
		b.bottom.Resize(fyne.NewSize(size.Width, bottomH))
		b.bottom.Move(fyne.NewPos(0, size.Height-bottomH))
	}
	if visible(b.left) {
		leftW = b.left.MinSize().Width
		b.left.Resize(fyne.NewSize(leftW, size.Height-topH-bottomH))
		b.left.Move(fyne.NewPos(0, topH))
	}
	if visible(b.right) {
		rightW = b.right.MinSize().Width
		b.right.Resize(fyne.NewSize(rightW, size.Height-topH-bottomH))
		b.right.Move(fyne.NewPos(size.Width-rightW, topH))
	}

	mid := fyne.NewSize(size.Width-leftW-rightW, size.Height-topH-bottomH)
	pos := fyne.NewPos(leftW, topH)
	for _, c := range objects {
		if !c.Visible() || b.isEdge(c) {
			continue
		}
		c.Resize(mid)
		c.Move(pos)
	}
}

func (b *tightBorderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, c := range objects {
		if !c.Visible() || b.isEdge(c) {
			continue
		}
		min = min.Max(c.MinSize())
	}

	// Horizontal edges (left/right) add their width and only need to fit the
	// tallest; vertical edges (top/bottom) add their height and fit the widest.
	for _, o := range []fyne.CanvasObject{b.left, b.right} {
		if visible(o) {
			m := o.MinSize()
			min = fyne.NewSize(min.Width+m.Width, fyne.Max(min.Height, m.Height))
		}
	}
	for _, o := range []fyne.CanvasObject{b.top, b.bottom} {
		if visible(o) {
			m := o.MinSize()
			min = fyne.NewSize(fyne.Max(min.Width, m.Width), min.Height+m.Height)
		}
	}
	return min
}
