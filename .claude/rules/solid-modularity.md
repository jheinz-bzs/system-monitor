# Rule: SOLID & Clear Modularity

Full doc: [docs/conventions/solid-modularity.md](../../docs/conventions/solid-modularity.md)

**MUST**
- **S** — Keep one reason to change per file/function. Split when a second
  appears (the `linechart.go` four-job split is the model).
- **O** — Extend behavior by adding a registry entry / lookup-table entry, not by
  editing central wiring. Prefer `map` + default fallback over a growing
  value-keyed `switch` (see `themeColors`/`themeSizes`).
- **L** — Implementations behind an interface must be substitutable; degrade via
  fallback (nil collector → placeholder), never by type-checking the concrete.
- **I** — Keep interfaces tiny (`Source` is one method). Define them at the consumer.
- **D** — High-level packages depend on abstractions, not concretes. Respect the
  monitor → metrics/ringbuffer → ui layering; UI never calls gopsutil.

**MUST NOT**
- Add a cross-layer import of a concrete type (route through a seam package).
- Introduce a circular import (extract a seam instead).
- Abstract before the third real example (premature abstraction is a violation too).

Self-check: *Does this file now change for two reasons? Am I editing wiring to
add a feature? Does a high-level package reach into a low-level concrete?*
