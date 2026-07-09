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

---

## ADR-005: Namespaced struct dictionaries for package-level constants

**Date:** 2026-06-03
**Status:** Active
**Area:** Architecture

### Context

The `internal/ui` package accumulated large families of related globals: ~22 color tokens, 6 font faces, 9 icons, custom theme size-name tokens, and the status kinds. As loose package-level vars/consts (`colorAccent`, `fontMonoRegular`, `iconOverview`, `sizeNameMetricValue`, `statusHealthy`, …) their origin was opaque at the call site — across files there was no signal that a bare identifier belonged to "the palette" or "the icon set," and the flat namespace invited collisions (the palette could not be named `color` because `image/color` owns that identifier).

### Decision

**Group each family of related package constants into a single struct-typed namespace var — `palette.X` (colors), `font.X` (faces), `icon.X` (icons), `sizeName.X` (custom theme size names), `status.X` (status kinds) — rather than loose package-level globals. The base-derived `space*` spacing scale (`spacing.go`) is the parallel standard for gaps/padding.**

### Rationale

A `dict.Field` read makes the origin obvious cross-file without import-path noise, namespaces away collisions (hence `palette` over `color`), and lets related values be defined and reviewed as one block. This is a plain, allocation-free Go idiom — a struct literal assigned to a package var — that changes only the *grouping*, not behavior: field types stay identical (`color.Color`, `fyne.Resource`, `fyne.ThemeSizeName`). Component dimensions that don't recur on the 4px grid stay as their own literal-px named consts rather than being forced onto the spacing scale, avoiding false coupling. The cost is a one-time sweep of every call site and a couple of local-variable renames to avoid shadowing the new namespaces (e.g. `status` → `statusRegion` in `buildContent`, `font` param → `fontSrc`).

---

## ADR-007: Source seam, linechart split, and tab registry (§6 phase 1)

**Date:** 2026-06-09
**Status:** Active
**Area:** Architecture

### Context

Three pressure points identified in CODE-FLOWMAP.md §6 were addressed together because they form a coherent dependency chain: extracting the `Source` seam enables the tab registry; splitting `linechart.go` reduces SRP debt; extracting tokens separates the design system from the Fyne binding layer.

### Decision

**Three mechanical refactors were applied as §6 phase 1: (1) `Source` interface + adapters extracted to `internal/series`; (2) `linechart.go` split into `format.go` (chart math) and `raster.go` (vector geometry); (3) design-system tokens extracted from `theme.go` into `tokens.go`; (4) `liveSources` converted from a struct to `map[tabID]series.Source` and the `newTabs` switch replaced with a `tabRegistry` lookup map.**

### Rationale

**(1) `internal/series` (DIP):** `Source` lived inside `internal/ui/linechart.go`, forcing `ui` to import `internal/monitor` concretely in `app.go` and hand-adapt each collector. Moving the interface to a neutral package lets both sides depend only on the seam. The composition root (`app.go`) retains the monitor import legitimately for collector construction; the cross-layer adaptation concern is gone.

**(2) `linechart.go` split (SRP):** The 722-line file had four unrelated reasons to change. `format.go` (pure math/string, no Fyne dependency) and `raster.go` (vector geometry, independently testable) are now separate files. The widget API + renderer remain in `linechart.go`. No logic was changed; this was a pure grouping move within `package ui`.

**(3) `tokens.go` extraction (SRP):** `theme.go` held both the Fyne theme implementation (`monitorTheme`, `themeColors`, `themeSizes`) and the design-system token dictionaries (`palette`, `sizeName`, `colorNameTextSecondary`, `rgb`). Two distinct reasons to change — a color update vs. a Fyne API change — now live in separate files.

**(4) Tab registry (OCP):** The `newTabs` switch required editing for every new live tab (alongside `liveSources` and `app.go`). `liveSources` is now `map[tabID]series.Source` and `tabRegistry` is a `map[tabID]tabBuilder`. Adding a new live tab = one registry entry in `tabRegistry` + one map assignment in `app.go`. `newTabs` is not edited.

**Remaining §6 work:** moving chart files to an `internal/ui/chart/` sub-package (requires tokens in a neutral sub-package to avoid a circular import) is deferred as §6 phase 2.

---

## ADR-008: fastwalk + background DiskUsageScanner seam for the directory treemap

**Date:** 2026-06-16
**Status:** Active
**Area:** Architecture

### Context

The Disk tab's "Storage — by directory" treemap (BZS253-52, Phase 2) needs the largest directories within a volume, sized by total bytes. Computing a directory's size requires walking its whole subtree (a dir's size is the sum of its descendants — display depth limits what's *shown*, not what's *walked*). A full-volume walk takes seconds, so it cannot run on the 1s poll tick that drives every other collector. We also had to choose a traversal library: `github.com/charlievieth/fastwalk` (a fast parallel walk primitive) vs `gdu`'s analyzer (which builds the size tree for us).

### Decision

**Add `fastwalk` and run the walks in a new `monitor.DiskUsageScanner` on its own background goroutine — crawling every volume once at launch (not per poll tick, which is far too fast for a multi-second walk), caching each result under its own volume root. The scanner exposes a mutex-guarded `[]DirSize` snapshot of the *selected* volume's cache (mirroring `DiskCollector.Usage()`); the size aggregation and tile selection are pure, unit-tested functions (`selectDirs` → `buildTree` + `dirNode.selectCells`). The UI consumes the snapshot through the `diskDirSource` seam and never walks the filesystem itself.**

### Rationale

fastwalk over gdu: accurate dir sizing needs the full subtree walk regardless, so gdu's tree-building buys little while adding a heavier dependency and a hard-link dedup pass that is invisible in a treemap. fastwalk gives us just the fast walk; we own the aggregation and selection, which is exactly the pure, testable core we wanted (tested against both synthetic trees and a real temp tree). Selection is not a flat top-N: `selectCells` runs a hallway-collapse (`representative` looks through single-child passthrough dirs) + greedy-budget expansion, breaking the largest still-meaningful directory into its child directories until a budget fills. Every tile is a real directory shown at its true subtree size; bytes that aren't a directory of their own are deliberately *not* drawn — files sitting directly in an expanded directory (a file isn't a directory) and subdirectories below the noise floor (lumping them into one box would hide what the treemap is for). Tiles therefore don't sum to the volume size, and that's intended: the Volumes bars beside the treemap carry the used/free total.

Per-volume cache, crawl-once cadence: each volume's snapshot is stored under its own root and `Dirs()` returns only `cache[selected]`, so selecting a volume (`SetRoot`) just changes which cache is read — it never starts a walk. This fixes a real bug in the earlier shared-`dirs` design, where a slow walk (e.g. a 1-minute `G:` crawl) finishing *after* the user had switched back to `C:` clobbered the displayed `C:` tiles with `G:` data. With per-volume caches a completing walk updates only its own entry, never the view. There is also no periodic re-crawl: volumes are walked once at launch (the displayed one first), and data refreshes only per launch — a deliberate choice to keep the walks predictable and switching instant.

Warm-start cache: the scanner persists the per-volume snapshot map to `diskcache.json` next to the executable and seeds from it on launch, so the tab shows the last run's tiles during the multi-second cold crawl instead of a blank "scanning…". This is a deliberate, narrow exception to the app's "no persistence" principle (which is about *metric history* — the ring buffers): it caches only the small per-volume tile lists, is strictly best-effort (a missing/corrupt/unwritable file is logged and ignored, never fatal), and is superseded by each launch's crawl. No free-space tile is drawn in the directory treemap — it shows directories only; free space already lives in the Volumes bars beside it.

The background-goroutine cadence keeps the expensive walk off the poll tick — the scanner is *not* a `Collector` and isn't registered with the `Poller`; its goroutine is owned by the app `ctx` and stops on window close. The `diskDirSource` seam keeps the dependency-inverted layering intact (UI → seam, never UI → gopsutil/fastwalk); the `diskScanController` that bridges the concrete scanner and the partition snapshot to the seam lives in the composition root (`app.go`), the only place that legitimately knows both. Deliberate simplification (ponytail): no hard-link dedup, and the walk materializes one `fileEntry` per file so the aggregation can stay a pure slice function — and `cellBudget` guards the expansion loop rather than hard-capping the final tile count — all carry `// ponytail:` comments naming the upgrade path (track `(dev,inode)`; build the tree in the walkFn; hard top-N truncate) should accuracy, memory, or a pathologically flat volume ever demand it.

---

## ADR-006: Typed tab IDs with self-describing tab definitions

**Date:** 2026-06-03
**Status:** Active
**Area:** Architecture

### Context

The shell hosts eight tabs (Overview, CPU, …, Connections) and routes nav selection to per-tab content. The scaffold described tabs with a `{name string; icon}` struct and selected content by matching the display string — `if d.name == "Overview"`. That couples routing to a human-facing label (a typo or rename silently breaks routing), gives the compiler nothing to check exhaustiveness against, and offers no structural place to attach the real per-tab content panes that are coming next.

### Decision

**Identify each tab by a typed `tabID` enum (`tabOverview … tabConnections`), and give `tabDef` an `id`, `name`, `icon`, and `content []fyne.CanvasObject` with an `addChild` method. A `newTabs()` builder declares identity then populates `content` by switching on `id`; `buildContent` ranges the result and builds each pane from `content`. No routing keys off the display string.**

### Rationale

Switching on a typed `id` decouples routing from labels (rename `name` freely; routing is unaffected) and turns content assignment into a `switch` the compiler and reader can reason about. The `content []fyne.CanvasObject` slice is the deliberate seam for wiring real multi-pane tab content later — single-pane tabs render identically today via a one-child `container.NewStack`. `newTabs()` is a builder (not a package-level value literal) specifically so `content` is built fresh per call and repeated `buildContent` invocations never double-append to a shared slice. The cost is a little more declaration ceremony than a flat string-keyed list, accepted for type safety and the extension point.

---

## ADR-007: Persisted settings via app.Preferences(); startup-applied; reversing "no settings screen"

**Date:** 2026-06-23
**Status:** Active
**Area:** Architecture / UI

### Context

BZS253-72 adds a Settings tab so users can adjust app preferences (start tab, poll cadence, GC memory cap, dark/light appearance) rather than relying on hardcoded defaults and `GOMEMLIMIT`. This reverses the MVP's explicit "No settings screen — out of scope" line (CLAUDE.md), now that the app is in its stretch phase. The app has no persistence for *metric history* by design (in-memory ring buffers), so a settings store had to be chosen, along with how/when each setting takes effect. Of the candidate settings, a theme/appearance toggle was kept but auto-update settings were deferred — their engine (BZS253-71) isn't built, and a control with nothing to drive would violate the "each setting takes effect" criterion; its typed keys can be added when that card lands.

### Decision

**Persist preferences through Fyne's built-in `app.Preferences()` — no config file or database. Typed keys, defaults, and accessors live in one home (`internal/ui/prefs.go`) behind a narrow consumer-defined `prefStore` interface (`fyne.Preferences` satisfies it structurally; tests use a map-backed fake). Every setting is read once at startup and applied then — documented "next launch" per row — not live. The light palette is selected by swapping the package-global `palette` var (and rebuilding `themeColors`) in `applyTheme` before any widget is constructed; the Settings tab is a normal `tabRegistry` entry.**

### Rationale

`app.Preferences()` adds no dependency and no file I/O of our own; it's the idiomatic Fyne store, and keys are namespaced in a `prefKey` dictionary (no loose string literals). The `prefStore` seam is one tiny interface defined at the consumer — fakeable without standing up a Fyne app — so the set→get→default round-trip is unit-tested directly. Startup-only application is the deliberate lazy choice: every widget reads the global `palette` at construction (not through theme indirection), so a *live* theme switch would require rebuilding the whole window — far more machinery than the value warrants. Reading prefs once and applying before the UI builds satisfies the acceptance criterion ("live or on next launch, documented") at a fraction of the cost; `pollInterval` likewise becomes a startup-set `var` (was a `const`) so the time axes and the status-bar poll label all describe the chosen cadence. Every accessor clamps out-of-range stored values to its default (no nonexistent start tab, no zero poll tick, no unknown palette), so a hand-edited or stale preference can't wedge the app. The light palette is a coherent first pass, not a design-reviewed artifact — the design system is dark-first and no light wireframe exists — and is marked as such in `tokens.go` for a future tuning pass.

---

## ADR-009: Self-update from GitHub Releases via the `internal/update` seam

**Date:** 2026-06-24
**Status:** Active
**Area:** Architecture / Infrastructure

### Context

BZS253-71 adds opt-in self-update: the app should notice a newer GitHub release and replace its own binary on user confirmation. This needs a build-time version to compare against (none existed), a place for the check/download/verify/swap logic that doesn't drag Fyne into testable code, a UI affordance, and a release pipeline that produces predictably-named, checksummed assets for the downloader to resolve. Releases had been manual; there was no `.github/` workflow. Updating a *running* binary is also OS-specific — Windows blocks deleting/overwriting a locked `.exe`, Unix does not.

### Decision

**Self-update lives in a Fyne-free `internal/update` leaf package (peer to `internal/monitor`), consumed by the UI through a `Controller` seam (`Snapshot`/`Check`/`Start`) wired only in the composition root; the binary is downloaded, never built on the user's machine. The version is stamped via `-ldflags "-X main.version=…"`; a non-release ("dev") build disables the feature. A 3-OS native-runner GitHub Actions matrix (`.github/workflows/release.yml`) builds per platform, names assets `system-monitor-<goos>-<goarch>[.exe]`, and publishes one `checksums.txt` — the naming being the contract the downloader resolves against. Binary swap is build-tagged: Windows renames the running exe to `<exe>.old` (cleaned up next launch) then moves the new one in; Unix renames over the running file directly.**

### Rationale

A leaf package keeps the network/semver/checksum logic unit-testable with a mocked `http.RoundTripper` and no Fyne app, and preserves layering — `update` imports only stdlib, the UI depends on it through two function-typed `buildSources` fields, and only `Run` knows the concrete `Controller`. No new dependency: stdlib `net/http`/`crypto/sha256` cover the work, and a ~15-line `vX.Y.Z` comparator avoids pulling in `golang.org/x/mod/semver` (escalate only if tags grow pre-release suffixes). Native runners sidestep Fyne's painful CGO cross-compilation — each OS builds itself. The Windows rename dance avoids a separate helper process (a running exe can be renamed even while locked); Unix needs no dance at all. macOS ships a bare `darwin-arm64` binary that self-updates like Linux — Developer ID signing/notarization and `.app`-bundle self-update are deferred to a future card, since the current audience is technical. All update activity is opt-in and surfaced in the status bar; the startup check is a non-blocking goroutine and any failure (offline, checksum mismatch, API error) degrades to a logged no-op with no retry loop.

---

## ADR-010: Opt-in auto-install (deviation from BZS253-71 "no silent replacement")

**Date:** 2026-06-24
**Status:** Active
**Area:** UI / Product

### Context

BZS253-71's acceptance criteria state "no silent background replacement" — every update is meant to be user-confirmed. After the initial click-to-update flow shipped, the product owner (the card reporter) asked for an *automatic* update option so the app can keep itself current without a click. A blanket auto-installer would contradict the AC; the question was how to add the convenience without reintroducing the surprise the AC guards against.

### Decision

**Auto-install is an opt-in Settings toggle (`autoUpdate`), OFF by default. With it off, behavior is unchanged: detect → show the banner/pill → install only on click. With it on, a found update is downloaded, verified, and installed on next launch with no click. The decision lives in the composition root (`Run` reads the pref and calls `Controller.Start` after a successful check); the `update.Controller` stays unaware of preferences.**

### Rationale

Making it opt-in and default-off preserves the AC's intent — nothing replaces the binary silently *unless the user explicitly turned that on* — so "opt-in/visible" still holds at the consent boundary even though the per-update click is waived. This is a conscious, recorded deviation from the literal AC, made by the card owner, not an accidental one. Keeping the policy in `Run` rather than the controller maintains the seam from ADR-009: the controller still just checks/installs on command, and the only new knowledge (read the pref, auto-start) sits with the composition root that already owns wiring. The auto path reuses the exact download→verify→swap→restart code, so it inherits the same checksum guard and graceful-failure behavior; a failed auto-install logs and no-ops like a manual one. Same-binary verification still applies — auto-install does not weaken the integrity check, only the human confirmation.

---

## ADR-011: Linux ships .deb + AppImage + bare binary; only the AppImage self-updates

**Date:** 2026-06-24
**Status:** Active
**Area:** Infrastructure / UI

### Context

The cross-platform story is a selling point, so the Linux release should offer real install formats, not just a raw binary. But ADR-009's self-update model — rename a downloaded binary over the running one — doesn't fit every Linux format: a `.deb` installs into root-owned `/usr/bin` under apt's control (the app can't and shouldn't swap it), while an AppImage is a single self-contained file that swaps exactly like a bare binary. Naming also matters: the in-app downloader resolves one asset name per platform from `runtime.GOOS/GOARCH`.

### Decision

**The Linux release publishes three artifacts from one native build: a bare binary (`system-monitor-linux-amd64`, manual/server use), a `.deb` (apt-managed, install-only), and an AppImage (`system-monitor-linux-amd64.AppImage`). In-app self-update on Linux targets the AppImage only: `assetName()` returns the `.AppImage`, and swap/restart/download operate on `$APPIMAGE` (the running AppImage file) rather than `os.Executable()` (which points into the read-only mount). `update.Supported()` gates the updater to AppImage launches — a bare-binary or `.deb` install isn't wired, so it shows no update affordance and manages its own updates. The `.deb` is built with nfpm and the AppImage with linuxdeploy in the release workflow.**

### Rationale

One native build feeds all three packagers, so there's no extra compile. Making the AppImage the self-update target is the natural fit — it's already a relocatable, user-writable single file, so the existing rename-swap works unchanged once it targets `$APPIMAGE` (a one-line indirection via `targetExecutable()`, shared by swap, restart, and the download dir so the temp file lands on the AppImage's volume for an atomic rename). Gating with `Supported()` avoids dishonest UI: a `.deb` user would otherwise see "update failed" when the swap hit a permission wall, so instead they see nothing and update through apt — the correct distro convention. nfpm (a Go binary, `go install`-able on the runner) drives the `.deb` from a small YAML with no FPM/Ruby; linuxdeploy bundles the GL/X11 shared libs the Fyne binary links, so the AppImage runs on machines without them. macOS code-signing/notarization remains deferred (ADR-009); this ADR only broadens Linux.

---

## ADR-012: Session tracking mode records metrics to a CSV file (scoped exception to no-persistence)

**Date:** 2026-07-08
**Status:** Active
**Area:** UI / Product

### Context

The no-persistence rule (ADR-001) is a defining constraint: metric history lives only in an in-memory ring buffer (~1 min at 1s), never on disk. But that same short window is a real limitation — a user who wants to catch a spike that happened while they were away, or capture a longer profiling run to inspect later, has nowhere for the data to go. BZS253-77 asks for an opt-in "tracking mode" that records one row of metrics per poll tick to a file for a session the user explicitly starts and stops.

### Decision

**Session tracking is a deliberate, scoped exception to the no-persistence rule. A user-started session appends one CSV row per poll tick (`internal/recorder`), stops on the user's command, and defaults OFF on every launch — it is session scope, not a persisted preference, so nothing records without an explicit start. The recorder registers as a third `OnTick` observer (after the redraw and the threshold watcher), reads each column through a plain `func() float64` supplied by the composition root, and writes through an `io.WriteCloser` — so its data path imports no Fyne, no monitor, and no UI, and is unit-tested without a running app. Output is CSV via stdlib `encoding/csv` (one header row, then `timestamp` + nine metric columns), `Flush()`ed per row so a crash mid-session keeps what was recorded. The file location is chosen by the user through the platform's native OS save dialog (`github.com/ncruces/zenity`) on start (default name `tracking-<timestamp>.csv`); zenity blocks, so it is invoked off the UI goroutine.**

### Rationale

The exception is narrow and honest: the no-persistence rule governs *ambient* history the app writes on its own; this is a *user-initiated export* of a session the user starts and stops, so "the app doesn't quietly keep your metrics" still holds — the only bytes on disk are ones the user asked for, to a location they picked. Defaulting off every launch (no persisted toggle) keeps the consent boundary at the same place ADR-010 put auto-update: nothing happens without an explicit act. CSV over a custom/binary format because it's grep-able, opens in any spreadsheet, and adds zero dependencies. The `OnTick` seam (ADR-007's neutral `series.Source` layering, extended by the poller's multi-observer tick) means no change to `Run()`'s collector wiring beyond registering one observer — the recorder is added, not woven in. Keeping the data path behind `io.WriteCloser` + `func() float64` rather than importing the series seam or a collector makes the "no Fyne in the data path" acceptance criterion structural, not just observed, and lets the test drive it with a `bytes.Buffer`. **Two recorded deviations from the card, both chosen by the card owner:** (1) the card scoped v1 as "default path, no chooser dialog yet" — the owner chose to ship a file picker now, so the user picks the location on start; (2) the picker is the platform's *native* OS dialog, which the card's "no new third-party dependency" AC cannot satisfy — Fyne's built-in `dialog` is drawn in-app, not native, and core Fyne has no native picker. So `github.com/ncruces/zenity` is added deliberately. zenity was chosen over alternatives (e.g. `sqweek/dialog`) for its cgo-free, cross-platform native dialogs (Windows via syscall, macOS via `osascript`, Linux via the `zenity`/`qarma` binary), keeping the build simple on the project's Windows + macOS targets. The recorder's data path is unaffected — it still takes only `io.WriteCloser`, so the new dependency lives solely in the composition root's toggle action.
