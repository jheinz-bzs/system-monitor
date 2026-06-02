package ui

// Overview tab content.
//
// Until the live 2×4 metric-panel grid (see CLAUDE.md / tab-01-overview) is
// wired up, the Overview tab doubles as an on-screen design-system reference:
// one instance of every typographic role and a swatch for every palette color.
// This lets the rendered IBM Plex faces and the exact palette be eyeballed
// against the wireframes without spinning up live data.
//
// Every swatch is stroked with a 1px outline so the darkest fills — including
// the swatch that is literally the window background — stay visible against the
// canvas instead of disappearing into it.

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
)

// newOverview builds the Overview tab: a scrollable column with a typography
// catalog and a color-swatch grid.
func newOverview() fyne.CanvasObject {
	body := container.New(layout.NewCustomPaddedVBoxLayout(24),
		newHeading("Overview"),
		typographyShowcase(),
		colorShowcase(),
	)
	padded := container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24), body)
	return container.NewVScroll(padded)
}

// typographyShowcase renders one labeled instance of each design-system
// typographic role exposed by typography.go.
func typographyShowcase() fyne.CanvasObject {
	samples := []struct {
		role   string
		sample fyne.CanvasObject
	}{
		{"Page title — Sans SemiBold 17", newHeading("System Monitor")},
		{"Section title — Sans SemiBold 14", newSubHeading("Per-Core Utilization")},
		{"Metric value — Mono Medium 26", newMetricValue("42.7%")},
		{"Table data — Mono Regular 12", newTableText("chrome.exe    1284    3.2%")},
		{"Column label — Mono Medium 11 UPPER", newColumnLabel("Process")},
		{"Meta / axis — Mono Regular 9", newMeta("-60s    -30s    now")},
		{"Status pill — healthy", container.NewHBox(newStatusPill("RUNNING", statusHealthy))},
		{"Status pill — warning", container.NewHBox(newStatusPill("ELEVATED", statusWarning))},
		{"Status pill — critical", container.NewHBox(newStatusPill("STOPPED", statusCritical))},
		{"Status pill — neutral", container.NewHBox(newStatusPill("IDLE", statusNeutral))},
	}

	rows := make([]fyne.CanvasObject, 0, len(samples)+1)
	rows = append(rows, newColumnLabel("Typography"))
	for _, s := range samples {
		rows = append(rows, container.New(layout.NewCustomPaddedVBoxLayout(2),
			newMeta(s.role),
			s.sample,
		))
	}
	return container.New(layout.NewCustomPaddedVBoxLayout(12), rows...)
}

// colorShowcase renders a swatch for every palette token defined in theme.go.
func colorShowcase() fyne.CanvasObject {
	swatches := []struct {
		name string
		col  color.Color
	}{
		{"bg", colorBG},
		{"surface", colorSurface},
		{"surface-2", colorSurface2},
		{"surface-3", colorSurface3},
		{"border", colorBorder},
		{"border-strong", colorBorderStrong},
		{"text", colorText},
		{"text-2", colorText2},
		{"text-3", colorText3},
		{"accent", colorAccent},
		{"accent-2", colorAccent2},
		{"green", colorGreen},
		{"yellow", colorYellow},
		{"red", colorRed},
		{"accent-line", colorAccentLine},
		{"accent-dim", colorAccentDim},
		{"green-dim", colorGreenDim},
		{"yellow-dim", colorYellowDim},
		{"red-dim", colorRedDim},
		{"disabled-btn", colorDisabledButton},
		{"pressed", colorPressed},
		{"shadow", colorShadow},
	}

	cells := make([]fyne.CanvasObject, 0, len(swatches))
	for _, s := range swatches {
		cells = append(cells, swatchCell(s.name, s.col))
	}
	grid := container.NewGridWrap(fyne.NewSize(120, 78), cells...)

	return container.New(layout.NewCustomPaddedVBoxLayout(12),
		newColumnLabel("Color Palette"),
		grid,
	)
}

// swatchCell is a single color sample: a stroked, rounded chip above its token
// name and hex value. The outline keeps near-background fills visible.
func swatchCell(name string, col color.Color) fyne.CanvasObject {
	chip := canvas.NewRectangle(col)
	chip.StrokeColor = colorText3
	chip.StrokeWidth = 1
	chip.CornerRadius = theme.Size(sizeNamePanelRadius)
	chip.SetMinSize(fyne.NewSize(0, 40))

	return container.New(layout.NewCustomPaddedVBoxLayout(2),
		chip,
		newMeta(name),
		newMeta(swatchHex(col)),
	)
}

// swatchHex formats a swatch color as #rrggbb, appending an opacity percentage
// for translucent tokens so the alpha isn't silently dropped from the label.
func swatchHex(c color.Color) string {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	if n.A == 0xff {
		return fmt.Sprintf("#%02x%02x%02x", n.R, n.G, n.B)
	}
	return fmt.Sprintf("#%02x%02x%02x %d%%", n.R, n.G, n.B, (int(n.A)*100+127)/255)
}
