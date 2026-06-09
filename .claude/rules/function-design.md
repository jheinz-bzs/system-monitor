# Rule: Function Design

Full doc: [docs/conventions/function-design.md](../../docs/conventions/function-design.md)

**MUST**
- Keep functions small and single-purpose — if you can't summarize it in one
  sentence, split it.
- Use guard clauses / early returns; keep the happy path at the lowest indent.
- Return errors last and wrap with context + `%w`
  (`fmt.Errorf("collect cpu: %w", err)`).
- Make orchestrators read like a story; push detail into helpers.
- Write comments that explain *why* (trade-off/constraint), not *what*.

**MUST NOT**
- Nest more than ~2 levels — refactor with a guard or extracted helper.
- Leave comments that restate the code.

Self-check: *One sentence to describe it? Happy path un-indented? Comments earn
their place?*
