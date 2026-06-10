package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// sizedRect builds a rectangle with a fixed MinSize, standing in for a pane
// with intrinsic content size (e.g. a fixed-width table).
func sizedRect(w, h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(w, h))
	return r
}

// When one pane's weighted share would undercut its content (the CPU tab's
// table at default window width), it clamps to its minimum and the OTHER pane
// absorbs everything left — not just a weighted sliver of slack. This keeps
// the clamped table flush (no dead space) and the neighbor as large as it can
// be.
func TestWeightedHBoxClampsAndRedistributes(t *testing.T) {
	small := sizedRect(50, 10)
	wide := sizedRect(620, 10)
	box := newWeightedHBox(spaceXL,
		weightedPane{object: small, weight: 1},
		weightedPane{object: wide, weight: 1.25},
	)

	// 1069 total, 1053 available: a pure 1.25/2.25 split would give wide
	// ~585 < its 620 min → wide clamps to 620 and small takes the remaining 433.
	box.Resize(fyne.NewSize(1069, 100))

	if got, want := wide.Size().Width, float32(620); got != want {
		t.Errorf("clamped pane width = %v, want exactly its %v min", got, want)
	}
	if got, want := small.Size().Width, float32(1069-spaceXL-620); got != want {
		t.Errorf("flexible pane width = %v, want the remaining %v", got, want)
	}
}

// With room for everyone, shares follow the pure weight ratio.
func TestWeightedVBoxSplitsByWeight(t *testing.T) {
	a := sizedRect(10, 100)
	b := sizedRect(10, 100)
	box := newWeightedVBox(0,
		weightedPane{object: a, weight: 3},
		weightedPane{object: b, weight: 1},
	)

	box.Resize(fyne.NewSize(50, 600))

	if got, want := a.Size().Height, float32(450); got != want {
		t.Errorf("pane a height = %v, want %v", got, want)
	}
	if got, want := b.Size().Height, float32(150); got != want {
		t.Errorf("pane b height = %v, want %v", got, want)
	}
}

// When the container is smaller than the summed minimums, every pane sits at
// its minimum (overflowing like CSS flex min-content) rather than collapsing.
func TestWeightedBoxOverflowsAtMinimums(t *testing.T) {
	a := sizedRect(300, 10)
	b := sizedRect(300, 10)
	box := newWeightedHBox(0,
		weightedPane{object: a, weight: 1},
		weightedPane{object: b, weight: 1},
	)

	box.Resize(fyne.NewSize(400, 100))

	if got, want := a.Size().Width, float32(300); got != want {
		t.Errorf("pane a width = %v, want its %v min", got, want)
	}
	if got, want := b.Size().Width, float32(300); got != want {
		t.Errorf("pane b width = %v, want its %v min", got, want)
	}
}
