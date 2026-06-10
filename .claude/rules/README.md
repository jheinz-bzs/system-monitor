# Claude Rules — Project Standards

Terse, enforceable constraints Claude applies while editing this repo. Each rule
is the MUST/MUST NOT distillation of a full convention doc under
[`docs/conventions/`](../../docs/conventions/) — read the matching doc when you
need rationale, examples, or the exception list.

### Cross-cutting
| Rule | Full doc |
|------|----------|
| [no-string-literals.md](no-string-literals.md) | [docs/conventions/no-string-literals.md](../../docs/conventions/no-string-literals.md) |
| [no-magic-numbers.md](no-magic-numbers.md) | [docs/conventions/no-magic-numbers.md](../../docs/conventions/no-magic-numbers.md) |
| [solid-modularity.md](solid-modularity.md) | [docs/conventions/solid-modularity.md](../../docs/conventions/solid-modularity.md) |
| [dry.md](dry.md) | [docs/conventions/dry.md](../../docs/conventions/dry.md) |
| [dependency-inverted-layered-seams.md](dependency-inverted-layered-seams.md) | [docs/conventions/dependency-inverted-layered-seams.md](../../docs/conventions/dependency-inverted-layered-seams.md) |

### Go craft
| Rule | Full doc |
|------|----------|
| [naming.md](naming.md) | [docs/conventions/naming.md](../../docs/conventions/naming.md) |
| [function-design.md](function-design.md) | [docs/conventions/function-design.md](../../docs/conventions/function-design.md) |
| [idiomatic-go.md](idiomatic-go.md) | [docs/conventions/idiomatic-go.md](../../docs/conventions/idiomatic-go.md) |
| [type-safety.md](type-safety.md) | [docs/conventions/type-safety.md](../../docs/conventions/type-safety.md) |

### Project-specific (UI / architecture)
| Rule | Full doc |
|------|----------|
| [ui-conventions.md](ui-conventions.md) | [docs/conventions/ui-conventions.md](../../docs/conventions/ui-conventions.md) |

The [`standards-reviewer`](../agents/standards-reviewer.md) subagent audits
against these at once. These are Go standards — Go idiom wins ties; see each
doc's "Allowed / not a violation" section before flagging.
