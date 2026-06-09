# Code Flowmap — System Monitor

A visual map of the Go source files in this codebase and how they depend on one
another. Generated from the import graph and the orchestration entry points.

Module: `github.com/josephheinz/system-monitor`

---

## 1. Package dependency overview

How the top-level packages relate. Arrows point from dependent → dependency.

```mermaid
graph TD
    main["cmd/system-monitor<br/>main.go"]
    ui["internal/ui<br/>(Fyne UI)"]
    monitor["internal/monitor<br/>(collectors + poller)"]
    metrics["internal/metrics<br/>(stats + capacity)"]
    ring["internal/ringbuffer<br/>(in-memory history)"]
    genassets["tools/genassets<br/>(asset codegen)"]

    main --> ui
    ui --> monitor
    ui --> metrics
    monitor --> metrics
    monitor --> ring
    genassets -.->|writes assets_gen.go| ui

    subgraph external [external deps]
        gopsutil["gopsutil/v4"]
        fyne["fyne.io/fyne/v2"]
    end

    monitor --> gopsutil
    ui --> fyne
```

---

## 2. Application boot & runtime flow

The runtime wiring: `main` calls `ui.Run`, which builds the collectors, adapts
their histories into chart sources, wires the shell UI, and starts the poller.

```mermaid
flowchart TD
    main["main.go · main()"] --> run["ui/app.go · Run()"]

    run --> mkwin["ui/window.go<br/>defaultWindowSize()"]
    run --> collectors["construct collectors<br/>monitor.NewCPUCollector / ..."]
    run --> poller["monitor/poller.go<br/>NewPoller(interval, collectors...)"]
    run --> shell["ui/shell.go<br/>buildContent(liveSources)"]

    poller -->|"Start(ctx)"| tick["Poller.tick → collectAll"]
    tick --> ccol["each Collector.Collect(ctx)"]
    ccol --> ringwrite["append to ringbuffer"]
    poller -->|"OnTick(fn)"| refresh["UI refresh callback"]

    shell --> tabs["newTabs(src)"]
    tabs --> cputab["ui/cpu.go<br/>CPU chart + per-core"]
    tabs --> overview["ui/overview.go<br/>panel grid"]
    tabs --> placeholders["newPlaceholder()<br/>(other 6 tabs)"]
```

---

## 3. internal/monitor — collectors & poller

Each collector implements the `Collector` interface and writes samples into a
ring buffer. Shared math lives in `internal/metrics`.

```mermaid
graph TD
    poller["poller.go<br/>Poller · Collector interface"]
    cpu["cpu.go · CPUCollector"]
    mem["memory.go · MemoryCollector"]
    disk["disk.go · DiskCollector"]
    net["network.go · NetworkCollector"]
    proc["processes.go · ProcessCollector"]
    host["host.go · host info"]

    poller -.->|drives| cpu
    poller -.->|drives| mem
    poller -.->|drives| disk
    poller -.->|drives| net
    poller -.->|drives| proc

    metrics["metrics: stats.go · capacity.go"]
    ring["ringbuffer.go"]
    gops["gopsutil/v4<br/>cpu·mem·disk·net·process·host"]

    cpu --> metrics & ring & gops
    mem --> metrics & ring & gops
    disk --> metrics & ring & gops
    net --> metrics & ring & gops
    proc --> gops
    host --> gops
```

---

## 4. internal/ui — composition

`shell.go` assembles the chrome (title bar, sidebar, tabs, status bar). Visual
primitives (theme, typography, spacing, colorize) and asset loaders feed the
widgets. `linechart.go` and `cpu.go` render the data-driven views.

```mermaid
graph TD
    app["app.go · Run()"] --> shell["shell.go · buildContent / newTabs"]

    shell --> sidebar["sidebar.go"]
    shell --> overview["overview.go"]
    shell --> cpu["cpu.go"]
    shell --> layout["layout.go"]

    cpu --> linechart["linechart.go"]
    overview --> linechart

    subgraph primitives [visual primitives]
        theme["theme.go"]
        typo["typography.go"]
        spacing["spacing.go"]
        colorize["colorize.go"]
    end

    subgraph assets [asset loading]
        assetsf["assets.go"]
        fonts["fonts.go"]
        icons["icons.go"]
        gen["assets_gen.go<br/>(generated)"]
    end

    sidebar --> primitives
    overview --> primitives
    cpu --> primitives & metrics["internal/metrics"]
    linechart --> primitives

    theme --> assets
    typo --> fonts
    fonts --> gen
    icons --> gen
    assetsf --> gen
```

---

## 5. Test coverage map

Files with co-located `_test.go` suites.

```mermaid
graph LR
    subgraph tested [has tests]
        t1["metrics/stats.go"] --- ts1["stats_test.go"]
        t2["monitor/cpu.go"] --- ts2["cpu_test.go"]
        t3["monitor/disk.go"] --- ts3["disk_test.go"]
        t4["monitor/memory.go"] --- ts4["memory_test.go"]
        t5["monitor/network.go"] --- ts5["network_test.go"]
        t6["monitor/poller.go"] --- ts6["poller_test.go"]
        t7["monitor/processes.go"] --- ts7["processes_test.go"]
        t8["ringbuffer/ringbuffer.go"] --- ts8["ringbuffer_test.go"]
        t9["ui/cpu.go"] --- ts9["cpu_test.go"]
        t10["ui/linechart.go"] --- ts10["linechart_test.go"]
    end
```

---

## 6. Recommended modular split (SOLID-aligned)

The diagram below is a **proposal**, not the current state. It targets the
concrete pressure points in today's layout:

- **`linechart.go` (721 lines)** does four unrelated jobs: the public chart
  widget API, the renderer's layout/geometry, low-level vector rasterization
  (`strokePolyline` / `addPoly` / `addDisc` / `signedArea`), and numeric +
  time formatting (`niceNum` / `formatCompact` / `formatAge`). Four reasons to
  change in one file → **SRP** violation.
- **The `Source` interface lives inside `linechart.go`**, yet it is the seam
  between `monitor` (data) and `ui` (rendering). Today `ui` imports `monitor`
  directly and adapts each collector by hand in `Run()`. Pulling the abstraction
  into a neutral package lets both sides depend on it instead of on each
  other → **DIP**.
- **`app.go · Run()` hard-wires every collector→source field manually.** Adding
  a tab means editing `Run`, `liveSources`, and `shell` together → **OCP**
  friction. A small registry/provider seam makes new metric areas additive.

Proposed package boundaries (new/extracted nodes in blue, kept nodes in grey):

```mermaid
graph TD
    classDef new fill:#1b3a6b,stroke:#4679fa,color:#e7eaf0;
    classDef keep fill:#222a36,stroke:#344150,color:#9aa6b6;

    main["cmd/system-monitor<br/>composition root"]:::keep

    subgraph seam [internal/series — shared abstraction]
        srcif["Source interface<br/>+ sourceFrom / sourceFunc adapters"]:::new
    end

    subgraph mon [internal/monitor]
        poller["poller.go"]:::keep
        collectors["cpu · memory · disk<br/>network · processes"]:::keep
    end

    subgraph data [internal/metrics + ringbuffer]
        metrics["stats · capacity"]:::keep
        ring["ringbuffer"]:::keep
    end

    subgraph chart [internal/ui/chart]
        widget["linechart.go<br/>(widget API + options)"]:::new
        renderer["renderer.go<br/>(layout / geometry / axes)"]:::new
        raster["raster.go<br/>(stroke / poly / disc)"]:::new
        numfmt["format.go<br/>(niceNum / compact / age)"]:::new
    end

    subgraph chrome [internal/ui]
        app["app.go · Run()<br/>(thin wiring)"]:::keep
        shell["shell.go · tabs"]:::keep
        registry["tabs registry<br/>(provider per metric area)"]:::new
    end

    subgraph design [internal/ui/theme]
        tokens["tokens.go<br/>(palette / sizes)"]:::new
        theme["theme.go<br/>(Fyne theme impl)"]:::keep
        primitives["typography · spacing<br/>colorize · assets"]:::keep
    end

    main --> app
    app --> registry --> shell
    shell --> widget

    collectors --> metrics & ring
    collectors -->|expose| srcif
    poller -.->|drives| collectors

    widget --> srcif
    widget --> renderer --> raster
    renderer --> numfmt
    widget --> tokens
    shell --> primitives
    theme --> tokens
    primitives --> tokens
```

### How each move maps to SOLID

| Move | Principle | Payoff |
|------|-----------|--------|
| Split `linechart.go` → `chart/{linechart, renderer, raster, format}.go` | **S**RP | Each file changes for one reason; raster math testable without Fyne. |
| Extract `Source` + adapters into `internal/series` | **D**IP, **I**SP | `monitor` and `ui/chart` both depend on a tiny interface, not on each other. `Source` stays a one-method interface (already ISP-clean). |
| Tab **registry / provider** seam instead of `liveSources` struct edited per tab | **O**CP | New metric area = register a provider; `Run`/`shell` untouched. |
| Collectors expose `Source`s rather than `ui` reaching into histories | **L**SP, **D**IP | Any collector is substitutable behind the same data seam; the existing `Collector` interface already models this on the poll side. |
| Pull color/size **tokens** out of `theme.go` into `theme/tokens.go` | **S**RP | The design-system values (the CLAUDE.md quick-reference table) get one home, consumed by chart + chrome alike. |

**Sequencing note:** the lowest-risk first step is extracting `internal/series`
(pure interface + the two existing adapters move verbatim) since it unblocks the
`ui ↛ monitor` decoupling. The `linechart.go` split is mechanical (functions are
already grouped). The tab registry is the largest change and should come last.

---

_Sections 1–5 reflect the static import graph plus the boot/runtime call flow.
Dotted arrows denote codegen or runtime-driven (not compile-time) relationships.
Section 6 is a forward-looking recommendation, not the current structure._
