# Standard: UI & Architecture Conventions (this project)

> The project-specific conventions for `internal/ui` (Fyne) and the cross-layer
> architecture invariants. The general principles these build on live in
> [solid-modularity](solid-modularity.md),
> [no-magic-numbers](no-magic-numbers.md),
> [no-string-literals](no-string-literals.md), and
> [dependency-inverted-layered-seams](dependency-inverted-layered-seams.md);
> this doc owns the concrete Fyne/architecture rules.

## Architecture invariants

- **Keep the layers separate.** Collection (`internal/monitor`), storage
  (`internal/metrics` + `internal/ringbuffer`), and presentation (`internal/ui`)
  stay distinct. **UI reads from ring buffers; it never calls `gopsutil`
  directly.** (This is the layering the
  [layered-seams](dependency-inverted-layered-seams.md) standard formalizes.)
- **Process IDs are first-class identifiers.** Design shared state so cross-tab
  navigation wires up cleanly — e.g. Port → owning process in the Processes tab.
  Don't bury a PID inside a display string; carry it as a typed value (see
  [type-safety](type-safety.md)) so a row can navigate by it.
- **No HTML/CSS conventions in Fyne code.** The wireframes under
  `docs/Wireframe Designs/*.html` are visual reference only. Translate their
  intent into Fyne's canvas/widget model — there is no CSS, no DOM, no flexbox.

## UI package conventions (`internal/ui`)

### Namespaced dictionaries over scattered globals
Group related package-level resources/tokens into a single struct var so call
sites are self-documenting about origin: `font.SansRegular`, `icon.Overview`,
`palette.AccentDim`, `sizeName.MetricValue`, `status.Healthy`. Prefer this to a
flat list of `fontSansRegular`, `colorAccentDim`, … globals.

- **Watch collisions:** the color dictionary is `palette` (not `color` — that's
  `image/color`); the theme size-name dictionary is `sizeName`, kept distinct
  from the numeric spacing scale. Don't let a struct var shadow a function
  parameter (e.g. `styledText`'s param is `fontSrc`, not `font`).

### Spacing scale
Use the base-derived t-shirt scale in [`spacing.go`](../../internal/ui/spacing.go)
(`baseUnit = 4` → `spaceXS…space2XL`) for all gaps/padding/insets. **Component
dimensions get their own literal-px named consts** (don't express them as
`baseUnit` multiples), and replace a literal with a scale const **only on an
exact match**. The full nuance lives in
[no-magic-numbers](no-magic-numbers.md) — this is the project's spacing source.

### Lookup tables over large value-keyed switches
A `switch` that maps one value to another (e.g. theme `Color()`/`Size()`) reads
better as a package-level `map` literal + a default fallback — see
`themeColors` / `themeSizes` in [`theme.go`](../../internal/ui/theme.go). Keep
small, order-sensitive boolean-guard switches (e.g. `Font()` on `TextStyle`) as
switches. (OCP rationale in [solid-modularity](solid-modularity.md).)

### Assets: no `//go:embed`
Bundled fonts/icons are compiled in via `make generate`
(`tools/genassets` → gitignored `assets_gen.go`) and loaded through
`resource("fonts/…")` / `resource("icons/…")`. See **ADR-004**
([`docs/ADR.md`](../../docs/ADR.md)). New assets: drop the file under
`internal/ui/fonts/` or `icons/` and re-run `make generate`. **Do not** introduce
`//go:embed`.

### Generated files
Files carrying a `// Code generated …; DO NOT EDIT.` header are gitignored and
produced by a `make` target (`assets_gen.go` via `make generate`). **Never
hand-edit them**; change the source asset and regenerate.

## Checklist

- [ ] Does UI code reach for `gopsutil` or a `monitor` concrete? → read the ring buffer / go through the seam instead.
- [ ] Is a PID carried as a typed identifier (cross-nav-ready), not embedded in a string?
- [ ] Am I importing an HTML/CSS mental model into Fyne layout? → translate to canvas/widget.
- [ ] New package resource/token → added to the right namespaced dictionary (no loose global, no collision)?
- [ ] Value→value mapping written as a growing switch? → `map` + default.
- [ ] Added an asset without `make generate`, or hand-edited a generated file? → fix.
- [ ] Reached for `//go:embed`? → use the codegen path (ADR-004).

## Related

- [dependency-inverted-layered-seams.md](dependency-inverted-layered-seams.md) — the layering and seams behind these rules.
- [no-magic-numbers.md](no-magic-numbers.md) / [no-string-literals.md](no-string-literals.md) — the token/dictionary single-sourcing.
- [`docs/ADR.md`](../../docs/ADR.md) — ADR-004 (asset codegen), and other recorded decisions.
