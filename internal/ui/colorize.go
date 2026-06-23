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

// solidFill is the fill token in the bundled Lucide glyphs: they draw as a
// stroke-only outline (fill="none"). Matched so the brand mark can turn the
// shape solid — a thin outline reads as a faint line at small icon sizes, where
// a filled mark stays bold and legible.
const solidFill = `fill="none"`

// fillShape returns a copy of the given SVG resource with its (none) fill
// replaced by c, turning a stroke-only Lucide glyph into a solid filled shape.
// Compose with colorizeStroke so the residual stroke matches the fill (otherwise
// the unresolved currentColor stroke renders as a stray dark outline). Renamed so
// the painter's name+size cache doesn't collide with the outline source.
func fillShape(src fyne.Resource, c color.Color) fyne.Resource {
	fill := fmt.Sprintf("fill=%q", hexString(c))
	out := bytes.ReplaceAll(src.Content(), []byte(solidFill), []byte(fill))
	return fyne.NewStaticResource("filled-"+src.Name(), out)
}

// colorizeStroke returns a copy of the given SVG resource with every
// "currentColor" token replaced by c, so the line stroke renders in that color.
func colorizeStroke(src fyne.Resource, c color.Color) fyne.Resource {
	hex := hexString(c)
	out := bytes.ReplaceAll(src.Content(), []byte("currentColor"), []byte(hex))
	// Prefix the color into the resource name. Fyne's painter caches rasterized
	// SVGs keyed on resource name (+ size), so two recolored copies sharing the
	// source's name would collide in the cache and the icon would never visibly
	// change color when its state flips. A color-qualified name keeps them
	// distinct (e.g. "#4679fa-overview.svg" vs "#9aa6b6-overview.svg").
	return fyne.NewStaticResource(hex+"-"+src.Name(), out)
}

// hexString formats a color as a #rrggbb string (alpha is dropped; nav icons
// are drawn fully opaque).
func hexString(c color.Color) string {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return fmt.Sprintf("#%02x%02x%02x", n.R, n.G, n.B)
}
