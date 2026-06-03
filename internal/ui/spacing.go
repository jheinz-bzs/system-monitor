package ui

// Spacing scale for the System Monitor UI.
//
// The whole spacing system is derived from a single baseUnit — the design
// system's 4px grid (design-system-03-spacing-borders-geometry.html) — so
// gaps, padding, and insets rescale from one knob and stay on-grid. These are
// the project standard for spacing/padding/gaps going forward.
//
// Only use a scale step where a value genuinely lands on the 4px grid.
// Component dimensions (widths/heights and other one-off geometry) and
// off-grid values keep their own literal-px named consts rather than being
// forced onto the scale — real design values aren't all 4px multiples, and
// inventing false relationships risks rounding drift against the
// pixel-identical contract.
//
// Declared as untyped numeric consts so they coerce to float32 (Fyne sizes) or
// int at each use site without explicit casts, exactly like the literals they
// replace.

const baseUnit = 4 // design-system 4px spacing grid

const (
	spaceXS  = baseUnit / 2 // 2
	spaceSM  = baseUnit     // 4
	spaceMD  = baseUnit * 2 // 8
	spaceLG  = baseUnit * 3 // 12
	spaceXL  = baseUnit * 4 // 16
	space2XL = baseUnit * 6 // 24
)
