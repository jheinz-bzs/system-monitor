---
name: docs-authoring
description: >-
  Write the content of a docs/ entry once its home is chosen. Use when drafting the actual
  text for project documentation — e.g. "write the docs/ entry", "draft this convention",
  "write an ADR for this decision", "fill in the integration reference", "what do I put in
  this doc". Gives the format and the prescriptive voice for docs/conventions/,
  docs/knowledge/decisions/ (mini-ADRs), and docs/knowledge/integrations/ entries, plus
  the rule to update docs in the same PR as the code. For deciding WHERE a doc belongs
  first, use docs-routing. Does not cover authoring rules, agents, or skills.
allowed-tools: Read, Write, Edit, Bash
---

# docs/ authoring — what to put in a doc

Write the content of a `docs/` entry. Assumes the home is already chosen (run
`docs-routing` first if not).

## Precondition

You know which `docs/` location the entry belongs in (`conventions/`,
`knowledge/decisions/`, or `knowledge/integrations/`). If you don't, stop and run
`docs-routing`.

## Governing principle

**Prescriptive beats descriptive.** "We do X because Y; avoid Z" outperforms a
neutral explanation every time. Capture resolved decisions and gotchas — not
tutorials or restatements of framework behavior. The *why* is what lets a future
session extrapolate to cases you never wrote down.

## `docs/conventions/` entries — the "how"

- Imperative house style: state the rule we follow here.
- One topic per file; keep the filename to the topic (e.g. `serialization-boundary.md`).
- Ground every rule in a concrete example drawn from the **actual** codebase, not a
  generic one.
- No framework tutorials — document only the delta from default behavior.

## `docs/knowledge/decisions/` entries — the "why" (mini-ADR)

Write each decision as a tiny ADR, using this template verbatim:

```md
# <decision / topic>
**Context:** the situation that forces a choice.
**Decision:** what we do (prescriptive).
**Why:** the reasoning — this is the part that generalizes.
**Example:** a concrete snippet.
```

The **Why** is the load-bearing part: it is what lets the agent handle cases the
ADR never spelled out. Don't omit it.

## `docs/knowledge/integrations/` entries — private reference

- Reference material with no public source: auth flow, field meanings, the quirks
  that aren't on the internet.
- Pair it with a workflow skill that points at it — the skill carries the procedure
  ("to wire up X, do these steps"), the doc carries the reference, loaded on demand.
- Name it predictably: `docs/knowledge/integrations/<name>.md`.

## Maintenance

- **Update the doc in the same PR as the code it describes** — stale docs actively
  mislead the agent.
- Keep `CLAUDE.md` an index, not a manual; detail lives in `docs/`.

## Scope

This skill covers only authoring content under `docs/`. It does not author rules,
agents, or skills. Use `docs-routing` to decide a doc's home before writing it.
