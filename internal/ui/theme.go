package ui

// This file defines the application's Fyne theme — the mapping layer between
// Fyne's semantic color/size names and the design-system tokens (tokens.go).
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
	theme.SizeNameText:               spaceLG, // 12; dense body/table text (13 rounded to grid)
	theme.SizeNameHeadingText:        spaceXL, // 16; page title (17 rounded to grid)
	theme.SizeNameSubHeadingText:     spaceXL, // 16; section title (14 rounded to grid)
	theme.SizeNameCaptionText:        spaceLG, // 12; panel/column labels, meta (11 rounded to grid)
	theme.SizeNamePadding:            spaceSM, // 4; --sm-1, dense, on the 4px scale
	theme.SizeNameInnerPadding:       spaceMD, // 8
	theme.SizeNameLineSpacing:        spaceSM, // 4
	theme.SizeNameSeparatorThickness: 1,       // 1px hairline border token
	theme.SizeNameInputBorder:        1,       // 1px hairline
	theme.SizeNameInputRadius:        spaceXS, // 2; industrial/sharp corners
	theme.SizeNameSelectionRadius:    spaceXS, // 2
	theme.SizeNameScrollBarRadius:    0,
	theme.SizeNameScrollBar:          spaceLG, // 12
	theme.SizeNameScrollBarSmall:     spaceSM, // 4 (3 rounded to grid)
	theme.SizeNameInlineIcon:         spaceXL, // 16 (18 rounded to grid)

	// Design-system typographic roles (Mono) not named by Fyne (grid-rounded).
	sizeName.MetricValue: space2XL, // 24; big metric readouts (26 rounded to grid)
	sizeName.TableText:   spaceLG,  // 12; table / body data
	sizeName.StatusPill:  spaceLG,  // 12; status pills (10.5 rounded to grid)
	sizeName.Meta:        spaceMD,  // 8; axis ticks, meta captions (9 rounded to grid)
	sizeName.PanelRadius: spaceSM,  // 4; --sm-radius, card / panel corners
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
