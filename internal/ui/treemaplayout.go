package ui

// Squarified treemap layout (Bruls, Huizing & van Wijk, 2000): pure geometry
// that packs weighted items into a rectangle as tiles whose aspect ratios stay
// as close to square as possible — the readable alternative to slice-and-dice,
// where one dominant item degenerates into a thin sliver. There is no Fyne
// dependency here, so the algorithm is unit-testable on its own; the treemap
// widget (treemap.go) turns these tiles into canvas rectangles.

import "math"

// treemapRect is a rectangle in the layout's coordinate space (origin top-left).
type treemapRect struct {
	x, y, w, h float64
}

// treemapTile is one positioned block: a rectangle plus the index of the weight
// it represents, back into the caller's input slice (so the caller maps it to a
// label and color).
type treemapTile struct {
	index      int
	x, y, w, h float64
}

// squarifyTreemap lays weights into a width×height rectangle (origin 0,0),
// returning one tile per positive weight. Weights are expected in descending
// order — that ordering is what keeps the tiles near-square — but any order
// yields a valid (if less tidy) tiling. Non-positive weights contribute no
// tile, and a degenerate rectangle or all-zero input yields none at all.
func squarifyTreemap(weights []float64, width, height float64) []treemapTile {
	var total float64
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total <= 0 || width <= 0 || height <= 0 {
		return nil
	}

	// Scale weights to areas so the whole set fills the rectangle exactly.
	scale := (width * height) / total
	areaOf := func(i int) float64 { return weights[i] * scale }

	tiles := make([]treemapTile, 0, len(weights))
	free := treemapRect{x: 0, y: 0, w: width, h: height}
	row := make([]int, 0, len(weights)) // indices in the row currently being grown

	for i := range weights {
		if weights[i] <= 0 {
			continue // zero/negative consumers get no block
		}
		// Grow the current row while adding this item keeps the worst aspect
		// ratio from getting worse; once it would worsen, lay the row out along
		// the free rect's shorter side and start a fresh row in what remains.
		side := math.Min(free.w, free.h)
		if len(row) > 0 && treemapWorst(row, i, areaOf, side) > treemapWorst(row, -1, areaOf, side) {
			free = layoutTreemapRow(&tiles, row, areaOf, free)
			row = row[:0]
		}
		row = append(row, i)
	}
	if len(row) > 0 {
		layoutTreemapRow(&tiles, row, areaOf, free)
	}
	return tiles
}

// treemapWorst returns the worst (largest) aspect ratio among the row's tiles
// when packed along a side of length side, optionally including a candidate
// item (extra >= 0 to include it, < 0 to measure the row as-is). This is the
// paper's worst() function: it drives the decision of whether one more item
// improves or degrades the current row.
func treemapWorst(row []int, extra int, areaOf func(int) float64, side float64) float64 {
	rmax, rmin, sum := 0.0, math.Inf(1), 0.0
	consider := func(i int) {
		a := areaOf(i)
		sum += a
		rmax = math.Max(rmax, a)
		rmin = math.Min(rmin, a)
	}
	for _, i := range row {
		consider(i)
	}
	if extra >= 0 {
		consider(extra)
	}
	if sum <= 0 {
		return math.Inf(1)
	}
	s2 := sum * sum
	w2 := side * side
	return math.Max((w2*rmax)/s2, s2/(w2*rmin))
}

// layoutTreemapRow places the row's items as a strip along the free rect's
// shorter side, appends a tile for each, and returns the free rect that remains
// for subsequent rows. A horizontal strip (when the free rect is wider than
// tall) runs across the top with its thickness measured downward; a vertical
// strip runs down the left with its thickness measured rightward.
func layoutTreemapRow(tiles *[]treemapTile, row []int, areaOf func(int) float64, free treemapRect) treemapRect {
	var sum float64
	for _, i := range row {
		sum += areaOf(i)
	}

	if free.w <= free.h {
		thickness := sum / free.w
		x := free.x
		for _, i := range row {
			tileW := areaOf(i) / thickness
			*tiles = append(*tiles, treemapTile{index: i, x: x, y: free.y, w: tileW, h: thickness})
			x += tileW
		}
		return treemapRect{x: free.x, y: free.y + thickness, w: free.w, h: free.h - thickness}
	}

	thickness := sum / free.h
	y := free.y
	for _, i := range row {
		tileH := areaOf(i) / thickness
		*tiles = append(*tiles, treemapTile{index: i, x: free.x, y: y, w: thickness, h: tileH})
		y += tileH
	}
	return treemapRect{x: free.x + thickness, y: free.y, w: free.w - thickness, h: free.h}
}
