# Standard: No Magic Numbers

> One of the cross-cutting standards. Pairs with
> [no-string-literals](no-string-literals.md); the spacing-scale and color-token
> mechanics it relies on are owned by [ui-conventions](ui-conventions.md).

## Principle

A number that has **units, identity, a design-system origin, or a non-obvious
meaning** must be a named constant (or come from the spacing scale / design
tokens). The reader should learn *what the number means* from its name, and the
value should have exactly one place to change.

## What counts as a violation

- **Design-system values inline**: a `12`, `16`, `26`, or a color hex typed into
  a widget instead of pulled from the spacing scale / `palette` / `sizeName`.
- **Durations and cadences** as bare numbers (`time.Second` use, poll intervals,
  history capacity).
- **Thresholds** (healthy/warning/critical cutoffs), **limits** (top-N process
  counts, ring-buffer capacity), **axis tick counts**, **opacity/alpha** values.
- **Any repeated numeric literal** — repetition means it's a concept, name it.

## Allowed — not violations

- **The identity values `0` and `1`** in their obvious roles: indices, `len == 0`
  guards, increments, `A: 0xff` (fully opaque). These are idiom, not magic.
- **Numbers that are self-evidently their own meaning at a single site** — e.g.
  dividing by `2` to find a midpoint, `i+1`.
- **Small loop bounds tied directly to an adjacent named thing.**

When unsure: *If this value changed, would someone need to know what it
represents to change it safely?* If yes, name it.

## How it applies in this project

This codebase has a worked-out system — use it, don't reinvent it.

- **Spacing comes from the base-derived scale**, not literals.
  [`spacing.go`](../../internal/ui/spacing.go) derives everything from one knob:

  ```go
  const baseUnit = 4 // design-system 4px grid
  const ( spaceXS = baseUnit/2; spaceSM = baseUnit; spaceMD = baseUnit*2; … )
  ```

  Use `spaceMD`/`spaceLG`/… for **all** gaps, padding, and insets.

- **Two-rule nuance from `spacing.go` (do not violate):**
  1. **Component dimensions get their own literal-px named consts** — widths,
     heights, fixed chrome heights (title bar 38, tab bar 40, row ~29). These are
     *not* expressed as `baseUnit` multiples, because real design values aren't
     all on the 4px grid and forcing them invents false coupling. Name them; don't
     scale them.
  2. **Exact-match only.** Replace a literal with a scale const only when it
     *equals* a scale step. Never snap an off-scale value (e.g. `3`) onto the
     scale — that changes the rendered pixels. The theme even documents its
     rounding (`13 rounded to grid` → `spaceLG`); honor those decisions, don't
     re-round.

- **Colors are tokens, never inline hex.** [`theme.go`](../../internal/ui/theme.go)
  holds every color in the `palette` struct var, built from `rgb(0x46,0x79,0xfa)`
  helpers and `color.NRGBA{…, A: 0x52}` for alpha tokens. Widgets read
  `palette.Accent`, `palette.AccentLine`; they never type a hex triplet inline.
  The categorical series colors live in `palette.Series` and wrap after 8.

- **Cadence and capacity are named and cross-referenced.**
  [`app.go`](../../internal/ui/app.go) defines:

  ```go
  const pollInterval = time.Second // matches metrics.HistoryCapacity resolution
  ```

  Note the comment ties it to `metrics.HistoryCapacity` — the two are one concept
  (1s resolution, ~60 samples). Keep that relationship explicit; never bury a
  raw `60` or `time.Second` at a use site.

## Checklist

- [ ] Does this number have units or a design origin? → named const / scale / token.
- [ ] Is it a gap/padding/inset equal to a scale step? → use the `space*` const.
- [ ] Is it a component dimension or off-grid value? → its own literal-px named const (do not force onto the scale).
- [ ] Is it a duration, threshold, limit, or alpha? → name it; cross-reference related consts.
- [ ] Is it just `0`/`1`/a local midpoint at one site? → fine inline.

## Related

- [no-string-literals.md](no-string-literals.md) — the string twin of this rule.
- [dependency-inverted-layered-seams.md](dependency-inverted-layered-seams.md) — Move 2 pulls tokens into `theme/tokens.go`, the single home for these values.
