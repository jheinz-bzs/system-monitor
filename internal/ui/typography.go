package ui

// Typography helpers for the System Monitor UI.
//
// These constructors produce text objects pre-styled to the design-system
// roles documented in CLAUDE.md / design-system-02-typography-icons.html, so
// call sites can write newHeading("Overview") instead of hand-setting font,
// size, and color every time.
//
// Each helper pins its typeface explicitly via canvas.Text.FontSource (the
// bundled IBM Plex faces from fonts.go) rather than relying on the theme's
// fyne.TextStyle resolution, so the face/weight matches the design system
// exactly: IBM Plex Sans for prose, IBM Plex Mono for data/labels/values, at
// the weight each role specifies. Sizes are still read from the active theme so
// the values stay centralized in theme.go.

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

// styledText builds a canvas.Text with an explicit font face, a theme-defined
// size, and a color. Because FontSource is set, the face and weight come
// directly from the given resource (TextStyle is not consulted).
func styledText(text string, font fyne.Resource, sizeName fyne.ThemeSizeName, col color.Color) *canvas.Text {
	t := canvas.NewText(text, col)
	t.FontSource = font
	t.TextSize = theme.Size(sizeName)
	return t
}

// --- Prose roles (IBM Plex Sans) --------------------------------------------

// newHeading returns a page title — Plex Sans SemiBold (600), 17px, primary
// text. Matches the design "Page title" role.
func newHeading(text string) *canvas.Text {
	return styledText(text, fontSansSemiBold, theme.SizeNameHeadingText, colorText)
}

// newSubHeading returns a section title — Plex Sans SemiBold (600), 14px,
// primary text.
func newSubHeading(text string) *canvas.Text {
	return styledText(text, fontSansSemiBold, theme.SizeNameSubHeadingText, colorText)
}

// --- Data roles (IBM Plex Mono) ---------------------------------------------

// newMetricValue returns a large numeric readout — Plex Mono Medium (500),
// 26px, primary text.
func newMetricValue(text string) *canvas.Text {
	return styledText(text, fontMonoMedium, sizeNameMetricValue, colorText)
}

// newTableText returns table / body data — Plex Mono Regular (400), 12px,
// secondary text.
func newTableText(text string) *canvas.Text {
	return styledText(text, fontMonoRegular, sizeNameTableText, colorText2)
}

// newColumnLabel returns an uppercase panel / column label — Plex Mono Medium
// (500), 11px (caption size), secondary text. The design's 0.06em letter
// tracking is not expressible in Fyne, so it is omitted.
func newColumnLabel(text string) *canvas.Text {
	return styledText(strings.ToUpper(text), fontMonoMedium, theme.SizeNameCaptionText, colorText2)
}

// newMeta returns muted meta / axis text — Plex Mono Regular (400), 9px, text-3.
func newMeta(text string) *canvas.Text {
	return styledText(text, fontMonoRegular, sizeNameMeta, colorText3)
}

// --- Status text ------------------------------------------------------------

// statusKind selects the semantic color for a status readout.
type statusKind int

const (
	statusHealthy  statusKind = iota // green  — running / healthy
	statusWarning                    // yellow — elevated / warning
	statusCritical                   // red    — stopped / critical
	statusNeutral                    // muted  — unknown / idle
)

// statusColor maps a statusKind onto the design palette.
func statusColor(kind statusKind) color.Color {
	switch kind {
	case statusHealthy:
		return colorGreen
	case statusWarning:
		return colorYellow
	case statusCritical:
		return colorRed
	default:
		return colorText2
	}
}

// newStatusText returns a status label — Plex Mono Regular (400), 10.5px,
// colored by kind. This is the text of a status pill; the pill's
// background/outline chrome belongs in a dedicated component and can wrap this
// when needed.
func newStatusText(text string, kind statusKind) *canvas.Text {
	return styledText(text, fontMonoRegular, sizeNameStatusPill, statusColor(kind))
}
