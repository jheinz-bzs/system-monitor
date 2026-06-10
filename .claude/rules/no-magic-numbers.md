# Rule: No Magic Numbers

Full doc: [docs/conventions/no-magic-numbers.md](../../docs/conventions/no-magic-numbers.md)

**MUST**
- Name any number with units, identity, a design origin, or non-obvious meaning
  (durations, thresholds, limits, tick counts, alpha values).
- Use the `space*` scale (`spacing.go`) for gaps/padding/insets **only when the
  value equals a scale step** (exact-match).
- Give component dimensions and off-grid values their **own literal-px named
  consts** — do NOT express them as `baseUnit` multiples.
- Read colors from `palette` (`palette.Accent`, `palette.AccentLine`, `palette.Series`);
  build them with the `rgb(...)`/`color.NRGBA` helpers in one place.
- Keep related consts cross-referenced (e.g. `pollInterval` ↔ `metrics.HistoryCapacity`).

**MUST NOT**
- Inline a hex color, a `12`/`16`/`26`, or a `time.Second`/`60` at a widget/use site.
- Snap an off-scale value (e.g. `3px`) onto the spacing scale — it changes rendered pixels.

**Allowed (do not flag)**
- `0`/`1` in obvious roles (indices, `len==0`, increments, `A: 0xff`); a local
  `/2` midpoint at a single site.

Self-check: *If this value changed, would someone need to know what it represents
to change it safely?* If yes → name it.
