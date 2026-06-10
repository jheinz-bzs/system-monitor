package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// Tapping a toggleChip must flip its state and report each new state through
// onChange — off on the first tap (chips start on here), back on for the
// second.
func TestToggleChipTapFlipsAndNotifies(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	var got []bool
	chip := newToggleChip(labelLegendPerCore, palette.Series[perCoreSwatchSeries], true,
		func(on bool) { got = append(got, on) })

	w := test.NewWindow(chip)
	defer w.Close()

	test.Tap(chip)
	test.Tap(chip)

	want := []bool{false, true}
	if len(got) != len(want) {
		t.Fatalf("onChange fired %d times, want %d", len(got), len(want))
	}
	for i, on := range want {
		if got[i] != on {
			t.Errorf("tap %d reported %v, want %v", i+1, got[i], on)
		}
	}
}

// A nil onChange must not panic — the chip is usable as purely visual state.
func TestToggleChipNilOnChange(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	chip := newToggleChip(labelLegendPerCore, palette.Series[perCoreSwatchSeries], true, nil)
	w := test.NewWindow(chip)
	defer w.Close()

	test.Tap(chip)
}
