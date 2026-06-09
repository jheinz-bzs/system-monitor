# Standard: SOLID & Clear Modularity

> The principles. Their concrete, project-specific application to package
> structure is the
> [dependency-inverted-layered-seams](dependency-inverted-layered-seams.md)
> standard — read that for the target package layout. This doc is how each SOLID
> letter shows up day-to-day, in Go terms.

## Principle

Each unit of code (file, type, function, package) has **one reason to change**,
depends on **abstractions rather than concretes** across boundaries, and exposes
the **smallest interface** that does the job. SOLID in Go is lighter than in
class-heavy languages — favor small interfaces, composition, and package
seams over inheritance hierarchies.

## The five, in Go, in this project

### S — Single Responsibility
A file/function changes for one reason. The canonical violation here is
[`linechart.go`](../../internal/ui/linechart.go) (~721 lines) mixing widget API,
layout geometry, raster math, and number formatting — four reasons to change.
The fix (flowmap §6 Move 2) splits it into `chart/{linechart,renderer,raster,
format}.go`.

> Day-to-day: if you can't summarize a function in one sentence, split it (see
> [function-design](function-design.md)). Orchestrators read like a story;
> details live in helpers.

### O — Open/Closed
Open to extension, closed to modification. The current
[`app.go · Run()`](../../internal/ui/app.go) violates this: adding a tab means
editing `Run`, the `liveSources` struct, and `shell`. The target is a **tab
registry** where a metric area registers a provider and the wiring is untouched
(flowmap §6 Move 3).

> Day-to-day: prefer a **lookup table + default fallback** to a growing
> value-keyed `switch`. The theme already does this — `themeColors` and
> `themeSizes` are `map` literals with a default arm, so adding a mapping is a
> new entry, not a new `case`. (Keep small order-sensitive boolean-guard
> switches like `Font()` as switches — see [ui-conventions](ui-conventions.md).)

### L — Liskov Substitution
Any implementation behind an interface must be usable without the caller knowing
which one it is. The `Collector` interface in
[`poller.go`](../../internal/monitor/poller.go) already models this: the poller
drives CPU, memory, disk, … collectors identically. The target `Source` seam
extends the same substitutability to the read/render side — any collector's
history is interchangeable behind `Source`.

> Day-to-day: a nil collector in `Run()` is handled by falling back to a
> placeholder tab, not by special-casing the type. Substitutes degrade
> gracefully; they don't leak their concrete identity.

### I — Interface Segregation
Keep interfaces tiny. `Source` is one method (`Values() []float64`); `Collector`
is small and focused. Don't grow a "manager" interface with ten methods where
callers use two.

> Day-to-day: "Accept interfaces, return concrete types" (see
> [idiomatic-go](idiomatic-go.md)). Define the interface at the *consumer*, sized
> to what the consumer needs.

### D — Dependency Inversion
High-level policy (UI) must not depend on low-level detail (collectors); both
depend on an abstraction. Today `ui` imports `monitor` directly. The fix is the
neutral `internal/series` seam both sides depend on — the heart of the
[layered-seams standard](dependency-inverted-layered-seams.md).

> Day-to-day: keep collection (`internal/monitor`), storage
> (`internal/metrics`/`ringbuffer`), and presentation (`internal/ui`) separate;
> UI reads ring buffers, never calls gopsutil. The seam package is where the two
> sides meet.

## Modularity guardrails (this project)

- **Respect the layer split**: monitor → metrics/ringbuffer → ui, meeting at
  seams. No cross-layer concrete imports.
- **No circular imports.** A cycle is the signal to extract a seam package.
- **Premature abstraction is also a violation.** Wait for the third example
  before generalizing (see [dry](dry.md)). SOLID justifies a seam that
  *exists under pressure* (the `linechart.go` and `Run()` pain points), not
  speculative interfaces.
- **Package = bounded responsibility.** New cross-cutting concept → consider a
  new small package over bloating an existing one.

## Checklist

- [ ] Does this file now have a second reason to change? → split it.
- [ ] Am I extending behavior by editing central wiring? → introduce/use a registry (OCP).
- [ ] Is this a value→value mapping written as a growing switch? → lookup table + default.
- [ ] Is the interface I'm adding the smallest the consumer needs, defined at the consumer?
- [ ] Does a high-level package import a low-level concrete? → route through a seam (DIP).
- [ ] Am I abstracting before the third real example? → stop; inline it.

## Related

- [dependency-inverted-layered-seams.md](dependency-inverted-layered-seams.md) — SOLID applied to the package graph (the target).
- [dry.md](dry.md) — modular boundaries are what make single-sourcing possible.
- [naming.md](naming.md), [function-design.md](function-design.md), [idiomatic-go.md](idiomatic-go.md), [type-safety.md](type-safety.md) — the Go-craft standards.
- [`docs/DESIGN-PRINCIPLES.md`](../DESIGN-PRINCIPLES.md).
