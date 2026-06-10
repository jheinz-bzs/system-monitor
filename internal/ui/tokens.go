package ui

// Design-system tokens extracted from theme.go (SRP: the palette / size values
// are a separate reason to change from the Fyne theme implementation).
//
// palette, sizeName, colorNameTextSecondary, and rgb are the single source of
// truth for every color, custom size name, and secondary-text name used across
// the ui package. theme.go's themeColors / themeSizes maps reference these
// values and are the Fyne-specific mapping layer; they stay in theme.go.

import (
	"image/color"

	"fyne.io/fyne/v2"
)

// colorPalette is the design-system color dictionary. Grouping the tokens into
// a single struct var lets call sites read palette.Accent / palette.Surface2 so
// the origin is obvious cross-file. (It's named palette, not color, because
// image/color already owns that identifier.)
type colorPalette struct {
	BG           color.Color // window body / canvas
	Surface      color.Color // panels, sidebar, cards
	Surface2     color.Color // headers, nav, inputs, status bar
	Surface3     color.Color // row hover / selected
	PlotBG       color.Color // chart plot area (darker than the window body)
	Border       color.Color // panel edges, h-grid
	BorderStrong color.Color // emphasized dividers, pill outlines

	Text  color.Color // primary values, headings
	Text2 color.Color // secondary labels, table data
	Text3 color.Color // axis ticks, meta, muted captions

	Accent  color.Color // primary line, active nav, primary button
	Accent2 color.Color // hover, focus ring, jump links

	Green  color.Color // healthy / running
	Yellow color.Color // warning / elevated
	Red    color.Color // critical / stopped

	// Translucent design tokens (design-system-03). Used by charts and by
	// status pills / panels rather than by standard widgets.
	AccentLine color.Color // --sm-accent-line, ~0.32α — chart primary line
	AccentDim  color.Color // --sm-accent-dim, ~0.14α — accent fill
	GreenDim   color.Color // --sm-green-dim, 0.16α — healthy pill fill
	YellowDim  color.Color // --sm-yellow-dim, 0.16α — warning pill fill
	RedDim     color.Color // --sm-red-dim, 0.16α — critical pill fill

	// Categorical series colors (c1–c8, design-system-01) for multi-series
	// charts — per-core CPU lines, multi-line plots. Assigned in order and
	// wrapped after eight. c1 is the accent, reserved in practice for an
	// emphasized headline series, so secondary coloring starts at c2.
	Series []color.NRGBA

	// SeriesMuted is the quiet slate hue for a "remainder" series — the memory
	// chart's free band — taken from the tab-03 wireframe's Free swatch
	// (#3a4452). Muted on purpose: headroom shouldn't compete with the
	// categorical hues that mark real usage.
	SeriesMuted color.NRGBA

	// Derived shades that don't have a dedicated design token.
	DisabledButton color.Color // dimmed panel for disabled buttons
	Pressed        color.Color // between surface-3 and border-strong
	Shadow         color.Color
}

// palette holds the design-system colors. Hex values are taken verbatim from
// design-system-01-color-palette.html / the CLAUDE.md quick reference.
var palette = colorPalette{
	BG:           rgb(0x0e, 0x10, 0x14),
	Surface:      rgb(0x16, 0x1a, 0x21),
	Surface2:     rgb(0x1b, 0x21, 0x2b),
	Surface3:     rgb(0x22, 0x2a, 0x36),
	PlotBG:       rgb(0x0b, 0x0d, 0x11),
	Border:       rgb(0x26, 0x2e, 0x3a),
	BorderStrong: rgb(0x34, 0x41, 0x50),

	Text:  rgb(0xe7, 0xea, 0xf0),
	Text2: rgb(0x9a, 0xa6, 0xb6),
	Text3: rgb(0x61, 0x6d, 0x7e),

	Accent:  rgb(0x46, 0x79, 0xfa),
	Accent2: rgb(0x6e, 0x93, 0xfb),

	Green:  rgb(0x3f, 0xb8, 0x77),
	Yellow: rgb(0xd8, 0xa1, 0x34),
	Red:    rgb(0xe2, 0x56, 0x3f),

	AccentLine: color.NRGBA{R: 0x46, G: 0x79, B: 0xfa, A: 0x52},
	AccentDim:  color.NRGBA{R: 0x46, G: 0x79, B: 0xfa, A: 0x24},
	GreenDim:   color.NRGBA{R: 0x3f, G: 0xb8, B: 0x77, A: 0x29},
	YellowDim:  color.NRGBA{R: 0xd8, G: 0xa1, B: 0x34, A: 0x29},
	RedDim:     color.NRGBA{R: 0xe2, G: 0x56, B: 0x3f, A: 0x29},

	Series: []color.NRGBA{
		{R: 0x46, G: 0x79, B: 0xfa, A: 0xff}, // c1 (accent)
		{R: 0x36, G: 0xc2, B: 0xd4, A: 0xff}, // c2
		{R: 0x8b, G: 0x7c, B: 0xf6, A: 0xff}, // c3
		{R: 0xd8, G: 0x7c, B: 0xc0, A: 0xff}, // c4
		{R: 0x54, G: 0xb8, B: 0x6a, A: 0xff}, // c5
		{R: 0xd8, G: 0xa1, B: 0x34, A: 0xff}, // c6
		{R: 0xe2, G: 0x85, B: 0x6b, A: 0xff}, // c7
		{R: 0x6e, G: 0x93, B: 0xfb, A: 0xff}, // c8
	},

	SeriesMuted: rgb(0x3a, 0x44, 0x52),

	DisabledButton: rgb(0x14, 0x18, 0x1e),
	Pressed:        rgb(0x2a, 0x34, 0x42),
	Shadow:         color.NRGBA{R: 0, G: 0, B: 0, A: 0x66},
}

// sizeName groups the custom theme size names for the design-system typographic
// roles that Fyne does not name natively (see
// design-system-02-typography-icons.html). They resolve through
// monitorTheme.Size, so the typography helpers can read them via theme.Size(...)
// and stay in sync with the rest of the design system.
//
// PanelRadius is the corner radius for cards / panels (--sm-radius, 4px). It is
// distinct from the 2px chip/input radius used by InputRadius and
// SelectionRadius. Standard widgets don't query it; panel components do.
var sizeName = struct {
	MetricValue fyne.ThemeSizeName // 26px
	TableText   fyne.ThemeSizeName // 12px
	StatusPill  fyne.ThemeSizeName // 10.5px
	Meta        fyne.ThemeSizeName // 9px
	PanelRadius fyne.ThemeSizeName // 4px
}{
	MetricValue: "monitor.metricValue",
	TableText:   "monitor.tableText",
	StatusPill:  "monitor.statusPill",
	Meta:        "monitor.meta",
	PanelRadius: "monitor.panelRadius",
}

// colorNameTextSecondary exposes the design's secondary text color (text-2),
// which Fyne does not name natively. Available for widgets/resources that
// recolor via theme.NewColoredResource; nav icons recolor through
// colorizeStroke instead (see colorize.go).
const colorNameTextSecondary fyne.ThemeColorName = "monitor.textSecondary" // text-2

// rgb builds an opaque color from 8-bit RGB components.
func rgb(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

// withAlpha returns c with its alpha channel replaced by a — how translucent
// chart fills are derived from their series' full-hue stroke.
func withAlpha(c color.Color, a uint8) color.NRGBA {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	n.A = a
	return n
}
