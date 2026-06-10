# Standard: Idiomatic Go

## Principle

Write Go the way the Go team and `gofmt` write it. Idiom isn't taste here — it's
the shared baseline that keeps the codebase legible and the tooling green. When
in doubt, prefer the standard library and the obvious idiom over cleverness.

## Rules

### Formatting & vet
- **`gofmt` is non-negotiable** — run `make fmt` (or `gofmt -w .`) before
  committing. Don't hand-format.
- **`make vet` must be clean.** Treat a vet finding as a build failure.

### Error handling
- **Handle every error.** Never `_ =` a meaningful error.
- **Don't `panic` in library code** — return errors. `panic` is acceptable only
  for truly unrecoverable startup failures.
- Wrap with context and `%w` (detail in [function-design](function-design.md)).

### Interfaces & types
- **Accept interfaces, return concrete types.** Take the narrow interface you
  need as a parameter; hand back the concrete thing you built.
- Define the interface at the **consumer**, sized to what the consumer uses
  (ISP — see [solid-modularity](solid-modularity.md)).

### Dependencies
- **Use the standard library before reaching for a dependency.** A new module is
  a maintenance and supply-chain cost; justify it.

### Imports
- **Group**: standard library → third-party → local
  (`github.com/josephheinz/...`), with a blank line between groups.
  `gofmt`/`goimports` enforces this.
- **No circular imports** — a cycle is the signal to extract a seam package (see
  [layered-seams](dependency-inverted-layered-seams.md)). **Remove unused imports.**

## How it applies in this project

- [`app.go`](../../internal/ui/app.go) shows the three-group import block (stdlib
  `context`/`time` → third-party `fyne.io/...` → local
  `github.com/josephheinz/system-monitor/internal/monitor`).
- `Run()` returns concrete types and the collectors satisfy the small `Collector`
  / `Source` interfaces — "accept interfaces, return concrete types" in practice.
- Build/vet go through the Makefile because Fyne needs CGO and the generated
  assets (`make generate`) — see [ui-conventions](ui-conventions.md) and
  [`docs/ADR.md`](../../docs/ADR.md) (ADR-004). A bare `go vet` on a fresh clone
  fails until `make generate` runs.
- Data comes from `gopsutil` (a justified third-party dep for cross-platform
  system stats); UI uses Fyne. Everything else leans on the stdlib.

## Checklist

- [ ] `make fmt` run, `make vet` clean?
- [ ] Every error handled (no silent `_ =` of a meaningful error)?
- [ ] No `panic` in library code (only unrecoverable startup)?
- [ ] Params are interfaces, returns are concrete types?
- [ ] Imports grouped (stdlib / third-party / local), none unused, no cycle?
- [ ] Could the stdlib do this instead of a new dependency?

## Related

- [function-design.md](function-design.md) — error wrapping and guard clauses.
- [type-safety.md](type-safety.md) — narrow/named types.
- [solid-modularity.md](solid-modularity.md) — small consumer-defined interfaces, no cycles.
