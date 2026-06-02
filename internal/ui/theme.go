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

// Design-system color palette. Hex values are taken verbatim from
// design-system-01-color-palette.html / the CLAUDE.md quick reference.
var (
	colorBG           = rgb(0x0e, 0x10, 0x14) // window body / canvas
	colorSurface      = rgb(0x16, 0x1a, 0x21) // panels, sidebar, cards
	colorSurface2     = rgb(0x1b, 0x21, 0x2b) // headers, nav, inputs, status bar
	colorSurface3     = rgb(0x22, 0x2a, 0x36) // row hover / selected
	colorBorder       = rgb(0x26, 0x2e, 0x3a) // panel edges, h-grid
	colorBorderStrong = rgb(0x34, 0x41, 0x50) // emphasized dividers, pill outlines

	colorText  = rgb(0xe7, 0xea, 0xf0) // primary values, headings
	colorText2 = rgb(0x9a, 0xa6, 0xb6) // secondary labels, table data
	colorText3 = rgb(0x61, 0x6d, 0x7e) // axis ticks, meta, muted captions

	colorAccent  = rgb(0x46, 0x79, 0xfa) // primary line, active nav, primary button
	colorAccent2 = rgb(0x6e, 0x93, 0xfb) // hover, focus ring, jump links

	colorGreen  = rgb(0x3f, 0xb8, 0x77) // healthy / running
	colorYellow = rgb(0xd8, 0xa1, 0x34) // warning / elevated
	colorRed    = rgb(0xe2, 0x56, 0x3f) // critical / stopped

	// Derived shades that don't have a dedicated design token.
	colorDisabledButton = rgb(0x14, 0x18, 0x1e) // dimmed panel for disabled buttons
	colorPressed        = rgb(0x2a, 0x34, 0x42) // between surface-3 and border-strong
	colorShadow         = color.NRGBA{R: 0, G: 0, B: 0, A: 0x66}
)

// Custom theme size names for the design-system typographic roles that Fyne
// does not name natively (see design-system-02-typography-icons.html). They
// resolve through monitorTheme.Size, so the typography helpers can read them
// via theme.Size(...) and stay in sync with the rest of the design system.
const (
	sizeNameMetricValue fyne.ThemeSizeName = "monitor.metricValue" // 26px
	sizeNameTableText   fyne.ThemeSizeName = "monitor.tableText"   // 12px
	sizeNameStatusPill  fyne.ThemeSizeName = "monitor.statusPill"  // 10.5px
	sizeNameMeta        fyne.ThemeSizeName = "monitor.meta"        // 9px
)

// monitorTheme is the System Monitor Fyne theme. It satisfies fyne.Theme.
type monitorTheme struct{}

// Compile-time assertion that monitorTheme implements the theme contract.
var _ fyne.Theme = (*monitorTheme)(nil)

// newTheme returns the application theme.
func newTheme() fyne.Theme { return &monitorTheme{} }

// Color maps Fyne's semantic color names onto the design-system palette. The
// variant is intentionally ignored: the design system defines a single dark
// theme.
func (m *monitorTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colorBG
	case theme.ColorNameButton:
		return colorSurface
	case theme.ColorNameDisabledButton:
		return colorDisabledButton
	case theme.ColorNameDisabled:
		return colorText3
	case theme.ColorNameError:
		return colorRed
	case theme.ColorNameFocus:
		return colorAccent
	case theme.ColorNameForeground:
		return colorText
	case theme.ColorNameForegroundOnError:
		return colorText
	case theme.ColorNameForegroundOnPrimary:
		return colorText
	case theme.ColorNameForegroundOnSuccess:
		return colorBG
	case theme.ColorNameForegroundOnWarning:
		return colorBG
	case theme.ColorNameHeaderBackground:
		return colorSurface2
	case theme.ColorNameHover:
		return colorSurface3
	case theme.ColorNameHyperlink:
		return colorAccent2
	case theme.ColorNameInputBackground:
		return colorSurface2
	case theme.ColorNameInputBorder:
		return colorBorder
	case theme.ColorNameMenuBackground:
		return colorSurface
	case theme.ColorNameOverlayBackground:
		return colorSurface
	case theme.ColorNamePlaceHolder:
		return colorText3
	case theme.ColorNamePressed:
		return colorPressed
	case theme.ColorNamePrimary:
		return colorAccent
	case theme.ColorNameScrollBar:
		return colorBorderStrong
	case theme.ColorNameScrollBarBackground:
		return colorSurface
	case theme.ColorNameSelection:
		return colorSurface3
	case theme.ColorNameSeparator:
		return colorBorder
	case theme.ColorNameShadow:
		return colorShadow
	case theme.ColorNameSuccess:
		return colorGreen
	case theme.ColorNameWarning:
		return colorYellow
	default:
		// Fall back to the default dark theme for any color name added in a
		// future Fyne release that we don't yet map explicitly.
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
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
		return fontMonoRegular
	case style.Bold && style.Italic:
		return fontSansSemiBoldItalic
	case style.Bold:
		return fontSansSemiBold
	case style.Italic:
		return fontSansItalic
	default:
		return fontSansRegular
	}
}

// Icon delegates to Fyne's default icon set.
func (m *monitorTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size maps Fyne's size names onto the design system's typographic scale,
// 4px-based spacing, and sharp-cornered geometry (radii of 0–2px rather than
// Fyne's rounded defaults).
func (m *monitorTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 13 // dense body/table text (design table data 12, labels 11)
	case theme.SizeNameHeadingText:
		return 17 // page title (Sans 17px / 600)
	case theme.SizeNameSubHeadingText:
		return 14
	case theme.SizeNameCaptionText:
		return 11 // panel/column labels, meta
	case theme.SizeNamePadding:
		return 6 // dense, on the 4px scale but with a little breathing room
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameLineSpacing:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1 // matches the 1px border token
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameInputRadius:
		return 2 // industrial/sharp corners
	case theme.SizeNameSelectionRadius:
		return 2
	case theme.SizeNameScrollBarRadius:
		return 0
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameScrollBarSmall:
		return 3
	case theme.SizeNameInlineIcon:
		return 18

	// Design-system typographic roles (Mono) not named by Fyne.
	case sizeNameMetricValue:
		return 26 // big metric readouts
	case sizeNameTableText:
		return 12 // table / body data
	case sizeNameStatusPill:
		return 10.5 // status pills
	case sizeNameMeta:
		return 9 // axis ticks, meta captions

	default:
		// Defer to the default theme for any size we don't override
		// (e.g. window-chrome sizes).
		return theme.DefaultTheme().Size(name)
	}
}

// rgb builds an opaque color from 8-bit RGB components.
func rgb(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}
