# Rule: No String Literals

Full doc: [docs/conventions/no-string-literals.md](../../docs/conventions/no-string-literals.md)

**MUST**
- Extract any string that is an identifier/key/domain term (theme name, map key,
  widget ID, tab label, resource path, connection state, protocol) into a named
  const or a namespaced dictionary (`palette`, `sizeName`, `font`, `icon`, …).
- Add new theme/resource names to the existing dictionary, not as a new loose
  `const fooName = "..."` global.
- Extract a literal the moment the same text appears a second time.

**MUST NOT**
- Type a raw theme/size/color name string at a call site (use `sizeName.X`).
- Re-type a resource path prefix instead of going through the `resource(...)` loader.

**Allowed (do not flag)**
- `fmt.Errorf`/log/`fmt` format strings at a single use site; struct tags;
  one-off self-evident UI prose. (Don't reuse the same message in two spots.)

Self-check: *Is this string an identifier the program reasons about, or text it
emits once?* Former → extract; latter → fine.
