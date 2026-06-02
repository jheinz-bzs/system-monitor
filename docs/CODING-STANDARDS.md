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

## Things to avoid
- Magic strings and numbers — extract to named constants (design tokens included).
- Deep nesting — refactor with guard clauses or extracted helpers.
- Premature abstraction — wait for the third example before generalizing.
- Comments that describe *what*. The code says *what*; comments say *why*.
