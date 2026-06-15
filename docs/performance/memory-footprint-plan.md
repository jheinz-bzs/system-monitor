# Memory Footprint — Diagnosis & Optimization Plan

**Status:** investigated, not yet implemented (deferred by request).
**Date:** 2026-06-15.
**Context:** Empty Fyne window measured ~60 MB RSS; with three live tabs (CPU,
Memory, Processes) wired up it sits ~300 MB. This doc records *why* and the
fixes to apply when we pick this up.

---

## TL;DR

It is **allocation churn + fixed Fyne/Go baseline, not a leak.** The live
working set (ring buffers, snapshots, widgets) is single-digit MB. Most of the
RSS is GC headroom held against per-second chart-buffer allocations, plus
unavoidable graphics-stack baseline that the OS reclaims only slowly.

Confirm it's not a leak: footprint should **plateau** (~30–60 s then hold), not
climb forever. A `runtime.GC()` + `debug.FreeOSMemory()` would visibly drop RSS
— which only happens with already-dead (churned) memory.

---

## Where the memory goes

### 1. Fixed baseline (~50–80 MB) — not our code
Fyne stands up an OpenGL context, a glyph/font atlas, and GPU-driver mappings.
On Windows, RSS includes those driver allocations. An idle Fyne window at
50–80 MB is normal.

### 2. Charts re-allocate full-resolution buffers every refresh — **the big lever**
`internal/ui/linechart.go` → `lineChartRenderer.renderSeries` (~line 456) runs
on every refresh (1 Hz) and allocates fresh buffers each time:

```go
img := image.NewRGBA(image.Rect(0, 0, w, h))            // fresh w×h×4 image
for _, f := range r.fills  { ras := vector.NewRasterizer(w, h); ... } // full w×h buffer per fill
for _, ln := range r.lines { ras := vector.NewRasterizer(w, h); ... } // full w×h buffer per line
```

`vector.NewRasterizer(w,h)` allocates a `w*h` coverage buffer. The **CPU chart
has ~14 series** (overall + 12 cores), so at ~1100×300 that is ~14 × 1.3 MB plus
the RGBA image — roughly **15–20 MB allocated and discarded per second, per
visible chart**.

The raster is generator-backed (`canvas.NewRaster(r.renderSeries)`,
linechart.go:206), so the painter only calls `renderSeries` for the **visible**
chart. But that one chart's churn alone drives the heap to its GC target (~2×),
and the OS scavenger returns pages slowly → RSS parks at the high-water mark.

### 3. Every poll tick refreshes all live tabs, not just the visible one
`internal/ui/shell.go` `buildContent` builds a refresh closure that iterates
**all** registered refreshers:

```go
refresh := func() { for _, r := range refreshers { r() } }
```

driven once a second from `app.go` (`poller.OnTick(func() { fyne.Do(refresh) })`).
So three live tabs do three tabs' worth of `Snapshot()`/`arrange()` work each
tick even though only one is on screen. (The hidden charts' *textures* aren't
regenerated — they're not in the scene graph — but the CPU/alloc work and dirty
bookkeeping still scale with tab count.)

### 4. Go returns freed memory to the OS slowly
The scavenger trickles freed pages back over minutes, so RSS overstates the live
set. This is why Task Manager's number looks alarming relative to what's
actually needed.

---

## Fixes (in priority order)

### Fix A — Reuse the chart image + rasterizer across frames (highest value)
**File:** `internal/ui/linechart.go` (`lineChartRenderer`, `renderSeries`).

- Cache the `*image.RGBA` on the renderer; reallocate only when `w`/`h` change.
  Clear it each frame (cheap relative to allocation) since lines don't cover the
  whole plot.
- Hold a single reusable `*vector.Rasterizer`; call `ras.Reset(w, h)` between
  series instead of `vector.NewRasterizer` per series.

Eliminates the multi-MB-per-second allocation that dominates the churn.
**Watch:** keep it correct under size/scale (HiDPI) changes; reset buffers on
resize. Guard the existing `w<=0 || h<=0` early-return.

### Fix B — Refresh only the active tab
**Files:** `internal/ui/shell.go` (`buildContent` / `newTabs`), driven from
`internal/ui/app.go`.

- Track the active tab index; the tick refreshes only that tab's refresher.
- Refresh a tab's pane when switching to it (in `selectIndex`) so the
  newly-shown tab isn't stale for up to one tick.

Stops snapshot/arrange work scaling with the number of live tabs.

**Architecture note:** respect the existing seam pattern — the refresher
registry already exists; this is a change to *which* refreshers fire, not new
cross-layer wiring. Keep `Run()` untouched.

### Fix C — Zero-code lever (ops, optional)
Set `GOMEMLIMIT` (e.g. `GOMEMLIMIT=200MiB`) to make the GC run more aggressively
and hold RSS down, at a small CPU cost. No code change. Useful as a stopgap or
for constrained deployments.

---

## How to verify (do this first when resuming)

1. Add a temporary memstats logger (goroutine logging `runtime.ReadMemStats`
   `HeapAlloc` / `HeapInuse` / `Sys` every few seconds to stderr). Capture a
   baseline trace: idle, one tab, three tabs, after a `debug.FreeOSMemory()`.
2. Apply Fix A; re-trace. Expect HeapAlloc churn (and the GC target, hence RSS)
   to drop sharply.
3. Apply Fix B; re-trace while switching tabs. Expect per-tick allocation to
   stop scaling with tab count.
4. Remove the temporary logger (or gate it behind a debug build tag / env var).

Pair the change with a benchmark in `linechart_test.go` (e.g.
`BenchmarkRenderSeries`) asserting allocations/op drop, so the win is locked in.

---

## References
- `internal/ui/linechart.go` — `renderSeries` (~456), `canvas.NewRaster` (~206),
  `lineChartRenderer` struct (~225).
- `internal/ui/shell.go` — `buildContent` refresh closure, `newTabs` refresher
  registry, `selectIndex`.
- `internal/ui/app.go` — `poller.OnTick(...)` drive loop.
- `internal/ui/raster.go` — `strokePolyline` / `fillPolygon` (consumers of the
  rasterizer; unchanged by the fixes, but relevant when reworking buffer reuse).
