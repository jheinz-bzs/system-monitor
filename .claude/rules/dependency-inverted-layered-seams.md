# Rule: Dependency-Inverted Layered Seams (codeflow)

Full doc: [docs/conventions/dependency-inverted-layered-seams.md](../../docs/conventions/dependency-inverted-layered-seams.md)
· Source diagram: [docs/CODE-FLOWMAP.md §6](../../docs/CODE-FLOWMAP.md)

The target codeflow pattern: layers (monitor → metrics/ringbuffer → ui) meet at
**neutral seam packages**; extension happens by **registering providers**, not by
editing wiring. The composition root (`cmd/system-monitor` + thin `ui.Run`) is the
only place that knows concretes.

**MUST**
- Put an interface shared by two packages in a package that depends on neither
  (the `Source` seam belongs in `internal/series`, not inside `linechart.go`).
- Consume data across layers through the seam interface, never by importing the
  other layer's concrete types.
- Add a new metric area / tab by registering a provider against the seam — leave
  `Run()` and `shell` untouched.
- Split a file when it grows a second reason to change (group by reason, not
  convenience); keep pure math/format helpers free of Fyne so they're testable.

**MUST NOT**
- Add a `ui → monitor` (or any cross-layer) concrete import.
- Add a feature by editing the central `liveSources` struct + `Run()` wiring.

**Sequencing (when implementing §6):** extract `internal/series` first (lowest
risk), then split `linechart.go` → `chart/{linechart,renderer,raster,format}.go`,
then the tab registry last. Record the seam extraction in [docs/ADR.md](../../docs/ADR.md).

Self-check: *Does this add a cross-layer concrete import? Am I editing wiring
instead of registering? Does the interface I added live in a neutral package?*
