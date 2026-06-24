# Design System Quick Reference

Exact tokens for writing Fyne code or making visual decisions. These mirror the
authoritative `docs/wireframe designs/design-system-*.html` pages — pull values
from here for convenience, from the HTML when in doubt.

## Colors

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

## Typography

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

## Spacing (4px base unit)

`4 / 8 / 12 / 16 / 24 / 32 / 48px`

Key fixed heights: title bar 38px, tab bar 40px, panel header 34px, status bar 26px, nav item 32px, button/input 28px, table row ~29px.

Sidebar: expanded 178px, compact 54px.

## Charts

- Horizontal gridlines: `#262e3a`; vertical gridlines: `#1b212b` (quieter)
- Primary/overall line: 2.2px solid; secondary series: 1px at 55% opacity
- Area and sparkline fills: 30%→0% vertical gradient, never flat
- Treemap: squarified, 2px gutter, fills at 20% α + 1px stroke at full hue
- Axis ticks: muted mono 9px; time axis runs left (−1m) → right (now)
