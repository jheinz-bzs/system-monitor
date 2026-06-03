# Coding Standards

## Naming
- Packages: short, lowercase, single-word, no underscores (`monitor`, `metrics`, `ui`)
- Exported identifiers: `PascalCase`; unexported: `camelCase`
- Acronyms keep their case: `CPUUsage`, `parseURL`, `pid` → `PID` when exported
- Files: lowercase, no spaces; `_test.go` for tests, `doc.go` for package docs
- Booleans read as predicates: `isRunning`, `hasParent`, `canRender`

## Function design
- Small, single-purpose. If you can't summarize it in one sentence, split it.
- Guard clauses and early returns over nested conditionals.
- Return errors as the last value; wrap with context via `fmt.Errorf("...: %w", err)`.
- Orchestrating functions read like a story; details live in helpers.

## Idiomatic Go
- `gofmt` is non-negotiable — run `make fmt` (or `gofmt -w .`) before committing.
- `make vet` must be clean.
- Handle every error; never `_ =` a meaningful error. Don't `panic` in library code —
  return errors. `panic` is acceptable only for truly unrecoverable startup failures.
- Accept interfaces, return concrete types.
- Use the standard library before reaching for a dependency.

## Type safety
- Use the narrowest type that works for parameters and struct fields.
- Prefer named types and typed constants (`iota` enums) over bare `int`/`string`
  where the set of values is known.
- Don't pass `interface{}`/`any` where a real type fits.

## Imports
- Group: standard library → third-party → local (`github.com/josephheinz/...`),
  blank line between groups. `gofmt`/`goimports` enforces this.
- No circular imports — refactor when a cycle appears. Remove unused imports.

## Architecture conventions (this project)
- Keep collection (`internal/monitor`), storage (`internal/metrics`), and
  presentation (`internal/ui`) separate. UI code reads from ring buffers; it does
  not call gopsutil directly.
- Process IDs are first-class identifiers — design shared state so cross-tab
  navigation (e.g. Port → owning process) wires up cleanly.
- No HTML/CSS conventions in Fyne code; translate visual intent to Fyne's
  canvas/widget model.

## UI package conventions (`internal/ui`)
- **Namespaced dictionaries over scattered globals.** Group related package-level
  resources/tokens into a single struct var so call sites are self-documenting
  about origin: `font.SansRegular`, `icon.Overview`, `palette.AccentDim`,
  `sizeName.MetricValue`, `status.Healthy`. Prefer this to a flat list of
  `fontSansRegular`, `colorAccentDim`, … globals.
  - Watch for collisions: the color dictionary is `palette` (not `color` — that's
    `image/color`); the theme size-name dictionary is `sizeName`, kept distinct
    from the numeric spacing scale. Don't let a struct var shadow a function
    parameter (e.g. `styledText`'s param is `fontSrc`, not `font`).
- **Spacing uses the base-derived t-shirt scale** in `spacing.go`: `baseUnit = 4`
  (the design's 4px grid) → `spaceXS…space2XL`. Use these for all gaps / padding /
  insets — they are the project standard going forward; don't reintroduce bare
  spacing literals.
  - Component dimensions (widths, heights, fixed chrome heights) get their **own
    literal-px named consts** — do *not* express them as `baseUnit` multiples.
    Real design values aren't all on the 4px grid, and forcing them on invents
    false coupling.
  - **Exact-match only:** replace a literal with a scale const only when it
    *equals* a scale step. Never snap an off-scale value (e.g. 3px) onto the
    scale — that changes the rendered look.
- **Lookup tables over large value-keyed switches.** A `switch` that maps one
  value to another (e.g. theme `Color()`/`Size()`) reads better as a package-level
  `map` literal + a default fallback. Keep small, order-sensitive boolean-guard
  switches (e.g. `Font()` on `TextStyle`) as switches.
- **No `//go:embed`.** Bundled fonts/icons are compiled in via `make generate`
  (`tools/genassets` → gitignored `assets_gen.go`) and loaded through
  `resource("fonts/…")` / `resource("icons/…")`. See ADR-004. New assets: drop
  the file under `internal/ui/fonts/` or `icons/` and re-run `make generate`.
- **Generated files** carry a `// Code generated …; DO NOT EDIT.` header, are
  gitignored, and are produced by a `make` target — never hand-edit them.

## Things to avoid
- Magic strings and numbers — extract to named constants (design tokens and the
  spacing scale included).
- Deep nesting — refactor with guard clauses or extracted helpers.
- Premature abstraction — wait for the third example before generalizing.
- Comments that describe *what*. The code says *what*; comments say *why*.
- `//go:embed` — use the asset-codegen path instead (ADR-004).
