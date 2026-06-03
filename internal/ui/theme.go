package ui

// This file defines the application's Fyne theme. It translates the design
// system documented in docs/Wireframe Designs/design-system-*.html (and the
// quick-reference table in CLAUDE.md) into Fyne's theme model so that the
// standard Fyne widgets render with the project's industrial/utilitarian,
// terminal-meets-data-tooling look.
//
// See https://docs.fyne.io/faq/theme/ for the theme contract.
//
// The design system is dark-only, so the theme ignores the requested
// fyne.ThemeVariant and always returns its dark palette.
//
// Fonts: the design calls for IBM Plex Mono (everything numeric/tabular) and
// IBM Plex Sans (titles and prose). Both families are bundled and embedded in
// fonts.go; Font returns them based on fyne.TextStyle. Icon still delegates to
// Fyne's default theme.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
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

// Custom theme color name exposing the design's secondary text color (text-2),
// which Fyne does not name natively. Kept available for widgets/resources that
// recolor via theme.NewColoredResource; the nav icons are line glyphs and
// recolor through colorizeStroke instead (see colorize.go).
const colorNameTextSecondary fyne.ThemeColorName = "monitor.textSecondary" // text-2

// monitorTheme is the System Monitor Fyne theme. It satisfies fyne.Theme.
type monitorTheme struct{}

// Compile-time assertion that monitorTheme implements the theme contract.
var _ fyne.Theme = (*monitorTheme)(nil)

// newTheme returns the application theme.
func newTheme() fyne.Theme { return &monitorTheme{} }

// themeColors maps Fyne's semantic color names onto the design-system palette.
// A map-miss falls through to the default dark theme (see Color), mirroring the
// old switch's default arm.
var themeColors = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:          palette.BG,
	theme.ColorNameButton:              palette.Surface,
	theme.ColorNameDisabledButton:      palette.DisabledButton,
	theme.ColorNameDisabled:            palette.Text3,
	theme.ColorNameError:               palette.Red,
	theme.ColorNameFocus:               palette.Accent,
	theme.ColorNameForeground:          palette.Text,
	colorNameTextSecondary:             palette.Text2,
	theme.ColorNameForegroundOnError:   palette.Text,
	theme.ColorNameForegroundOnPrimary: palette.Text,
	theme.ColorNameForegroundOnSuccess: palette.BG,
	theme.ColorNameForegroundOnWarning: palette.BG,
	theme.ColorNameHeaderBackground:    palette.Surface2,
	theme.ColorNameHover:               palette.Surface3,
	theme.ColorNameHyperlink:           palette.Accent2,
	theme.ColorNameInputBackground:     palette.Surface2,
	theme.ColorNameInputBorder:         palette.Border,
	theme.ColorNameMenuBackground:      palette.Surface,
	theme.ColorNameOverlayBackground:   palette.Surface,
	theme.ColorNamePlaceHolder:         palette.Text3,
	theme.ColorNamePressed:             palette.Pressed,
	theme.ColorNamePrimary:             palette.Accent,
	theme.ColorNameScrollBar:           palette.BorderStrong,
	theme.ColorNameScrollBarBackground: palette.Surface,
	theme.ColorNameSelection:           palette.Surface3,
	theme.ColorNameSeparator:           palette.Border,
	theme.ColorNameShadow:              palette.Shadow,
	theme.ColorNameSuccess:             palette.Green,
	theme.ColorNameWarning:             palette.Yellow,
}

// Color maps Fyne's semantic color names onto the design-system palette. The
// variant is intentionally ignored: the design system defines a single dark
// theme.
func (m *monitorTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if c, ok := themeColors[name]; ok {
		return c
	}
	// Fall back to the default dark theme for any color name added in a future
	// Fyne release that we don't yet map explicitly.
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

// Font returns the bundled IBM Plex faces (see fonts.go): IBM Plex Sans for
// prose and IBM Plex Mono for monospace/data text. Bold maps to Plex Sans
// SemiBold (600) to match the design's page-title weight. Symbol glyphs keep
// Fyne's built-in symbol font.
func (m *monitorTheme) Font(style fyne.TextStyle) fyne.Resource {
	switch {
	case style.Symbol:
		return theme.DefaultTheme().Font(style)
	case style.Monospace:
		return font.MonoRegular
	case style.Bold && style.Italic:
		return font.SansSemiBoldItalic
	case style.Bold:
		return font.SansSemiBold
	case style.Italic:
		return font.SansItalic
	default:
		return font.SansRegular
	}
}

// Icon delegates to Fyne's default icon set.
func (m *monitorTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// themeSizes maps Fyne's size names onto the design system's typographic scale,
// 4px-based spacing, and sharp-cornered geometry (radii of 0–2px rather than
// Fyne's rounded defaults). A map-miss falls through to the default theme (see
// Size), mirroring the old switch's default arm.
var themeSizes = map[fyne.ThemeSizeName]float32{
	theme.SizeNameText:               13, // dense body/table text (design table data 12, labels 11)
	theme.SizeNameHeadingText:        17, // page title (Sans 17px / 600)
	theme.SizeNameSubHeadingText:     14,
	theme.SizeNameCaptionText:        11, // panel/column labels, meta
	theme.SizeNamePadding:            4,  // --sm-1; dense, snapped to the 4px scale
	theme.SizeNameInnerPadding:       8,
	theme.SizeNameLineSpacing:        4,
	theme.SizeNameSeparatorThickness: 1, // matches the 1px border token
	theme.SizeNameInputBorder:        1,
	theme.SizeNameInputRadius:        2, // industrial/sharp corners
	theme.SizeNameSelectionRadius:    2,
	theme.SizeNameScrollBarRadius:    0,
	theme.SizeNameScrollBar:          12,
	theme.SizeNameScrollBarSmall:     3,
	theme.SizeNameInlineIcon:         18,

	// Design-system typographic roles (Mono) not named by Fyne.
	sizeName.MetricValue: 26,   // big metric readouts
	sizeName.TableText:   12,   // table / body data
	sizeName.StatusPill:  10.5, // status pills
	sizeName.Meta:        9,    // axis ticks, meta captions
	sizeName.PanelRadius: 4,    // --sm-radius; card / panel corners
}

// Size maps Fyne's size names onto the design system's typographic scale,
// 4px-based spacing, and sharp-cornered geometry (radii of 0–2px rather than
// Fyne's rounded defaults).
func (m *monitorTheme) Size(name fyne.ThemeSizeName) float32 {
	if s, ok := themeSizes[name]; ok {
		return s
	}
	// Defer to the default theme for any size we don't override
	// (e.g. window-chrome sizes).
	return theme.DefaultTheme().Size(name)
}

// rgb builds an opaque color from 8-bit RGB components.
func rgb(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}
