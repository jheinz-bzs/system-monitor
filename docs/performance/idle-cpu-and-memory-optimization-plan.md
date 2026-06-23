# Idle CPU & Memory Optimization — Audit & Plan

**Status:** audited, not yet implemented (future work — spec for a later ticket).
**Date:** 2026-06-18.
**Context:** At idle the app sits at a couple % CPU and ~300 MB RSS. This doc
records *why* and a tier-ranked set of fixes to cut both as far as possible
without losing functionality.
**Relationship to existing docs:** extends
[`memory-footprint-plan.md`](memory-footprint-plan.md), which already scopes the
memory-only side (its Fix A / Fix B / Fix C). This doc is the broader **idle CPU
+ memory** picture and ranks the work; for the memory-side implementation detail,
defer to that doc rather than duplicating it here. **Does not supersede any
current design spec** — purely additive future work.

---

## TL;DR

Idle cost is **two structural choices, not a leak**:

1. **Over-collection** — the poller calls `Collect` on *every* collector every
   1 s regardless of the visible tab, so the expensive gopsutil calls (full
   process detail + the connection table + disk usage) run continuously even on
   Overview. **This is the idle CPU.**
2. **Over-rendering + per-frame allocation** — every tab's refresher fires every
   tick, and the visible chart rebuilds its raster from freshly-allocated buffers
   each frame. **This is the ~300 MB** (GC headroom held against the churn; the
   Fyne/GL baseline ~50–80 MB is the floor).

Both axes share the same two root causes, so the same active-tab mechanism fixes
much of both.

---

## Root causes

### A. All collectors poll every tick, regardless of the visible tab
`internal/monitor/poller.go` (`collectAll`, ~line 116) iterates every registered
collector each tick; `internal/ui/app.go` (~line 150) registers all of them and
drives them at `pollInterval` (1 s). Nothing consults which tab is on screen, so
data only the Ports/Connections/Processes/Disk tabs need is sampled even on
Overview.

### B. Every tab's refresher fires every tick
`internal/ui/shell.go` (`newTabs`, ~line 260) builds `refresh := func() { for _,
r := range refreshers { r() } }`, driven once a second from `app.go` (~line 151,
`poller.OnTick(func() { fyne.Do(refresh) })`). All 8 tabs do `Snapshot()` /
`arrange()` CPU + allocation work each tick even though one is visible. (Hidden
charts' GPU *textures* aren't regenerated — they're generator-backed rasters not
in the scene graph — but the CPU/alloc work is.)

### C. The visible chart re-allocates raster buffers every frame
`internal/ui/linechart.go` (`renderSeries`, ~line 459) allocates a fresh
`image.NewRGBA(w,h)` plus a new `vector.NewRasterizer(w,h)` per series, per
refresh. The CPU chart (~14 series at ~1100×300) discards ~15–20 MB/s while
visible, which drives the GC target up; the OS scavenger returns pages slowly, so
RSS parks at the high-water mark.

---

## Findings — ranked

Tier = best thing to refactor for the return. **S** = highest value (dominant
lever on its axis); down to **F** = negligible / don't.

| # | Problem | Where | Fix | Est. improvement | Tier |
|---|---------|-------|-----|------------------|------|
| 1 | **Connection table enumerated every 1 s while hidden.** `gnet.Connections("all")` (4 Windows TCP/UDP table queries) runs every tick but only feeds the Ports + Connections tabs. | `monitor/processes.go:245`, `:364` | Collect connections only when a Ports/Connections tab is visible; otherwise every 3–5 s. | Large share of idle CPU | **S** |
| 2 | **Full per-process detail every 1 s.** `readProcess` queries Name/CPU/Mem/**Username**/Status for *all* ~N procs every tick (Username = per-PID SID→name resolution, the classic Windows cost). Overview only needs the *count*. | `monitor/processes.go:155`, `:190`, `:201` | Gate full detail to the visible process tab; keep the cheap PID-count for Overview's sparkline. | Large idle-CPU cut + less churn | **S** |
| 3 | **Chart raster reallocates buffers every frame.** `image.NewRGBA(w,h)` + a new `vector.NewRasterizer(w,h)` per series, per refresh (~14 series ⇒ ~15–20 MB/s discarded when visible). | `ui/linechart.go:459`, `:470`, `:478` | Cache `*image.RGBA`; reuse one rasterizer via `Reset` (memory-plan **Fix A**). | Kills dominant memory churn → big RSS drop | **S** |
| 4 | **All 8 tabs' refreshers fire every tick**, not just the visible one — each does `Snapshot()`/`arrange()` CPU + allocation off-screen. | `ui/shell.go:260`, `ui/app.go:151` | Refresh only the active tab; refresh on tab-switch (memory-plan **Fix B**). | ~8× less per-tick render/alloc work | **A** |
| 5 | **Username re-resolved every tick** even while a process tab *is* open (subset of #2 on the hot path). Username never changes for a live PID. | `monitor/processes.go:201` | Cache username per PID across ticks; only resolve new PIDs. | Big Windows payoff for ~10 lines | **A** |
| 6 | **Disk usage scanned every 1 s** — `Partitions(all)` + a `Usage` syscall per mount, for data that changes slowly. | `monitor/disk.go:58` | Sample usage every 5–10 s; keep the cheap I/O counters at 1 s. | Moderate idle-CPU cut | **B** |
| 7 | **Process list copied 3×/tick + name-map rebuilt 2×** — Ports and Connections adapters each call `Processes()` (full copy under lock) and rebuild a PID→name map; the process table copies it again. | `ui/app.go:117–129` | Build one snapshot + one name-map per tick; share it. | Mostly subsumed by #4 | **B** |
| 8 | **RSS parks high regardless** (Go scavenger returns pages slowly). | runtime | `GOMEMLIMIT` (~200 MiB) and/or periodic `debug.FreeOSMemory` — zero code (memory-plan **Fix C**). | Caps RSS at small CPU cost; stopgap, no root-cause fix | **C** |
| 9 | **Global 1 Hz poll / 60-sample ring buffer.** | `ui/app.go:26`, `metrics/capacity.go` | Lowering it would cut everything — but it's the spec'd 1 s-resolution, 1-min window. | Don't — degrades the product | **D** |
| 10 | **Micro-allocs** — `ringbuffer.Items()` copies (60 elems), per-cell string formatting. | `ringbuffer/ringbuffer.go:44` | — | Negligible at 1 Hz | **F** |

---

## Recommended sequence (lazy path, biggest bang first)

1. **S — collection gating + raster reuse (items 1–3).** The two dominant levers,
   one per axis. #1 + #2 share one mechanism: track the active tab and a
   "what this tab needs" map, then only run those collectors at 1 s, keeping a
   cheap always-on process-count for Overview. #3 is the contained
   `renderSeries` buffer-reuse edit already scoped in the memory plan.
2. **A — active-tab-only refresh + username cache (items 4–5).** Compound the
   above. #4 reuses the existing refresher registry — a change to *which*
   refreshers fire, **not** new cross-layer wiring (respects
   `dependency-inverted-layered-seams`; leave `Run()` untouched).
3. **C — `GOMEMLIMIT`** as an immediate stopgap while the above lands.
4. **Skip D / F.** Do not touch the poll rate or micro-allocs.

Expected end state (estimate, pending profile): idle CPU on a light tab drops to
~the three cheap syscalls (cpu%, mem, net) + one chart render; RSS heads from
~300 MB toward ~120–160 MB, lower with `GOMEMLIMIT`.

---

## Architecture notes for the implementer

- The active-tab signal is the shared dependency for #1, #2, #4. `selectIndex`
  in `ui/shell.go` already knows the active index — that is the hook. Surface it
  to the drive loop without adding a `ui → monitor` concrete import: pass the
  poller (or a small "collection policy" seam) the set of collectors the active
  tab needs, decided in the composition root (`app.go`), which is the only place
  that knows both the tabs and the concrete collectors.
- Keep the cheap "always-on" path (process *count* for the Overview sparkline)
  separate from the gated "full detail" path so Overview never forces a full
  per-process enumeration.
- Record the seam/decision in [`docs/ADR.md`](../ADR.md) if the collection-policy
  abstraction is introduced.

## Acceptance / verify

1. Add a temporary `runtime.ReadMemStats` logger (HeapAlloc / HeapInuse / Sys
   every few seconds). Capture baselines: idle on Overview, on CPU, on Processes,
   and after a `debug.FreeOSMemory()`. Also capture idle CPU% per tab (Task
   Manager or a profile) to confirm the #1/#2/#6 split before coding.
2. After **S**: HeapAlloc churn (and the GC target, hence RSS) drops sharply;
   idle CPU on Overview/CPU approaches the cheap-collector floor.
3. After **A**: per-tick allocation stops scaling with tab count; switching to a
   process tab still shows fresh data within one tick.
4. Lock wins in with benchmarks: `BenchmarkRenderSeries` (allocs/op down) and a
   poll-pass benchmark asserting the gated collectors don't run off the active
   tab. Remove the temporary memstats logger (or gate it behind a debug env var).

---

## References
- `internal/monitor/poller.go` — `collectAll` (~116), `tick` (~107).
- `internal/monitor/processes.go` — conn sampler `gnet.Connections` (~245),
  `Collect` (~364), `readProcess` (~190), `UsernameWithContext` (~201).
- `internal/monitor/disk.go` — `defaultDiskSampler` (~58).
- `internal/ui/app.go` — collector registration (~150), `OnTick` drive (~151),
  process/ports/conns adapters (~117–129), `pollInterval` (~26).
- `internal/ui/shell.go` — refresh closure / refresher registry (~260),
  `selectIndex` (~296).
- `internal/ui/linechart.go` — `renderSeries` (~459), `canvas.NewRaster` (~209).
- `docs/performance/memory-footprint-plan.md` — memory-side Fix A / B / C detail.
