# Rule: Type Safety

Full doc: [docs/conventions/type-safety.md](../../docs/conventions/type-safety.md)

**MUST**
- Use the narrowest type that fits params and struct fields.
- Model a known set of values as a named type + typed constants (`iota` enum) —
  e.g. connection states / protocols become `type connState string` + consts, not
  raw strings.
- Use generic constraints (like the `numeric` constraint in `linechart.go`) over
  `any` when adapting multiple concrete types.

**MUST NOT**
- Pass `interface{}`/`any` where a concrete or constrained type fits.
- Widen a type "just in case"; pass a whole struct when one field is used.

Self-check: *Is the value set known (→ named type)? Am I erasing a type to `any`?*
