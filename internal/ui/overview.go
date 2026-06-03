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
	body := container.New(
		layout.NewCustomPaddedVBoxLayout(24),
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
		{"Status pill — healthy", container.NewHBox(newStatusPill("RUNNING", status.Healthy))},
		{"Status pill — warning", container.NewHBox(newStatusPill("ELEVATED", status.Warning))},
		{"Status pill — critical", container.NewHBox(newStatusPill("STOPPED", status.Critical))},
		{"Status pill — neutral", container.NewHBox(newStatusPill("IDLE", status.Neutral))},
	}

	rows := make([]fyne.CanvasObject, 0, len(samples)+1)
	rows = append(rows, newColumnLabel("Typography"))
	for _, s := range samples {
		rows = append(
			rows,
			container.New(layout.NewCustomPaddedVBoxLayout(2),
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
		{"bg", palette.BG},
		{"surface", palette.Surface},
		{"surface-2", palette.Surface2},
		{"surface-3", palette.Surface3},
		{"border", palette.Border},
		{"border-strong", palette.BorderStrong},
		{"text", palette.Text},
		{"text-2", palette.Text2},
		{"text-3", palette.Text3},
		{"accent", palette.Accent},
		{"accent-2", palette.Accent2},
		{"green", palette.Green},
		{"yellow", palette.Yellow},
		{"red", palette.Red},
		{"accent-line", palette.AccentLine},
		{"accent-dim", palette.AccentDim},
		{"green-dim", palette.GreenDim},
		{"yellow-dim", palette.YellowDim},
		{"red-dim", palette.RedDim},
		{"disabled-btn", palette.DisabledButton},
		{"pressed", palette.Pressed},
		{"shadow", palette.Shadow},
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
	chip.StrokeColor = palette.Text3
	chip.StrokeWidth = 1
	chip.CornerRadius = theme.Size(sizeName.PanelRadius)
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
