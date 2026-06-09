---
name: docs-routing
description: >-
  Decide WHERE a piece of project documentation belongs before writing it. Use when an
  agent has a learning, decision, convention, or quirk to record and must choose its home —
  e.g. "where should this doc go", "add documentation for X", "document this decision",
  "is this a doc or a rule", "where do I put this convention". Routes the learning to
  docs/conventions/, docs/knowledge/decisions/, docs/knowledge/integrations/, an inline
  comment, or (sparingly) CLAUDE.md — and first checks whether it should be written down
  at all. Covers placement only; for what to write once the home is chosen, use
  docs-authoring. Does not cover authoring rules, agents, or skills.
allowed-tools: Read, Write, Edit, Bash
---

# docs/ routing — where a learning belongs

Pick the home for a piece of documentation under `docs/`. Decide placement only;
once the home is chosen, hand off to `docs-authoring` for the content.

## Precondition

You have a concrete learning to record (a decision, a convention, a gotcha, a
quirk). If you are unsure it is worth documenting at all, run the "When to
document" gate first — most things should not be written down.

## When to document at all

**Document the delta, not the textbook.** Capture only what a fresh session
*can't* reliably reproduce:

- Resolved decisions where Claude is otherwise inconsistent.
- Tribal knowledge with no public source (a quirky internal integration).
- Load-bearing fixes that must not be reverted.
- The *why* behind a non-obvious choice.

Do **not** document how a framework works generically (Go / Fyne) —
Claude already knows it — or what well-named code already says.

**Two triggers that mean "write it down now":**
1. The fresh-session test fails — a clean session gets it wrong or inconsistent.
2. You've re-prompted the same thing twice.

If neither holds and it isn't load-bearing, **stop — don't document it.**

## Routing procedure (decision tree, in order)

1. **Is it a constraint Claude should apply automatically when editing matching
   files?** → it's a path-scoped rule, **not** documentation. Out of scope for
   this skill — just note that a rule is the right home and stop.
2. **Is it tied to one exact line ("don't touch this")?** → an inline **comment**
   at that line. Locality beats a doc; it can't be missed because it's where the
   edit happens.
3. **Is it imperative house style — the "how" we do things here?** →
   `docs/conventions/`, one topic per file.
4. **Is it rationale / a decision / tribal knowledge — the "why"?** →
   `docs/knowledge/`:
   - a decision with context and reasoning (an ADR) → `docs/knowledge/decisions/`.
   - reference material for a private integration (auth flow, field meanings,
     quirks) → `docs/knowledge/integrations/<name>.md`.
5. **Does every session need it, and is it tiny and universal?** → `CLAUDE.md`,
   **sparingly** — it pays an always-loaded tax.

## Context-cost reminder

Bias toward `docs/**` (loaded only on demand). Keep `CLAUDE.md` an index, not a
manual — under ~200 lines. The default home for new documentation is `docs/`, not
`CLAUDE.md`.

## Discoverability — make the new doc reachable

When you add a doc, ensure it can be found again without bloating always-loaded
context:

- **Index directories, not files, in `CLAUDE.md`.** Point at folders (e.g.
  `docs/knowledge/integrations/`); never add a per-file list. The directory
  pointer is constant-size no matter how many docs you add.
- **Use predictable naming** (`docs/knowledge/integrations/<name>.md`) so the path
  is guessable from the task — a naming convention beats a maintained list.
- **Update `docs/README.md`** (the human-browsable map) if one exists; that map
  lives one tier down and is read on demand.
- **Skills self-index** via their `description` — never list skills in `CLAUDE.md`.

## Scope

This skill decides placement only. It does not cover what to write (see
`docs-authoring`) and does not cover authoring rules, agents, or skills.
