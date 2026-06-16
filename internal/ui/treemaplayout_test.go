package ui

import (
	"math"
	"testing"
)

// sumTileArea totals the area of a tile set — the laid-out tiles should fill
// the requested rectangle (modulo float rounding).
func sumTileArea(tiles []treemapTile) float64 {
	var a float64
	for _, t := range tiles {
		a += t.w * t.h
	}
	return a
}

func TestSquarifyFillsTheRectangle(t *testing.T) {
	const w, h = 200.0, 100.0
	tiles := squarifyTreemap([]float64{6, 6, 4, 3, 2, 1}, w, h)

	if got, want := sumTileArea(tiles), w*h; math.Abs(got-want) > 1e-6 {
		t.Errorf("tiled area = %.4f, want %.4f (tiles must fill the rect)", got, want)
	}
}

func TestSquarifyOneTilePerPositiveWeight(t *testing.T) {
	// The two zero weights and the negative one contribute no tiles.
	tiles := squarifyTreemap([]float64{10, 0, 5, -3, 2}, 120, 80)

	if len(tiles) != 3 {
		t.Fatalf("tile count = %d, want 3 (zero/negative weights get no tile)", len(tiles))
	}
	for _, tile := range tiles {
		if w := []float64{10, 0, 5, -3, 2}[tile.index]; w <= 0 {
			t.Errorf("tile points at index %d with non-positive weight %v", tile.index, w)
		}
	}
}

func TestSquarifyTilesStayWithinBounds(t *testing.T) {
	const w, h = 300.0, 150.0
	tiles := squarifyTreemap([]float64{40, 25, 20, 8, 5, 2}, w, h)

	const eps = 1e-6
	for _, tile := range tiles {
		if tile.x < -eps || tile.y < -eps || tile.x+tile.w > w+eps || tile.y+tile.h > h+eps {
			t.Errorf("tile %+v escapes the %g×%g rect", tile, w, h)
		}
	}
}

func TestSquarifyAreaProportionalToWeight(t *testing.T) {
	// A weight twice another's should claim twice the area.
	tiles := squarifyTreemap([]float64{8, 4}, 120, 60)
	if len(tiles) != 2 {
		t.Fatalf("tile count = %d, want 2", len(tiles))
	}
	byIndex := map[int]treemapTile{}
	for _, tile := range tiles {
		byIndex[tile.index] = tile
	}
	big, small := byIndex[0].w*byIndex[0].h, byIndex[1].w*byIndex[1].h
	if ratio := big / small; math.Abs(ratio-2) > 1e-6 {
		t.Errorf("area ratio = %.4f, want 2 (weights 8:4)", ratio)
	}
}

func TestSquarifyDegenerateInputsYieldNoTiles(t *testing.T) {
	cases := []struct {
		name          string
		weights       []float64
		width, height float64
	}{
		{"all zero", []float64{0, 0}, 100, 100},
		{"empty", nil, 100, 100},
		{"zero width", []float64{1, 2}, 0, 100},
		{"zero height", []float64{1, 2}, 100, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if tiles := squarifyTreemap(c.weights, c.width, c.height); len(tiles) != 0 {
				t.Errorf("got %d tiles, want 0", len(tiles))
			}
		})
	}
}
