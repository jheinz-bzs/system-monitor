# Project Conventions

The single, authoritative home for this codebase's engineering standards. Each
standard has a full doc here (rationale + how it applies to *this* project with
real code references) and a terse enforcement counterpart under
[`.claude/rules/`](../../.claude/rules/) that Claude consults while editing.

> These docs absorb and replace the former monolithic `docs/CODING-STANDARDS.md`.
> Conventions live here now.

## The standards

### Cross-cutting

| Standard | Doc | One-line rule |
|----------|-----|---------------|
| No string literals | [no-string-literals.md](no-string-literals.md) | A string that carries meaning or repeats belongs in a named constant or dictionary. |
| No magic numbers | [no-magic-numbers.md](no-magic-numbers.md) | A number with units, identity, or a design origin belongs in a named constant or the spacing scale. |
| SOLID & modularity | [solid-modularity.md](solid-modularity.md) | One reason to change per file; depend on abstractions; keep interfaces small. |
| DRY | [dry.md](dry.md) | Each fact lives in exactly one place; wait for the third occurrence before abstracting. |
| Dependency-inverted layered seams | [dependency-inverted-layered-seams.md](dependency-inverted-layered-seams.md) | Layers meet at a neutral abstraction package; extend by registering, not by editing wiring. |

### Go craft

| Standard | Doc | One-line rule |
|----------|-----|---------------|
| Naming | [naming.md](naming.md) | Follow Go house style; names say what a thing *is*. |
| Function design | [function-design.md](function-design.md) | Small, single-purpose, guard-claused; comments say *why*. |
| Idiomatic Go | [idiomatic-go.md](idiomatic-go.md) | `gofmt`/`vet` clean; handle every error; accept interfaces, return concretes; stdlib first. |
| Type safety | [type-safety.md](type-safety.md) | Narrowest type; named types/typed constants for known value sets; no needless `any`. |

### Project-specific (UI / architecture)

| Standard | Doc | One-line rule |
|----------|-----|---------------|
| UI & architecture conventions | [ui-conventions.md](ui-conventions.md) | Keep layers separate; namespaced dictionaries; spacing scale; no `//go:embed`; no HTML/CSS in Fyne. |

## How these are enforced

1. **While coding** — the rules under `.claude/rules/` state the hard MUST/MUST NOT
   constraints. Claude reads the relevant rule (and, when it needs detail, the
   matching doc here) before writing code in that area.
2. **On review** — the [`standards-reviewer`](../../.claude/agents/standards-reviewer.md)
   subagent audits a diff or file set against the standards and reports
   violations with file:line and the fix.

## Relationship to the code flowmap

The *dependency-inverted layered seams* standard is the named pattern extracted
from the **proposed** diagram in [`docs/CODE-FLOWMAP.md` §6](../CODE-FLOWMAP.md).
That section is the target end-state; the standard is how we get there. Sections
1–5 of the flowmap describe today's structure (the starting point).

## Pragmatism clause

These are Go standards, not C#/Java dogma. Go idiom wins ties: an error message
string, a `fmt.Errorf` format, a struct tag, or a one-off log line is **not** a
"string literal" violation; explicit per-site error handling is **not** a DRY
violation. The point is to eliminate *meaningful, repeated, or load-bearing*
patterns — not the letter sequence. Each doc names its explicit exceptions. When
a rule and Go idiom genuinely conflict, raise it rather than contorting the code.
