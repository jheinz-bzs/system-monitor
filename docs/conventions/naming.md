# Standard: Naming

## Principle

Names carry the meaning the code can't. Follow Go community conventions exactly —
they're what make a Go file readable to any Go developer — and let a name say
what a thing *is* (nouns for values, predicates for booleans, verbs for
functions).

## Rules

- **Packages**: short, lowercase, single-word, no underscores —
  `monitor`, `metrics`, `ui`, `ringbuffer`. The package name is part of every
  call site (`monitor.NewCPUCollector`), so don't stutter (`monitor.Monitor`).
- **Exported identifiers**: `PascalCase`. **Unexported**: `camelCase`.
- **Acronyms keep their case**: `CPUUsage`, `parseURL`, `HTTPServer`. When an
  acronym is exported it's all-caps (`PID`); unexported, all-lower (`pid`).
- **Files**: lowercase, no spaces. `_test.go` for tests, `doc.go` for package
  docs. One concern per file (ties into
  [solid-modularity](solid-modularity.md) SRP and the
  [layered-seams](dependency-inverted-layered-seams.md) file splits).
- **Booleans read as predicates**: `isRunning`, `hasParent`, `canRender`,
  `visible`. Not `running`/`flag`/`status` for a bool.

## How it applies in this project

- Collectors follow the type/constructor idiom: `CPUCollector` +
  `NewCPUCollector`, `MemoryCollector` + `NewMemoryCollector`
  ([`internal/monitor`](../../internal/monitor)).
- Acronyms in domain types: `CPU`, `PID`, `IO` keep their case
  (`CPUCollector`, not `CpuCollector`).
- Namespaced dictionaries name themselves by domain so call sites are
  self-documenting: `palette.Accent`, `sizeName.MetricValue`, `font.SansRegular`
  — see [ui-conventions](ui-conventions.md) for the dictionary convention itself.
- The chart series option `setVisible(v bool)` and field `visible` read as
  predicates ([`linechart.go`](../../internal/ui/linechart.go)).

## Checklist

- [ ] Package name short, lowercase, single-word, no stutter?
- [ ] Exported = PascalCase, unexported = camelCase?
- [ ] Acronym cased correctly (`PID`/`pid`, `CPU`, `URL`)?
- [ ] File lowercase, single-concern, right suffix (`_test.go`/`doc.go`)?
- [ ] Boolean named as a predicate (`is*`/`has*`/`can*`)?

## Related

- [function-design.md](function-design.md), [idiomatic-go.md](idiomatic-go.md), [type-safety.md](type-safety.md) — the rest of the Go-craft set.
