# Standard: DRY (Don't Repeat Yourself)

## Principle

Every piece of knowledge — a value, a rule, a calculation, a layout decision —
has **one authoritative representation** in the codebase. DRY is about *knowledge*,
not about text: two snippets that happen to look alike but represent different
decisions are *not* a violation, and forcing them together couples things that
should move independently.

## The rule of three

Don't abstract on the second occurrence. Two similar things might diverge.
**On the third**, the pattern is real — extract it. (This is the
"premature abstraction" guard, stated as a positive rule; see also
[solid-modularity](solid-modularity.md).) Until then, a little duplication is
cheaper than the wrong abstraction.

## What counts as a violation

- **The same magic value in two places** — a color, size, threshold, duration,
  or string key copied rather than referenced. (This is where DRY meets
  [no-magic-numbers](no-magic-numbers.md) and [no-string-literals](no-string-literals.md).)
- **The same calculation inlined repeatedly** instead of a shared helper —
  e.g. value→Y-pixel mapping, "nice" axis rounding, byte/percent formatting.
- **The same adaptation written per case** — hand-wiring each collector into a
  source in `Run()` is the same line shape N times; the registry seam removes the
  repetition (see [layered-seams](dependency-inverted-layered-seams.md)).
- **A fact stated in code *and* prose that can drift** — e.g. the 1s/60-sample
  resolution living in three unrelated literals.

## What is NOT a violation

- **Coincidental similarity.** Two functions with similar structure but different
  reasons to change should stay separate.
- **Test setup that reads clearly when explicit.** Mild repetition in tests often
  beats a clever shared fixture.
- **Go's accepted verbosity** — explicit error handling at each call site is not
  duplication to "fix."

## How it applies in this project

- **Shared math already lives in `internal/metrics`.** `stats.go` / `capacity.go`
  are the single home for calculations every collector needs (per the flowmap §3:
  "Shared math lives in `internal/metrics`"). New cross-collector math goes here,
  not copy-pasted into `cpu.go` and `memory.go`.

- **Design tokens are single-sourced.** `palette`, `sizeName`, and the `space*`
  scale in [`theme.go`](../../internal/ui/theme.go) /
  [`spacing.go`](../../internal/ui/spacing.go) are *the* source for those values.
  Referencing them (not re-typing the hex/number) is DRY in action. Flowmap §6
  Move 2 takes this further by giving tokens one home in `theme/tokens.go`.

- **Chart formatting/geometry helpers exist to be reused.** `valueToY`,
  `niceNum`, `niceRange`, `formatCompact`, `formatAge` in
  [`linechart.go`](../../internal/ui/linechart.go) are shared helpers — when the
  Memory/Network/Disk charts arrive, they call these, they don't reimplement
  axis math. (The split into `chart/format.go` + `chart/renderer.go` makes the
  shared home explicit.)

- **The poll cadence is one concept across files.** `pollInterval` in `app.go`
  is documented as matching `metrics.HistoryCapacity`. Keep the relationship
  single-sourced rather than restating `time.Second` / `60` independently.

## DRY vs. the other standards (resolving tension)

- DRY can pull toward **premature abstraction**; SRP/OCP and the rule of three
  hold it back. Extract only a *real* third occurrence.
- Single-sourcing a token (DRY) is the *same act* as eliminating a magic
  number/string. When you remove a literal, you are usually also de-duplicating.

## Checklist

- [ ] Is this the third occurrence of the same knowledge? → extract a shared home.
- [ ] Am I copying a token/value that already has a name? → reference it instead.
- [ ] Is this a real shared decision, or just code that looks similar? → don't couple coincidences.
- [ ] Does the shared math belong in `internal/metrics` / a chart helper rather than inline?
- [ ] Am I abstracting on occurrence #2? → wait.

## Related

- [no-magic-numbers.md](no-magic-numbers.md), [no-string-literals.md](no-string-literals.md) — de-duplicating values *is* removing magic literals.
- [solid-modularity.md](solid-modularity.md) — modular seams are what let a fact live in one place.
