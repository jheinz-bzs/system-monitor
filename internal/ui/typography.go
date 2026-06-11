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
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
)

// styledText builds a canvas.Text with an explicit font face, a theme-defined
// size, and a color. Because FontSource is set, the face and weight come
// directly from the given resource (TextStyle is not consulted).
func styledText(text string, fontSrc fyne.Resource, sizeToken fyne.ThemeSizeName, col color.Color) *canvas.Text {
	t := canvas.NewText(text, col)
	t.FontSource = fontSrc
	t.TextSize = theme.Size(sizeToken)
	return t
}

// --- Prose roles (IBM Plex Sans) --------------------------------------------

// newHeading returns a page title — Plex Sans SemiBold (600), 17px, primary
// text. Matches the design "Page title" role.
func newHeading(text string) *canvas.Text {
	return styledText(text, font.SansSemiBold, theme.SizeNameHeadingText, palette.Text)
}

// newSubHeading returns a section title — Plex Sans SemiBold (600), 14px,
// primary text.
func newSubHeading(text string) *canvas.Text {
	return styledText(text, font.SansSemiBold, theme.SizeNameSubHeadingText, palette.Text)
}

// --- Data roles (IBM Plex Mono) ---------------------------------------------

// newMetricValue returns a large numeric readout — Plex Mono Medium (500),
// 26px, primary text.
func newMetricValue(text string) *canvas.Text {
	return styledText(text, font.MonoMedium, sizeName.MetricValue, palette.Text)
}

// newTableText returns table / body data — Plex Mono Regular (400), 12px,
// secondary text.
func newTableText(text string) *canvas.Text {
	return styledText(text, font.MonoRegular, sizeName.TableText, palette.Text2)
}

// newColumnLabel returns an uppercase panel / column label — Plex Mono Medium
// (500), 11px (caption size), secondary text. The design's 0.06em letter
// tracking is not expressible in Fyne, so it is omitted.
func newColumnLabel(text string) *canvas.Text {
	return styledText(strings.ToUpper(text), font.MonoMedium, theme.SizeNameCaptionText, palette.Text2)
}

// newPageSubtitle returns the muted page-header subtitle (e.g. the CPU tab's
// "12 CORES · …") — Plex Mono Regular (400), 11px (caption size), UPPERCASE,
// text-3.
func newPageSubtitle(text string) *canvas.Text {
	return styledText(strings.ToUpper(text), font.MonoRegular, theme.SizeNameCaptionText, palette.Text3)
}

// newTableHeaderLabel returns a data-table column header — the column-label
// role recolored to the muted text-3 the table wireframes use for header rows
// (panel titles keep text-2 via newColumnLabel). The text is rendered as
// given: dataTable's layoutHeader owns the UPPERCASE transform, re-applying
// it on every arrange so header rewrites (sort markers) stay uppercase too.
func newTableHeaderLabel(text string) *canvas.Text {
	return styledText(text, font.MonoMedium, theme.SizeNameCaptionText, palette.Text3)
}

// newMeta returns muted meta / axis text — Plex Mono Regular (400), 9px, text-3.
func newMeta(text string) *canvas.Text {
	return styledText(text, font.MonoRegular, sizeName.Meta, palette.Text3)
}

// --- Status text ------------------------------------------------------------

// statusKind selects the semantic color for a status readout.
type statusKind int

// status namespaces the statusKind values so call sites read status.Healthy /
// status.Critical and the origin is obvious cross-file.
var status = struct{ Healthy, Warning, Critical, Neutral statusKind }{
	Healthy: 0, Warning: 1, Critical: 2, Neutral: 3,
}

// statusColor maps a statusKind onto the design palette.
func statusColor(kind statusKind) color.Color {
	switch kind {
	case status.Healthy:
		return palette.Green
	case status.Warning:
		return palette.Yellow
	case status.Critical:
		return palette.Red
	default:
		return palette.Text2
	}
}

// newStatusText returns a status label — Plex Mono Regular (400), 10.5px,
// colored by kind. This is the text of a status pill; the pill's
// background/outline chrome is added by newStatusPill.
func newStatusText(text string, kind statusKind) *canvas.Text {
	return styledText(text, font.MonoRegular, sizeName.StatusPill, statusColor(kind))
}

// Pill chrome geometry. The 2px radius matches the design's chip/input radius
// (theme InputRadius); the insets give the text a little breathing room.
const (
	pillHPad   = spaceMD // 8
	pillVPad   = spaceSM // 4; vertical pad (3 rounded to grid)
	pillRadius = spaceXS // 2; corner radius (already on-scale)
)

// pillFill maps a statusKind onto its translucent (0.16α) background fill.
// Neutral has no dedicated design token, so it falls back to surface-3.
func pillFill(kind statusKind) color.Color {
	switch kind {
	case status.Healthy:
		return palette.GreenDim
	case status.Warning:
		return palette.YellowDim
	case status.Critical:
		return palette.RedDim
	default:
		return palette.Surface3
	}
}

// newStatusPill wraps a status label in pill chrome: a kind-tinted translucent
// fill, a 1px border-strong outline, and rounded corners. The returned object
// hugs its text — place it in an HBox (or similar) so a stretching parent
// layout doesn't widen it to fill the row.
func newStatusPill(text string, kind statusKind) fyne.CanvasObject {
	bg := canvas.NewRectangle(pillFill(kind))
	bg.StrokeColor = palette.BorderStrong
	bg.StrokeWidth = 1
	bg.CornerRadius = pillRadius

	label := newStatusText(text, kind)
	padded := container.New(
		layout.NewCustomPaddedLayout(pillVPad, pillVPad, pillHPad, pillHPad), label)
	return container.NewStack(bg, padded)
}
