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

// placeHBar pins a full-width bar (top or bottom) at its MinSize height and
// returns that height (0 when the bar is absent). atTop pins it to y=0;
// otherwise it sits flush against the bottom edge.
func placeHBar(o fyne.CanvasObject, size fyne.Size, atTop bool) float32 {
	if !visible(o) {
		return 0
	}
	h := o.MinSize().Height
	y := float32(0)
	if !atTop {
		y = size.Height - h
	}
	o.Resize(fyne.NewSize(size.Width, h))
	o.Move(fyne.NewPos(0, y))
	return h
}

// placeVBar pins a sidebar (left or right) spanning the mid-band left between
// the top and bottom bars, and returns its MinSize width (0 when absent).
// atLeft pins it to x=0; otherwise it sits flush against the right edge.
func placeVBar(o fyne.CanvasObject, size fyne.Size, topH, bottomH float32, atLeft bool) float32 {
	if !visible(o) {
		return 0
	}
	w := o.MinSize().Width
	x := float32(0)
	if !atLeft {
		x = size.Width - w
	}
	o.Resize(fyne.NewSize(w, size.Height-topH-bottomH))
	o.Move(fyne.NewPos(x, topH))
	return w
}

func (b *tightBorderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	topH := placeHBar(b.top, size, true)
	bottomH := placeHBar(b.bottom, size, false)
	leftW := placeVBar(b.left, size, topH, bottomH, true)
	rightW := placeVBar(b.right, size, topH, bottomH, false)

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

// addEdges grows min to fit a set of edge objects along one axis. Side edges
// (left/right, addsWidth=true) add their width and only need min tall enough
// for the tallest; stacked edges (top/bottom) add their height and need min
// wide enough for the widest.
func addEdges(min fyne.Size, addsWidth bool, edges ...fyne.CanvasObject) fyne.Size {
	for _, o := range edges {
		if !visible(o) {
			continue
		}
		m := o.MinSize()
		if addsWidth {
			min = fyne.NewSize(min.Width+m.Width, fyne.Max(min.Height, m.Height))
		} else {
			min = fyne.NewSize(fyne.Max(min.Width, m.Width), min.Height+m.Height)
		}
	}
	return min
}

func (b *tightBorderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, c := range objects {
		if !c.Visible() || b.isEdge(c) {
			continue
		}
		min = min.Max(c.MinSize())
	}

	min = addEdges(min, true, b.left, b.right)
	min = addEdges(min, false, b.top, b.bottom)
	return min
}
