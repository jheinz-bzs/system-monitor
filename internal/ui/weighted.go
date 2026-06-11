package ui

// weightedBoxLayout splits a container's full extent along one axis among its
// children, separated by a fixed gap. Every child is floored at its MinSize
// along the axis; only the slack above those floors is distributed in
// proportion to the per-child weights. This is the Fyne translation of the
// wireframes' flex semantics (e.g. the CPU tab's 1.3 : 1 chart-to-bottom split
// and 1 : 1.25 per-core-to-processes split): panes grow by flex-grow ratio but
// never shrink below their content, so a fixed-width table keeps its last
// column on screen however the window is sized. No stock Fyne layout expresses
// this: Box gives every child only its MinSize and Grid gives every child an
// equal share.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

var _ fyne.Layout = (*weightedBoxLayout)(nil)

// weightedPane pairs a pane with its flex weight.
type weightedPane struct {
	object fyne.CanvasObject
	weight float32
}

type weightedBoxLayout struct {
	horizontal bool
	gap        float32
	weights    []float32
}

// newWeightedHBox lays items out left→right, splitting the width by weight.
func newWeightedHBox(gap float32, items ...weightedPane) *fyne.Container {
	return newWeightedBox(true, gap, items)
}

// newWeightedVBox lays items out top→bottom, splitting the height by weight.
func newWeightedVBox(gap float32, items ...weightedPane) *fyne.Container {
	return newWeightedBox(false, gap, items)
}

// newWeightedBox builds the container. The pane set is fixed at construction —
// adding children to the returned container later would desync them from the
// weights, so Layout ignores any extras.
func newWeightedBox(horizontal bool, gap float32, items []weightedPane) *fyne.Container {
	l := &weightedBoxLayout{horizontal: horizontal, gap: gap}
	objects := make([]fyne.CanvasObject, 0, len(items))
	for _, it := range items {
		l.weights = append(l.weights, it.weight)
		objects = append(objects, it.object)
	}
	return container.New(l, objects...)
}

// totalWeight sums the weights, guarding against a zero sum so Layout never
// divides by zero (all-zero weights degrade to equal shares of nothing).
func (l *weightedBoxLayout) totalWeight() float32 {
	var sum float32
	for _, w := range l.weights {
		sum += w
	}
	return sum
}

func (l *weightedBoxLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if l.totalWeight() <= 0 || len(objects) == 0 || len(objects) > len(l.weights) {
		return // objects added past construction have no weight to honor
	}
	axis := size.Height
	if l.horizontal {
		axis = size.Width
	}
	shares := l.resolveShares(objects, axis-l.gap*float32(len(objects)-1))

	var offset float32
	for i, o := range objects {
		if l.horizontal {
			o.Resize(fyne.NewSize(shares[i], size.Height))
			o.Move(fyne.NewPos(offset, 0))
		} else {
			o.Resize(fyne.NewSize(size.Width, shares[i]))
			o.Move(fyne.NewPos(0, offset))
		}
		offset += shares[i] + l.gap
	}
}

// resolveShares distributes avail across the panes purely by weight, then
// clamps any pane whose share would fall below its MinSize to that minimum and
// re-distributes the remainder among the rest — the CSS flex min-content
// algorithm. Pure weights keep the wireframe ratios when there is room; clamps
// only bend the ratio as far as content demands (so a fixed-width table takes
// exactly its width and the neighboring pane absorbs the slack). When every
// pane is clamped the container overflows at the minimums, like flex items
// refusing to shrink below min-content. Each pass freezes at least one pane,
// so the loop runs at most len(objects) times.
func (l *weightedBoxLayout) resolveShares(objects []fyne.CanvasObject, avail float32) []float32 {
	mins := make([]float32, len(objects))
	for i, o := range objects {
		m := o.MinSize()
		if l.horizontal {
			mins[i] = m.Width
		} else {
			mins[i] = m.Height
		}
	}

	shares := make([]float32, len(objects))
	frozen := make([]bool, len(objects))
	for {
		remaining := avail
		var weightSum float32
		for i := range objects {
			if frozen[i] {
				remaining -= shares[i]
			} else {
				weightSum += l.weights[i]
			}
		}
		if weightSum <= 0 {
			return shares // everything clamped
		}
		clamped := false
		for i := range objects {
			if frozen[i] {
				continue
			}
			shares[i] = remaining * l.weights[i] / weightSum
			if shares[i] < mins[i] {
				shares[i] = mins[i]
				frozen[i] = true
				clamped = true
			}
		}
		if !clamped {
			return shares
		}
	}
}

func (l *weightedBoxLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	minSize := fyne.NewSize(0, 0)
	for _, o := range objects {
		m := o.MinSize()
		if l.horizontal {
			minSize = fyne.NewSize(minSize.Width+m.Width, fyne.Max(minSize.Height, m.Height))
		} else {
			minSize = fyne.NewSize(fyne.Max(minSize.Width, m.Width), minSize.Height+m.Height)
		}
	}
	if len(objects) > 1 {
		gaps := l.gap * float32(len(objects)-1)
		if l.horizontal {
			minSize = fyne.NewSize(minSize.Width+gaps, minSize.Height)
		} else {
			minSize = fyne.NewSize(minSize.Width, minSize.Height+gaps)
		}
	}
	return minSize
}
