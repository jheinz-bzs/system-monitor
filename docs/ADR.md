# Architecture Decision Records

This file is an append-only log of non-obvious technical decisions for this project. Each entry captures *why* we chose what we chose, so future contributors don't re-litigate the same questions.

The `update-adr` skill maintains this file. Humans are welcome to edit. Decisions that turn out to be wrong should be marked **Superseded** with a pointer to the replacement entry — not deleted.

## Entry format

```
## ADR-NNN: [Concise Decision Title]

**Date:** YYYY-MM-DD
**Status:** Active | Superseded by ADR-NNN
**Area:** [Database | API | Architecture | Infrastructure | etc.]

### Context

[1-2 paragraphs explaining what prompted the decision.]

### Decision

[1 bold sentence: what we decided.]

### Rationale

[Why this choice over alternatives.]
```

---

## ADR-001: In-memory ring buffer over persistence

**Date:** 2026-06-02
**Status:** Active
**Area:** Architecture

### Context

A system monitor must show recent activity (CPU, memory, network, etc.) updating at ~1s resolution. We had to decide whether to persist metric history to disk/a database or hold it in memory.

### Decision

**Metric history lives entirely in an in-memory ring buffer per metric (~1 minute at 1s resolution) — no database, no file I/O for metrics.**

### Rationale

System monitoring is a recency problem: users care about what's happening now and in the last moments, not historical analytics. Keeping history in memory makes the app lightweight, offline, and privacy-respecting (nothing touches disk), and removes an entire persistence layer from a ~1-month exploration project. A bounded ring buffer caps memory use and matches the fixed-window charts. The tradeoff — no long-term history across restarts — is acceptable and explicitly out of scope.

---

## ADR-002: Fyne for the native desktop UI

**Date:** 2026-06-02
**Status:** Active
**Area:** Architecture

### Context

The project is an exploration of native desktop development outside web frameworks, targeting Windows, macOS, and Linux. We needed a Go-native UI toolkit.

### Decision

**Build the UI with the Fyne toolkit, accepting its CGO/OpenGL build requirement.**

### Rationale

Fyne is a pure-Go, cross-platform toolkit that renders its own widgets (no Electron, no embedded browser), which fits the "lightweight native app" goal and the single-language (Go) codebase. It runs on all three target desktops from one codebase. The cost is a CGO/OpenGL dependency (a C compiler is required per platform — e.g. mingw-w64 on Windows); we accept this in exchange for a native, dependency-light binary. Because Fyne does not use HTML/CSS, visual intent from the design system is translated into Fyne's canvas/widget model rather than web conventions.

---

## ADR-003: Collection / storage / UI package split

**Date:** 2026-06-02
**Status:** Active
**Area:** Architecture

### Context

Metric data flows from the OS (via gopsutil) into history buffers and then into eight UI tabs. Without clear boundaries, gopsutil calls and buffer logic would leak into widget code.

### Decision

**Separate concerns into three internal packages: `internal/monitor` (gopsutil-backed collection), `internal/metrics` (in-memory ring-buffer storage), and `internal/ui` (Fyne presentation). UI reads from buffers; it does not call gopsutil directly.**

### Rationale

The split keeps collection testable in isolation (collectors assert on shape/invariants), keeps storage independent of both the data source and the renderer, and lets the UI consume a stable in-memory interface. It also supports treating process IDs as first-class shared state so cross-tab navigation (e.g. Ports → owning process) can be wired cleanly without the UI reaching back into collection code.

---

## ADR-004: Code-generated bundled assets instead of `//go:embed`

**Date:** 2026-06-03
**Status:** Active
**Area:** Build / Infrastructure

### Context

Fonts (IBM Plex faces, ~1.1 MB) and nav icons (Lucide SVGs) must ship inside the binary so the app stays self-contained (ADR-002). The scaffold originally bundled them with one `//go:embed` directive per file, each populating a bare `var x []byte`. That pattern is opaque: `//go:embed` reads as a comment to anyone who doesn't know it's a compile-time directive, and the `[]byte` vars look empty/uninitialized at the call site. The two ways to get bytes *into* a Go binary are `//go:embed` or compiling the bytes as Go source (codegen); loading from disk at runtime was rejected because it breaks the single self-contained binary.

### Decision

**Bundled assets are compiled into Go source by a generator (`tools/genassets`) that writes `internal/ui/assets_gen.go` (base64 byte maps); no `//go:embed` appears anywhere in the codebase. All assets are loaded through an explicit `resource("fonts/…")` function (`internal/ui/assets.go`). The generated file is gitignored and produced by `make generate`.**

### Rationale

Codegen keeps the self-contained single `.exe` while making the data flow obvious — every asset is fetched by path through a normal function call, and there are no magic comment directives or bare `[]byte` globals. `resource()` uses `path.Base()` so Fyne resource names are unchanged (no behavior change). Tradeoffs accepted: (1) a build step — `make generate`, wired as a prerequisite of `build`/`run`/`vet`, and required once on a fresh clone since `assets_gen.go` is gitignored; (2) a missing asset becomes a runtime panic at first `resource()` call rather than a compile error (the generator `log.Fatal`s on an empty glob to catch the common case early). The base64 encoding keeps the generated source ~1.5 MB rather than ~6–7 MB of raw byte literals. This reintroduces a pre-`go:embed`-style "bindata" step by deliberate developer preference, valuing call-site clarity over the idiomatic directive.
