# CLAUDE.md — System Monitor

This file tells Claude how to understand and work with the design artifacts in this project.

---

## Project Summary

A native desktop system monitoring app built in **Go** using the **Fyne UI toolkit** and **gopsutil** for system data. The target audience is developers and power users. The app has no persistence — all metric history is held in an in-memory ring buffer (last ~1 minute at 1s resolution).

The app has **8 tabs**: Overview, CPU, Memory, Disk, Network, Processes, Ports, Connections.

---

## Design Artifacts

These files are the authoritative design reference. Always consult them before making layout, color, typography, or component decisions.

### `.claude/system-monitor-design-doc.docx`

The **product spec**. Describes what each tab does, the tech stack, and the guiding principles. Read this first to understand intent — especially the principle that chart types should be chosen per-metric, not templated uniformly, and that tabs use however many panes make sense for the data.

### `Design_Markdown`

The **design system brief**. Contains:
- Platform context (native Fyne app, not HTML/CSS — wireframes are for reference translation)
- Tone direction: industrial/utilitarian, dense not cluttered, terminal-meets-data-tooling (htop, Grafana dark, Linear, Warp)
- The full layout pattern for each tab (how many panes, what goes where)
- The chart-type-per-tab table
- What component patterns and chart visual language need to be defined

### `docs/wireframe designs/*.html`

The **full design system + wireframes**, as standalone HTML mockups. This is the primary visual reference — **open the relevant `.html` file in a browser before making any UI change**. The wireframe is the layout contract.

Design-system pages:

| File | Content |
|------|---------|
| `design-system-01-color-palette.html` | Color palette |
| `design-system-02-typography-icons.html` | Typography & icon spec |
| `design-system-03-spacing-borders-geometry.html` | Spacing, borders, geometry |
| `design-system-04-components-panels-charts-tables.html` | Components A: metric panel, chart container, data table |
| `design-system-05-components-controls-nav-chrome.html` | Components B: buttons, nav, pills, status bar |
| `design-system-06-chart-language.html` | Chart language: grid, axes, series, sparkline, treemap |

Per-tab wireframes:

| File | Tab |
|------|-----|
| `tab-01-overview-panel-grid.html` | Overview (panel grid) |
| `tab-01-overview-sidebar-compact.html` | Overview — compact sidebar variant |
| `tab-01-overview-sidebar-expanded.html` | Overview — expanded sidebar variant |
| `tab-02-cpu-chart-per-core-table.html` | CPU |
| `tab-03-memory-line-chart-breakdown.html` | Memory |
| `tab-04-disk-treemap-volumes-io.html` | Disk |
| `tab-05-network-three-line-bandwidth.html` | Network |
| `tab-06-processes-treemap-sortable-table.html` | Processes |
| `tab-07-ports-table-cross-nav.html` | Ports |
| `tab-08-connections-tcp-udp-table.html` | Connections |

These HTML mockups are for **visual reference only** — Fyne renders its own widgets and does not use HTML/CSS. Translate the visual intent into Fyne's canvas/widget model.

---

## Design System Quick Reference

Pull these values from the HTML wireframes when writing Fyne code or making visual decisions.

### Colors

| Token | Hex | Use |
|-------|-----|-----|
| `bg` | `#0e1014` | Window body / canvas |
| `surface` | `#161a21` | Panels, sidebar, cards |
| `surface-2` | `#1b212b` | Headers, nav, inputs, status bar |
| `surface-3` | `#222a36` | Row hover / selected |
| `plot-bg` | `#0b0d11` | Chart plot area |
| `border` | `#262e3a` | Panel edges, h-grid |
| `border-strong` | `#344150` | Emphasized dividers, pill outlines |
| `text` | `#e7eaf0` | Primary values, headings |
| `text-2` | `#9aa6b6` | Secondary labels, table data |
| `text-3` | `#616d7e` | Axis ticks, meta, muted captions |
| `accent` | `#4679fa` | Primary line, active nav, primary button |
| `accent-2` | `#6e93fb` | Hover, focus ring, jump links |
| `green` | `#3fb877` | Healthy / running |
| `yellow` | `#d8a134` | Warning / elevated |
| `red` | `#e2563f` | Critical / stopped |

Categorical series colors (per-core lines, multi-series): `c1 #4679fa`, `c2 #36c2d4`, `c3 #8b7cf6`, `c4 #d87cc0`, `c5 #54b86a`, `c6 #d8a134`, `c7 #e2856b`, `c8 #6e93fb`. Wrap after 8.

### Typography

- **IBM Plex Mono** — everything numeric, tabular, labels, axis ticks, status pills
- **IBM Plex Sans** — page titles and prose only

| Role | Font | Size | Weight |
|------|------|------|--------|
| Metric value | Mono | 26px | 500 |
| Page title | Sans | 17px | 600 |
| Table data | Mono | 12px | 400 |
| Panel/column label | Mono | 11px | 500, UPPERCASE, 0.06em tracking |
| Status pill | Mono | 10.5px | 400 |
| Axis tick / meta | Mono | 9px | 400, `text-3` color |

Tabular-nums on all Mono. No italics. Two weights max per family.

### Spacing (4px base unit)

`4 / 8 / 12 / 16 / 24 / 32 / 48px`

Key fixed heights: title bar 38px, tab bar 40px, panel header 34px, status bar 26px, nav item 32px, button/input 28px, table row ~29px.

Sidebar: expanded 178px, compact 54px.

### Charts

- Horizontal gridlines: `#262e3a`; vertical gridlines: `#1b212b` (quieter)
- Primary/overall line: 2.2px solid; secondary series: 1px at 55% opacity
- Area and sparkline fills: 30%→0% vertical gradient, never flat
- Treemap: squarified, 2px gutter, fills at 20% α + 1px stroke at full hue
- Axis ticks: muted mono 9px; time axis runs left (−1m) → right (now)

---

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

---

## Architecture Notes for Code Changes

- **Process IDs are first-class identifiers.** Shared state across tabs should be designed so cross-tab navigation (e.g. Port → owning process in Processes tab) can be wired up cleanly. The cross-nav link component is already defined in the design system.
- **Ring buffer per metric.** No database, no file I/O for metrics. Charts show ~1 minute of data at 1s resolution.
- **No settings screen** — out of scope for the current version.
- **Fyne renders its own widgets.** Do not reference HTML/CSS conventions when writing Fyne layout code. Translate visual intent from the wireframes into Fyne's canvas/widget model.

---

## How to Use These Files When Making Changes

1. **Understand the feature** — read `.claude/system-monitor-design-doc.docx` for intent and scope.
2. **Check the wireframe** — open the relevant `docs/wireframe designs/tab-*.html` file in a browser and find the matching tab. The wireframe is the layout contract.
3. **Apply the design system** — pull exact tokens (colors, sizes, spacing) from the `docs/wireframe designs/design-system-*.html` pages or from the quick reference table above.
4. **Respect tab pane structure** — don't add or remove panes from a tab without a deliberate reason. The number of panes per tab was chosen to fit the data.
5. **Chart types are not interchangeable** — use the chart type specified per tab (see table above and `design-system-06-chart-language.html` for chart language conventions).

## Project Standards (conventions)

Ten engineering standards govern code changes in `internal/*` and `cmd/*`.
`docs/conventions/` is their single home (it absorbed the former
`docs/CODING-STANDARDS.md`). **Consult the relevant standard before writing or
changing code in its area** — do not paste these files in wholesale; open the
one you need on demand. Index: `docs/conventions/README.md`.

| Standard | Quick rule (`.claude/rules/`) | Full doc (`docs/conventions/`) |
|----------|-------------------------------|--------------------------------|
| No string literals | `.claude/rules/no-string-literals.md` | `docs/conventions/no-string-literals.md` |
| No magic numbers | `.claude/rules/no-magic-numbers.md` | `docs/conventions/no-magic-numbers.md` |
| SOLID & modularity | `.claude/rules/solid-modularity.md` | `docs/conventions/solid-modularity.md` |
| DRY | `.claude/rules/dry.md` | `docs/conventions/dry.md` |
| Dependency-inverted layered seams (codeflow) | `.claude/rules/dependency-inverted-layered-seams.md` | `docs/conventions/dependency-inverted-layered-seams.md` |
| Naming | `.claude/rules/naming.md` | `docs/conventions/naming.md` |
| Function design | `.claude/rules/function-design.md` | `docs/conventions/function-design.md` |
| Idiomatic Go | `.claude/rules/idiomatic-go.md` | `docs/conventions/idiomatic-go.md` |
| Type safety | `.claude/rules/type-safety.md` | `docs/conventions/type-safety.md` |
| UI & architecture conventions | `.claude/rules/ui-conventions.md` | `docs/conventions/ui-conventions.md` |

- The **rules** files are the terse MUST/MUST NOT constraints; the **docs** carry
  rationale, project examples, and the exception lists. Read a doc when you need
  detail, then apply.
- These are Go standards — Go idiom wins ties; honor each doc's "Allowed / not a
  violation" section.
- The codeflow standard is the pattern from `docs/CODE-FLOWMAP.md` §6 (the
  proposed diagram) — the target end-state, not today's structure.
- After non-trivial code changes, audit with the **`standards-reviewer`** agent
  (`.claude/agents/standards-reviewer.md`).

<!-- bizstream-bcs:start -->
## BizStream BCS docs

For project context, read these:
- `docs/TECH-STACK.md`
- `docs/conventions/` — engineering standards (replaced `docs/CODING-STANDARDS.md`; see the Project Standards table above)
- `docs/DESIGN-PRINCIPLES.md`
- `docs/TESTING-CONSIDERATIONS.md`
- `docs/ADR.md`
- `docs/USING-GITHUB.md` — and check `~/.claude/CLAUDE.md` plus per-project auto-memory for personal overrides before any GitHub write operation

## Execution rules
- Fyne requires CGO (`CGO_ENABLED=1`) and a C compiler. Ensure `gcc` is on `PATH` before building/running (Windows: WinLibs mingw-w64). Prefer the Makefile targets (`make run`/`build`/`vet`/`fmt`/`tidy`).
- Bundled fonts/icons are compiled into `internal/ui/assets_gen.go` by `tools/genassets` (no `//go:embed`). That file is gitignored, so on a fresh clone run `make generate` once before a bare `go build`/`go vet` — the `make run`/`build`/`vet` targets do this automatically. Re-run after changing any file under `internal/ui/fonts/` or `internal/ui/icons/`.
<!-- bizstream-bcs:end -->
