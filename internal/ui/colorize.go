package ui

// Stroke-aware SVG recoloring for the bundled line icons.
//
// Fyne's theme.NewColoredResource recolors an SVG via svg.Colorize, which only
// rewrites *fill* colors. Our nav glyphs are Lucide line icons — they have
// fill="none" and draw entirely with stroke="currentColor" — so the built-in
// colorizer leaves them untouched. Instead we bake the target color directly
// into the SVG by replacing the "currentColor" keyword, producing a new
// StaticResource per state (accent when active, text-2 when idle).
//
// This is safe because we control the icon source: every bundled Lucide glyph
// uses the literal token "currentColor" for its stroke.

import (
	"bytes"
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
)

// colorizeStroke returns a copy of the given SVG resource with every
// "currentColor" token replaced by c, so the line stroke renders in that color.
func colorizeStroke(src fyne.Resource, c color.Color) fyne.Resource {
	out := bytes.ReplaceAll(src.Content(), []byte("currentColor"), []byte(hexString(c)))
	return fyne.NewStaticResource(src.Name(), out)
}

// hexString formats a color as a #rrggbb string (alpha is dropped; nav icons
// are drawn fully opaque).
func hexString(c color.Color) string {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return fmt.Sprintf("#%02x%02x%02x", n.R, n.G, n.B)
}
