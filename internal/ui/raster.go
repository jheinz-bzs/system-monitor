package ui

// Low-level vector rasterization helpers extracted from linechart.go (SRP:
// pure geometry with no Fyne widget dependency — independently testable).

import (
	"math"

	"fyne.io/fyne/v2"
	"golang.org/x/image/vector"
)

// strokePolyline adds a filled stroke for pts to the rasterizer: a quad per
// segment plus a disc at every vertex for round joins and caps. All sub-paths
// are wound consistently so the rasterizer unions them (rather than cancelling
// overlaps), which is what keeps the opacity flat along the line.
func strokePolyline(ras *vector.Rasterizer, pts []fyne.Position, sx, sy, width float32) {
	hw := width / 2
	at := func(i int) (float32, float32) { return pts[i].X * sx, pts[i].Y * sy }

	for i := 0; i < len(pts)-1; i++ {
		x0, y0 := at(i)
		x1, y1 := at(i + 1)
		dx, dy := x1-x0, y1-y0
		l := float32(math.Hypot(float64(dx), float64(dy)))
		if l == 0 {
			continue
		}
		// Unit normal and along-segment vector, each scaled to the half-width.
		nx, ny := -dy/l*hw, dx/l*hw
		ex, ey := dx/l*hw, dy/l*hw // extend ends so quads overlap at joints
		addPoly(ras, [][2]float32{
			{x0 - ex + nx, y0 - ey + ny}, {x1 + ex + nx, y1 + ey + ny},
			{x1 + ex - nx, y1 + ey - ny}, {x0 - ex - nx, y0 - ey - ny},
		})
	}
	for i := range pts {
		cx, cy := at(i)
		addDisc(ras, cx, cy, hw)
	}
}

// fillPolygon adds the closed polygon pts (widget DP) to the rasterizer,
// scaled to device pixels — the fill counterpart of strokePolyline, used for
// stacked-area bands.
func fillPolygon(ras *vector.Rasterizer, pts []fyne.Position, sx, sy float32) {
	p := make([][2]float32, len(pts))
	for i, pt := range pts {
		p[i] = [2]float32{pt.X * sx, pt.Y * sy}
	}
	addPoly(ras, p)
}

// addPoly adds a closed polygon, normalizing it to a consistent (CCW) winding
// so overlapping sub-paths union under the non-zero rule instead of cancelling.
func addPoly(ras *vector.Rasterizer, p [][2]float32) {
	if len(p) < 3 {
		return
	}
	if signedArea(p) < 0 {
		for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
			p[i], p[j] = p[j], p[i]
		}
	}
	ras.MoveTo(p[0][0], p[0][1])
	for i := 1; i < len(p); i++ {
		ras.LineTo(p[i][0], p[i][1])
	}
	ras.ClosePath()
}

// addDisc adds a 12-gon approximating a filled circle of radius rad at (cx, cy).
func addDisc(ras *vector.Rasterizer, cx, cy, rad float32) {
	const sides = 12
	p := make([][2]float32, sides)
	for k := 0; k < sides; k++ {
		a := 2 * math.Pi * float64(k) / sides
		p[k] = [2]float32{cx + rad*float32(math.Cos(a)), cy + rad*float32(math.Sin(a))}
	}
	addPoly(ras, p)
}

// signedArea returns twice the signed area of polygon p (shoelace); positive is
// counter-clockwise in screen space.
func signedArea(p [][2]float32) float32 {
	var a float32
	for i := range p {
		j := (i + 1) % len(p)
		a += p[i][0]*p[j][1] - p[j][0]*p[i][1]
	}
	return a
}
