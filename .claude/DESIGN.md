# System Monitor — Wireframe & Design System Brief

## What This Is

A native desktop system monitoring application built in Go with the Fyne UI toolkit. Think a developer-focused alternative to Task Manager / Activity Monitor — clean, dense, and built for people who actually want to understand what their machine is doing.

The goal of this brief is to produce:
1. A unified design system (colors, typography, spacing, component patterns)
2. Wireframes for each of the 8 tabs

---

## Platform & Constraints

- **Native desktop app** — not a website, not Electron. Built with [Fyne](https://fyne.io), a Go UI framework.
- Fyne renders its own widgets; it does not use HTML/CSS. However, the wireframes can be produced as HTML/SVG mockups for reference — the developer will translate the visual language into Fyne code.
- Target platform: primarily Windows, but should feel at home on macOS too.
- **Dark theme preferred** — system monitors live in dark environments (terminals, late nights, secondary monitors).
- Dense but not cluttered. This is a tool for power users, not a consumer dashboard.

---

## App Structure

The app has a persistent left sidebar or top tab bar for navigation between 8 tabs. All tabs share the same chrome (title bar, nav, status bar if any).

### Tabs

1. **Overview** — Grid of metric panels, one per key system metric. At-a-glance health view. No drill-down here, just live numbers and sparklines.
2. **CPU** — Detailed CPU view. Multiple panes: line chart of usage over time (overall + per-core lines on one chart), and a table or list of top CPU-consuming processes below.
3. **Memory** — Stacked area chart of used/cached/free over time. Process memory breakdown below (table or treemap).
4. **Disk** — Two panes: storage breakdown (treemap or bar chart by drive/directory), and I/O over time (read/write/total lines).
5. **Network** — Three-line chart: upload, download, total bandwidth on a shared time axis.
6. **Processes** — Full process table (sortable, filterable by name/user/etc). Treemap visualization above the table showing CPU or memory dominance by process.
7. **Ports** — Table of listening ports with owning process, port number, protocol. Button/link per row to navigate to the owning process in the Processes tab.
8. **Connections** — Live table of active TCP/UDP connections: local address, remote address, owning process, state.

---

## Layout Patterns

Each tab may use a different number of panes depending on what makes sense for the data:

- Some tabs have **one pane** (e.g. Network — just the chart)
- Most tabs have **two panes** — visualization on top, detail table below
- Some may have **three panes** (e.g. CPU — combined chart, per-core chart, process table)

There is no enforced template. Let the data dictate the layout.

---

## Chart Types (by tab)

| Tab | Chart Type |
|-----|-----------|
| Overview | Sparklines or mini area charts per panel |
| CPU | Multi-line chart (overall + per core) |
| Memory | Stacked area chart |
| Disk | Treemap for storage, line chart for I/O |
| Network | Three-line chart (upload / download / total) |
| Processes | Treemap (sized by CPU or memory) |
| Ports | Table only |
| Connections | Table only |

---

## Design Direction

**Tone:** Industrial / utilitarian with precision. Think terminal aesthetics meets modern data tooling — not flashy, not minimal-for-minimalism's-sake. Every element earns its place.

**Not:** Bubbly, consumer-friendly, gradient-heavy, rounded-everything. This is a tool for developers.

**References to consider:** htop, Grafana (dark theme), Linear, Warp terminal — dense, purposeful, confident.

---

## Design System Deliverables

Please define:

- **Color palette** — background, surface, border, text (primary/secondary/muted), accent, and semantic colors (green = healthy, yellow = warning, red = critical)
- **Typography** — font choices, sizes for headings, labels, table data, chart axis labels, status values
- **Spacing scale** — base unit and scale
- **Component patterns** — metric panel (overview card), data table rows, chart containers, tab/nav style, buttons, input fields (for filter/search)
- **Chart visual language** — line colors, grid style, axis style, tooltip style

---

## Wireframe Deliverables

For each of the 8 tabs, produce a wireframe showing:

- The layout and pane structure
- Placeholder charts (correct type, labeled axes, no real data needed)
- Table structures where applicable (column headers, a few placeholder rows)
- Any interactive elements (search/filter inputs, sort indicators, action buttons)

Wireframes can be low-to-mid fidelity but should use the design system colors and typography so they feel cohesive. The developer will use these as a direct reference when building in Fyne.

---

## Notes

- The app uses an **in-memory ring buffer** for metric history — no database. Charts show approximately the last 10 minutes of data at 1-second resolution.
- Cross-tab navigation is a stretch goal: e.g. clicking a process in the Ports tab jumps to that process highlighted in the Processes tab. Design with this in mind but it does not need to be wired up in wireframes.
- The app has no settings screen yet — out of scope for now.
