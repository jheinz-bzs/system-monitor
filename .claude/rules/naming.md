# Rule: Naming

Full doc: [docs/conventions/naming.md](../../docs/conventions/naming.md)

**MUST**
- Packages: short, lowercase, single-word, no underscores, no stutter
  (`monitor`, not `monitor.Monitor`).
- Exported = `PascalCase`; unexported = `camelCase`.
- Acronyms keep case: `PID`/`pid`, `CPU`, `URL` (`CPUCollector`, not `CpuCollector`).
- Files: lowercase, no spaces, single concern; `_test.go` / `doc.go` suffixes.
- Booleans as predicates: `isRunning`, `hasParent`, `canRender`.

**MUST NOT**
- Name a bool `running`/`flag`/`status`; widen a name into stutter.

Self-check: *Does the name say what the thing is, in Go's house style?*
