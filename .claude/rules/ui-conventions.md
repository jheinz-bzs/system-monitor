# Rule: UI & Architecture Conventions (this project)

Full doc: [docs/conventions/ui-conventions.md](../../docs/conventions/ui-conventions.md)

**Architecture MUST**
- Keep monitor / metrics+ringbuffer / ui layers separate; UI reads ring buffers
  and never calls `gopsutil` directly.
- Carry process IDs as typed first-class identifiers (cross-tab nav ready), not
  embedded in display strings.
- Translate the HTML wireframes into Fyne canvas/widget code — no HTML/CSS mental model.

**UI package MUST**
- Group package resources/tokens into namespaced dictionaries (`palette.`,
  `sizeName.`, `font.`, `icon.`, `status.`); mind collisions (`palette` not
  `color`; `sizeName` ≠ spacing scale).
- Use the `space*` scale for gaps/padding (exact-match); component dimensions get
  their own literal-px consts.
- Prefer `map` + default fallback over large value-keyed switches (`themeColors`/
  `themeSizes`); keep small boolean-guard switches (`Font()`).
- Add fonts/icons via `make generate` (ADR-004) and load through `resource(...)`.

**MUST NOT**
- Use `//go:embed`; hand-edit a `DO NOT EDIT` generated file; add a UI→monitor
  concrete import.

Self-check: *Does UI touch gopsutil/monitor concretes? Is a PID typed? New token
in the right dictionary? go:embed avoided?*
