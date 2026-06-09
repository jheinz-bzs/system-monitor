# Standard: Function Design

## Principle

A function does one thing, states its preconditions first, and reads top-to-
bottom like a sentence. Orchestrators tell the story; helpers hold the detail.
Depth and length are the two smells — both are fixed by extracting.

## Rules

- **Small, single-purpose.** If you can't summarize a function in one sentence,
  split it. (Same test as SRP at the file level — see
  [solid-modularity](solid-modularity.md).)
- **Guard clauses and early returns over nested conditionals.** Handle the
  edge/error case and `return` early; keep the happy path at the lowest
  indentation. **Deep nesting is a violation** — refactor with guards or extracted
  helpers, don't keep indenting.
- **Errors are the last return value**, and wrap with context:
  `fmt.Errorf("collect cpu: %w", err)`. (`%w` so callers can `errors.Is`/`As`.)
- **Orchestrating functions read like a story; details live in helpers.**
- **Comments say *why*, not *what*.** The code already says what it does; a
  comment earns its place by explaining a non-obvious reason, trade-off, or
  constraint. Delete comments that restate the line below them.

## How it applies in this project

- [`app.go · Run()`](../../internal/ui/app.go) is the model orchestrator: it
  reads as a sequence (new app → set theme → build collectors → wire content →
  start poller → show window), and its comments explain *why* (e.g. why the
  redraw is driven from the poller rather than a separate UI ticker — a real
  trade-off, not a restatement).
- The collectors' nil-handling uses guard-style fallback (`if cpu != nil { … }`)
  rather than nesting the whole wiring inside a conditional.
- The renderer in [`linechart.go`](../../internal/ui/linechart.go) splits its
  story (`arrange`) from its details (`layoutYLabels`, `layoutGrid`,
  `seriesPoints`, `buildLines`) — orchestrator + helpers.

## Checklist

- [ ] Can I summarize this function in one sentence? If not → split.
- [ ] Is the happy path at the lowest indent, with edge cases guarded early?
- [ ] More than ~2 levels of nesting? → extract a helper or invert with a guard.
- [ ] Is the error the last return, wrapped with `%w` and context?
- [ ] Does every comment explain *why*? Delete the *what* comments.

## Related

- [solid-modularity.md](solid-modularity.md) — SRP at file/type level; same single-purpose test.
- [idiomatic-go.md](idiomatic-go.md) — error-handling idioms in full.
- [dry.md](dry.md) — extract a repeated helper only on the third occurrence.
