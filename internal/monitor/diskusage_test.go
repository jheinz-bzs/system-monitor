package monitor

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// testParams mirrors the production tuning but with a caller-set byte floor, so
// the selection logic can be exercised against small synthetic trees.
func testParams(floor uint64, budget int) selectParams {
	return selectParams{
		dominance:         dominanceThreshold,
		localSignificance: localSignificance,
		floorFraction:     0, // floor comes from minBytes alone in tests
		minBytes:          floor,
		cellBudget:        budget,
	}
}

// entry builds a fileEntry under root from path components, for synthetic trees.
func entry(root string, size int64, parts ...string) fileEntry {
	return fileEntry{path: filepath.Join(append([]string{root}, parts...)...), size: size}
}

// TestBuildTreeRollup checks the Phase −1 reconstruction: localSize lands in the
// immediate parent, subtreeSize rolls up, and depth counts from the root.
func TestBuildTreeRollup(t *testing.T) {
	root := filepath.FromSlash("/data")
	tree := buildTree([]fileEntry{
		entry(root, 100, "a", "f1"),
		entry(root, 50, "a", "sub", "f2"),
		entry(root, 25, "loose.bin"), // file directly under root
	}, root)

	if tree.subtreeSize != 175 {
		t.Errorf("root subtree = %d, want 175", tree.subtreeSize)
	}
	if tree.localSize != 25 {
		t.Errorf("root localSize = %d, want 25 (loose.bin)", tree.localSize)
	}
	a := tree.children["a"]
	if a == nil || a.subtreeSize != 150 || a.localSize != 100 || a.depth != 1 {
		t.Errorf("a = %+v, want subtree 150 / local 100 / depth 1", a)
	}
	if sub := a.children["sub"]; sub == nil || sub.subtreeSize != 50 || sub.depth != 2 {
		t.Errorf("a/sub = %+v, want subtree 50 / depth 2", sub)
	}
}

// TestRepresentativeCollapsesHallway confirms a dominated single-child chain
// collapses to the node where size actually concentrates.
func TestRepresentativeCollapsesHallway(t *testing.T) {
	root := filepath.FromSlash("/data")
	// root → home → joe → cache → (big file); every link is a hallway.
	tree := buildTree([]fileEntry{
		entry(root, 1000, "home", "joe", "cache", "big.bin"),
	}, root)

	rep := tree.representative(testParams(1, cellBudget))
	want := filepath.Join(root, "home", "joe", "cache")
	if rep.path != want {
		t.Errorf("representative = %q, want %q", rep.path, want)
	}
}

// TestRepresentativeStopsAtBranch confirms a node holding significant direct
// files is not looked through even when one child dominates.
func TestRepresentativeStopsAtBranch(t *testing.T) {
	root := filepath.FromSlash("/data")
	// home: one child dominates by bytes (900/1020 = 88% ≥ 85%), but home also
	// holds 120 of its own direct files (12% ≥ 10% local significance), so the
	// hallway rule must NOT look through it — it stays a branch point.
	tree := buildTree([]fileEntry{
		entry(root, 900, "home", "big", "f"),
		entry(root, 120, "home", "own.bin"),
	}, root)

	rep := tree.children["home"].representative(testParams(1, cellBudget))
	if rep.path != filepath.Join(root, "home") {
		t.Errorf("representative = %q, want the locally-heavy home dir", rep.path)
	}
}

// TestSelectCellsDropsLooseFilesAndSubFloor covers the full selection: the
// branch point gets expanded into its above-floor child directories, while a
// sub-floor subdirectory and files sitting directly in the root are dropped
// entirely (not lumped into an "(other)"/"(files)" tile) — every tile is a real
// directory.
func TestSelectCellsDropsLooseFilesAndSubFloor(t *testing.T) {
	root := filepath.FromSlash("/data")
	tree := buildTree([]fileEntry{
		entry(root, 1000, "big1", "x"),
		entry(root, 1000, "big2", "y"),
		entry(root, 10, "tiny", "z"),   // below floor 100 → dropped
		entry(root, 500, "direct.bin"), // a loose file at root → dropped (not a dir)
	}, root)

	got := tree.selectCells(testParams(100, cellBudget))
	want := []DirSize{
		{Path: filepath.Join(root, "big1"), Bytes: 1000},
		{Path: filepath.Join(root, "big2"), Bytes: 1000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("selectCells =\n  %v\nwant\n  %v", got, want)
	}
}

// TestSelectCellsBudgetStopsExpansion confirms the cell budget halts further
// expansion: a node that could be broken down stays whole once the budget fills.
func TestSelectCellsBudgetStopsExpansion(t *testing.T) {
	root := filepath.FromSlash("/data")
	// root branches into a (which itself branches), b, c. With budget 3, the
	// first expansion of root yields exactly 3 tiles and a is never expanded.
	tree := buildTree([]fileEntry{
		entry(root, 1000, "a", "sub1", "f"),
		entry(root, 1000, "a", "sub2", "f"),
		entry(root, 1500, "b", "f"),
		entry(root, 1200, "c", "f"),
	}, root)

	got := tree.selectCells(testParams(100, 3))
	want := []DirSize{
		{Path: filepath.Join(root, "a"), Bytes: 2000},
		{Path: filepath.Join(root, "b"), Bytes: 1500},
		{Path: filepath.Join(root, "c"), Bytes: 1200},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("selectCells budget=3 =\n  %v\nwant\n  %v", got, want)
	}
}

// TestCacheRoundTrip covers the warm-start cache: persist a snapshot map, then
// load it back into a fresh scanner. cachePath is injected so the test never
// touches the real executable directory.
func TestCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), cacheFileName)
	want := map[string][]DirSize{
		"C:\\": {{Path: "C:\\Users", Bytes: 100}, {Path: "C:\\Windows", Bytes: 50}},
	}

	(&DiskUsageScanner{cachePath: path}).persist(want)

	reader := &DiskUsageScanner{cachePath: path, cache: map[string][]DirSize{}}
	reader.loadCache()
	if !reflect.DeepEqual(reader.cache, want) {
		t.Errorf("loaded cache =\n  %v\nwant\n  %v", reader.cache, want)
	}
}

// TestLoadCacheMissingFileIsNoop confirms a cold start (no cache file yet)
// leaves the cache empty instead of failing.
func TestLoadCacheMissingFileIsNoop(t *testing.T) {
	s := &DiskUsageScanner{
		cachePath: filepath.Join(t.TempDir(), "absent.json"),
		cache:     map[string][]DirSize{},
	}
	s.loadCache()
	if len(s.cache) != 0 {
		t.Errorf("cache = %v, want empty after a missing file", s.cache)
	}
}

// TestDirsReadsSelectedVolume guards the per-volume isolation: Dirs returns the
// selected volume's own cache, and SetRoot switches volumes without ever showing
// another volume's tiles (the stale-volume clobber bug).
func TestDirsReadsSelectedVolume(t *testing.T) {
	s := &DiskUsageScanner{
		cache: map[string][]DirSize{
			"C:\\": {{Path: "C:\\Users", Bytes: 100}},
			"G:\\": {{Path: "G:\\Games", Bytes: 200}},
		},
		selected: "C:\\",
	}

	if got := s.Dirs(); len(got) != 1 || got[0].Path != "C:\\Users" {
		t.Errorf("Dirs on C: = %v, want [C:\\Users]", got)
	}
	s.SetRoot("G:\\")
	if got := s.Dirs(); len(got) != 1 || got[0].Path != "G:\\Games" {
		t.Errorf("Dirs after SetRoot(G:) = %v, want [G:\\Games]", got)
	}
}

// TestWalkRoot covers the mountpoint → walkable-root normalization: a bare
// Windows volume name gains the separator that makes it the drive root, while
// directory paths pass through unchanged.
func TestWalkRoot(t *testing.T) {
	sep := string(filepath.Separator)
	cases := map[string]string{
		"C:":       "C:" + sep, // bare volume → drive root, not the current dir on C:
		"C:" + sep: "C:" + sep, // already a root
		"":         "",         // no volume selected
	}
	for in, want := range cases {
		if got := walkRoot(in); got != want {
			t.Errorf("walkRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestWalkAndSelect exercises the real walk over a temp tree with known byte
// totals, then selects — covering walkFiles + selectDirs end to end. A minimal
// floor keeps every directory in play so the assertion is about aggregation,
// not the noise filter; the loose root file is dropped because it's not a dir.
func TestWalkAndSelect(t *testing.T) {
	root := t.TempDir()
	write := func(rel string, n int) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, bytes.Repeat([]byte{'x'}, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("big/file1", 1000)
	write("big/sub/file2", 500) // big → 1500
	write("small/file3", 100)   // small → 100
	write("loose.bin", 300)     // a loose file at root → dropped (not a directory)

	entries, report := walkFiles(context.Background(), root)
	if report.files != 4 || report.bytes != 1900 {
		t.Errorf("walk report = %d files / %d bytes, want 4 / 1900 (a file per written inode)", report.files, report.bytes)
	}
	got := selectDirs(entries, root, testParams(1, cellBudget))
	want := []DirSize{
		{Path: filepath.Join(root, "big"), Bytes: 1500},
		{Path: filepath.Join(root, "small"), Bytes: 100},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("walk+select =\n  %v\nwant\n  %v", got, want)
	}
}

// TestWalkAndSelectDedupsHardLinks confirms a file reachable through two hard
// links is counted once, not once per path: one and two hold links to the same
// inode, so the treemap shows a single 1000-byte tile, never two 1000-byte
// tiles.
func TestWalkAndSelectDedupsHardLinks(t *testing.T) {
	root := t.TempDir()
	dir1 := filepath.Join(root, "one")
	dir2 := filepath.Join(root, "two")
	for _, dir := range []string{dir1, dir2} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir1, "data.bin"), bytes.Repeat([]byte{'x'}, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir1, "data.bin"), filepath.Join(dir2, "data.bin")); err != nil {
		t.Skipf("hard links unsupported: %v", err)
	}

	entries, report := walkFiles(context.Background(), root)
	if report.files != 1 || report.bytes != 1000 {
		t.Errorf("walk report = %d files / %d bytes, want 1 / 1000 (the inode once, not both links)", report.files, report.bytes)
	}
	got := selectDirs(entries, root, testParams(1, cellBudget))
	if len(got) != 1 {
		t.Fatalf("got %v, want exactly one tile (the inode counted once)", got)
	}
	if got[0].Bytes != 1000 {
		t.Errorf("counted %d bytes, want 1000 (one link, not 2000 across both)", got[0].Bytes)
	}
}

// TestCrawlMissingSkipsCachedVolumes is the core of "only scan if the cache
// doesn't exist": a volume already present in the warm-start cache must not be
// re-walked at launch, so a re-launch with a populated cache does no filesystem
// work. The sentinel would be overwritten (to nil) if the volume were re-walked.
func TestCrawlMissingSkipsCachedVolumes(t *testing.T) {
	root := t.TempDir()
	walk := walkRoot(root)
	sentinel := []DirSize{{Path: "sentinel", Bytes: 42}}
	s := &DiskUsageScanner{
		ctx:      context.Background(),
		cache:    map[string][]DirSize{walk: sentinel},
		scanning: map[string]bool{},
		reports:  map[string]walkReport{},
	}

	s.crawlMissing([]string{root})

	got := s.cache[walk]
	if len(got) != 1 || got[0].Path != "sentinel" {
		t.Errorf("cached volume was re-walked: cache = %v, want sentinel preserved", got)
	}
}

// TestCrawlMissingScansUncachedVolume is the complement: a volume with no cached
// snapshot is walked at launch, creating its cache entry (present even when the
// walk finds nothing above the floor — an empty temp dir here).
func TestCrawlMissingScansUncachedVolume(t *testing.T) {
	root := t.TempDir()
	walk := walkRoot(root)
	s := &DiskUsageScanner{
		ctx:      context.Background(),
		cache:    map[string][]DirSize{},
		selected: walk,
		scanning: map[string]bool{},
		reports:  map[string]walkReport{},
	}

	s.crawlMissing([]string{root})

	if _, ok := s.cache[walk]; !ok {
		t.Error("uncached volume was not scanned: no cache entry created")
	}
	if r := s.Report(); r.root != walk {
		t.Errorf("Report() = %+v, want a walk record for %q", r, walk)
	}
}

// TestWalkFilesCountsSkippedUnreadable covers the visibility fix: a directory
// the user cannot read must be counted in the walk report's skipped tally, not
// vanish silently. The readable siblings are still counted, so a partial crawl
// is now visible instead of looking like a complete one.
func TestWalkFilesCountsSkippedUnreadable(t *testing.T) {
	root := t.TempDir()
	write := func(rel string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte{'x'}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("open/a")
	write("open/b")
	write("locked/c")
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) }) // let TempDir's cleanup delete it

	entries, report := walkFiles(context.Background(), root)
	if report.skipped < 1 {
		t.Errorf("skipped = %d, want >= 1 (the locked dir), report = %+v", report.skipped, report)
	}
	if report.files != 2 || report.bytes != 2 {
		t.Errorf("counted %d files / %d bytes, want 2 / 2 (only the readable siblings)", report.files, report.bytes)
	}
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2 (the locked subtree is absent)", len(entries))
	}
}

// TestReportZeroForCachedOnlyVolume is the stale-cache indicator: a volume whose
// snapshot came only from the warm-start cache has never been walked this
// session, so Report() returns the zero value (root "") — a UI can distinguish
// "fresh crawl" from "shown from a previous run's cache" by that.
func TestReportZeroForCachedOnlyVolume(t *testing.T) {
	walk := walkRoot(filepath.Join(t.TempDir(), "vol"))
	s := &DiskUsageScanner{
		cache:    map[string][]DirSize{walk: {{Path: "old", Bytes: 1}}},
		selected: walk,
		reports:  map[string]walkReport{},
	}
	if r := s.Report(); r != (walkReport{}) {
		t.Errorf("Report() = %+v, want the zero report (no session walk yet)", r)
	}
}
