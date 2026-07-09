# CLAUDE.md — System Monitor

Native desktop system monitor in **Go** + the **Fyne** UI toolkit + **gopsutil**,
for developers and power users. No persistence — metric history lives in an
in-memory ring buffer (~1 min at 1s resolution). **9 tabs**: Overview, CPU,
Memory, Disk, Network, Processes, Ports, Connections, Settings.

## Design Artifacts (authoritative — consult before any layout/color/type change)

- **`.claude/system-monitor-design-doc.docx`** — product spec: what each tab
  does, tech stack, principles (chart type chosen per-metric not templated; panes
  per tab fit the data). Read first for intent.
- **`Design_Markdown`** — design-system brief: tone (industrial/utilitarian,
  htop / Grafana-dark / Linear / Warp), per-tab layout pattern, chart-type-per-tab.
- **`docs/wireframe designs/*.html`** — primary visual reference; **open the
  matching file in a browser before a UI change** (it's the layout contract).
  Fyne renders its own widgets — translate visual intent, no HTML/CSS mental model.
  - Design system: `design-system-01..06-*.html` (palette · typography/icons ·
    spacing/geometry · components panels/charts/tables · controls/nav/chrome ·
    chart language).
  - Per tab: `tab-01-overview-*.html` … `tab-08-connections-*.html`.

Exact tokens (colors, type scale, spacing, chart language): **`docs/DESIGN-SYSTEM.md`**.

## Tab Layouts

| Tab | Panes | Top | Bottom |
|-----|-------|-----|--------|
| Overview | 1 | 2×4 grid of metric panels with sparklines | — |
| CPU | 3 | Multi-line chart (overall + per-core) | Per-core grid (left) + top processes table (right) |
| Memory | 2 | Stacked area chart (used/cached/buffers) + breakdown bar | Top processes by memory table |
| Disk | 2 | Treemap (storage by dir) + volumes list (right) | I/O line chart (read/write/total) |
| Network | 1 | Stat panels row + three-line bandwidth chart | — |
| Processes | 2 | Treemap sized by CPU or memory | Full sortable/filterable process table |
| Ports | 1 | Filterable table with cross-nav jump links | — |
| Connections | 1 | Filterable table with state pills and cross-nav links | — |

## Architecture Notes

- **Process IDs are first-class identifiers.** Shared state across tabs is designed
  so cross-tab nav (e.g. Port → owning process) wires up cleanly; the cross-nav
  link component is defined in the design system.
- **Ring buffer per metric.** No database, no file I/O for metrics. ~1 min at 1s.
- **Settings persist via `app.Preferences()`** (BZS253-72) — typed keys + defaults
  in `internal/ui/prefs.go`; no config file or DB (no-persistence rule still
  governs *metric history*). Changes apply live via `applyHooks` (theme/cadence
  rebuild the widget tree); see ADR-007 (storage) and ADR-013 (live apply).
- **Fyne renders its own widgets** — translate wireframe intent into Fyne's
  canvas/widget model, no HTML/CSS conventions.

## Working on a change

1. Intent → the `.docx`. 2. Layout → the tab wireframe HTML. 3. Tokens →
`docs/DESIGN-SYSTEM.md`. 4. Don't add/remove panes from a tab without a
deliberate reason. 5. Chart types aren't interchangeable (see chart language).

## Project Standards (conventions)

Ten engineering standards govern code in `internal/*` and `cmd/*`. **Consult the
relevant one before writing/changing code in its area** — open it on demand, don't
paste wholesale. Index: `docs/conventions/README.md`. Each has a terse MUST/MUST
NOT quick rule in `.claude/rules/` and a full doc (rationale + examples +
exceptions) in `docs/conventions/`:

no-string-literals · no-magic-numbers · solid-modularity · dry ·
dependency-inverted-layered-seams · naming · function-design · idiomatic-go ·
type-safety · ui-conventions

- Go standards — Go idiom wins ties; honor each doc's "Allowed / not a violation".
- The codeflow standard targets `docs/CODE-FLOWMAP.md` §6 (proposed end-state).
- After non-trivial changes, audit with the **`standards-reviewer`** agent.

<!-- bizstream-bcs:start -->
## BizStream BCS docs

For project context: `docs/TECH-STACK.md`, `docs/conventions/` (engineering
standards), `docs/DESIGN-PRINCIPLES.md`, `docs/TESTING-CONSIDERATIONS.md`,
`docs/ADR.md`, `docs/USING-GITHUB.md`. Check `~/.claude/CLAUDE.md` plus
per-project auto-memory for personal overrides before any GitHub write.

## Execution rules

- Fyne needs CGO (`CGO_ENABLED=1`) and a C compiler — ensure `gcc` is on `PATH`
  (Windows: WinLibs mingw-w64). Prefer the Makefile targets (`make run`/`build`/
  `vet`/`fmt`/`tidy`).
- Fonts/icons are compiled into `internal/ui/assets_gen.go` by `tools/genassets`
  (no `//go:embed`). That file is gitignored, so on a fresh clone run
  `make generate` once before a bare `go build`/`go vet` (the `make` targets do it
  automatically). Re-run after changing files under `internal/ui/fonts|icons/`.
<!-- bizstream-bcs:end -->
