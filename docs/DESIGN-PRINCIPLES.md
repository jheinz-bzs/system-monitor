# Design Principles

## What this is

A native desktop system monitor in Go + Fyne — a developer-focused alternative
to Task Manager / Activity Monitor. Scoped as a ~1-month exploration of native
desktop development outside web frameworks. It is lightweight, offline, and
privacy-respecting: everything lives on the user's machine, with no telemetry
and no network dependency.

## Audience

Developers and power users who actively want to understand what their machine is
doing — the htop / Grafana / Warp crowd, not the casual Task Manager user.
**Optimize for someone who reads dense, tabular data fluently and values
precision over hand-holding.**

## Design goals

- **Dense, not cluttered.** Every element earns its place. Maximize information
  per screen; no decorative chrome. "Industrial / utilitarian with precision."
- **Terminal-meets-data-tooling tone.** References: htop, Grafana dark, Linear,
  Warp. Confident and purposeful — never bubbly, gradient-heavy, or
  rounded-everything.
- **Right chart for the metric, not a template.** The four chart types
  (multi-line, stacked area, treemap, sparkline) plus tables each appear where
  the data calls for them. Visualization fits the metric, not the reverse.
- **Layout follows the data.** Tabs use however many panes make sense (one, two,
  or three) — visualization pane on top, detail table below where data is rich.
  No enforced template.
- **Dark by default.** System monitors live in dark environments.

## The design system is authoritative

Pull exact values from the wireframe PDF / CLAUDE.md quick reference — don't
improvise:

- **Color:** cool-neutral surfaces (chroma ≤ 0.01), accent off `#4679fa`,
  semantic green/yellow/red for healthy/elevated/critical, an 8-color
  categorical ramp for per-core/multi-series (wrap after 8).
- **Type:** IBM Plex Mono for *everything* numeric/tabular/labels (tabular-nums,
  uppercased 0.06em-tracked labels); IBM Plex Sans for titles/prose only. Two
  weights max per family. No italics.
- **Geometry:** 4px base grid; 1px solid borders (no 2px); 4px radius on
  cards/panels/buttons, 2–3px on bars/pills, 50% on status dots; fixed chrome
  heights (title 38, tab 40, panel header 34, status 26, row ~29).
- **Charts:** horizontal gridlines `#262e3a`, vertical quieter `#1b212b`;
  primary line 2.2px, secondary 1px @ 55%; fills are 30%→0% gradients, never
  flat; treemap squarified with 2px gutter, 20% α fill + full-hue stroke; time
  axis runs −Nm (left) → now (right).

## Platform & architecture considerations

- **Native on Windows, macOS, and Linux.** Fyne renders its own widgets — it
  does not use HTML/CSS. Translate visual intent into Fyne's canvas/widget
  model; never reach for web conventions. The UI should feel at home on all
  three desktops (watch CGO/OpenGL build requirements per platform).
- **Recency over history.** Metric history is an in-memory ring buffer
  (~1 minute at 1s resolution). System monitoring is a recency problem — we are
  not building a long-term metrics store.
- **Process IDs are first-class.** Shared cross-tab state is designed so
  cross-tab navigation (e.g. a Ports row → its owning process in Processes)
  wires up cleanly. The cross-nav link component is part of the design system.

## What we are NOT optimizing for

- Persistence / historical analytics — recency only, no database, no file I/O
  for metrics.
- Configurability — there is no settings screen (out of scope for this version);
  we pick sensible defaults.
- Consumer-friendly polish — no flashy, gradient-heavy, rounded UI.
