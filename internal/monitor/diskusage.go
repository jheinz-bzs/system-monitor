package monitor

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/charlievieth/fastwalk"
)

// Directory-selection tuning (see dirNode.selectCells). A directory counts as a
// passthrough "hallway" — looked through, not shown — only when one child holds
// at least dominanceThreshold of its bytes AND it holds less than
// localSignificance of the bytes in its own direct files. Directories below the
// noise floor max(minFloorBytes, floorFraction*total) are dropped; cellBudget
// caps how many tiles the selection grows to.
const (
	dominanceThreshold = 0.85
	localSignificance  = 0.10
	floorFraction      = 0.01
	minFloorBytes      = 50 << 20 // 50 MiB absolute floor, so small disks don't surface junk
	cellBudget         = 16
	minExpandTiles     = 2 // a dir is worth expanding only if it yields ≥2 above-floor tiles
)

// cacheFileName is the warm-start snapshot written under the OS per-user cache
// directory, in cacheDirName. A full-volume walk takes seconds, so on launch
// the scanner shows the previous run's tiles from this file while the fresh
// crawl runs — the treemap is never blank waiting on a cold walk. cacheFileMode
// and cacheDirMode are their permission bits.
const (
	cacheFileName = "diskcache.json"
	cacheDirName  = "system-monitor"
	cacheFileMode = 0o644
	cacheDirMode  = 0o755
)

// DirSize is one treemap tile chosen by the selection algorithm: the absolute
// path of a real directory and its summed subtree size in bytes.
type DirSize struct {
	Path  string
	Bytes uint64
}

// fileEntry is one walked file: its absolute path and size in bytes. It is the
// input to selectDirs, kept separate from the walk so the aggregation stays a
// pure, testable function.
type fileEntry struct {
	path string
	size int64
}

// fileID uniquely identifies one file's data on a volume, so two paths that
// hard-link to the same inode collapse to one entry. volume is the device
// (Unix) or volume serial (Windows); index is the inode (Unix) or file index
// (Windows).
type fileID struct {
	volume uint64
	index  uint64
}

// DiskUsageScanner exposes the largest directories of the currently selected
// volume as a snapshot, mirroring the DiskCollector.Usage() pattern (mutex-
// guarded, copied on read). Each volume's result is cached under its own root
// and persisted, so switching volumes (SetRoot) only changes which cache is read
// — it never triggers a walk, and a slow walk can never land on the wrong
// volume's view.
//
// At launch it walks only the volumes missing from the warm-start cache: a
// re-launch with a populated cache does no filesystem walking at all (the walk
// is the app's heaviest startup cost). Refreshing a cached volume is then an
// explicit, on-demand Rescan — there is no periodic or automatic re-crawl. The
// zero value is not usable; build one with NewDiskUsageScanner.
type DiskUsageScanner struct {
	mu sync.RWMutex

	// ctx governs every walk (launch crawl and on-demand rescans); a cancelled
	// ctx stops an in-flight walk and prevents new ones from caching.
	ctx context.Context

	// selected is the displayed volume root; Dirs returns cache[selected].
	selected string

	// cache is the latest snapshot per volume root, persisted to cachePath so a
	// fresh launch shows the previous run's tiles immediately. A volume's
	// presence here (any value, even nil) means "already scanned" — the launch
	// crawl skips it. cachePath is "" when the executable can't be located, in
	// which case caching is silently skipped.
	cache     map[string][]DirSize
	cachePath string

	// scanning marks volume roots with an in-flight walk, so the launch crawl and
	// a manual rescan can't redundantly walk the same volume at once.
	scanning map[string]bool
}

// NewDiskUsageScanner builds a scanner seeded from the on-disk cache (so the
// treemap shows the previous run's tiles immediately), then walks only the roots
// not already cached, in the background. roots[0] is the initially displayed
// volume and is crawled first so the visible treemap fills first. Walks stop
// when ctx is cancelled.
func NewDiskUsageScanner(ctx context.Context, roots []string) *DiskUsageScanner {
	s := &DiskUsageScanner{
		ctx:       ctx,
		cache:     map[string][]DirSize{},
		cachePath: cachePath(),
		scanning:  map[string]bool{},
	}
	if len(roots) > 0 {
		s.selected = walkRoot(roots[0])
	}
	s.loadCache()
	go s.crawlMissing(roots)
	return s
}

// walkRoot turns a volume mountpoint into a walkable directory path. gopsutil
// reports a Windows drive as a bare volume name ("C:"), which the OS resolves
// to the *current directory* on that drive, not its root — so a bare volume
// name gets the trailing separator that makes it the drive root ("C:\"). Real
// directory mounts (Unix "/", "/data") have no bare volume name and pass
// through unchanged.
func walkRoot(mount string) string {
	if mount != "" && filepath.VolumeName(mount) == mount {
		return mount + string(filepath.Separator)
	}
	return mount
}

// SetRoot selects the volume whose cached snapshot Dirs returns. It does not
// trigger a walk — the volume was already crawled at launch — so switching is
// instant and shows only that volume's own data.
func (s *DiskUsageScanner) SetRoot(root string) {
	walk := walkRoot(root)
	s.mu.Lock()
	s.selected = walk
	s.mu.Unlock()
}

// Dirs returns a copy of the selected volume's directory snapshot, largest
// first (empty until that volume's crawl lands and no prior cache exists).
func (s *DiskUsageScanner) Dirs() []DirSize {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.cache[s.selected]
	out := make([]DirSize, len(src))
	copy(out, src)
	return out
}

// crawlMissing walks only the volumes with no cached snapshot yet, in order
// (roots[0], the initially displayed volume, first) so the visible treemap fills
// first. A volume already in the warm-start cache is left untouched — only an
// explicit Rescan refreshes it — so a re-launch with a populated cache does no
// filesystem walking. Stops when ctx is cancelled.
func (s *DiskUsageScanner) crawlMissing(roots []string) {
	for _, root := range roots {
		if s.ctx.Err() != nil {
			return
		}
		walk := walkRoot(root)
		if walk == "" {
			continue
		}
		s.mu.RLock()
		_, cached := s.cache[walk]
		s.mu.RUnlock()
		if cached {
			continue // warm-start cache already covers this volume
		}
		s.scan(walk)
	}
}

// Rescan launches a fresh background walk of the currently selected volume,
// replacing its cached snapshot when it lands — the manual refresh the Disk
// tab's scan button triggers. It returns immediately; a walk already in flight
// for that volume is a no-op (the in-progress guard in scan).
func (s *DiskUsageScanner) Rescan() {
	s.mu.RLock()
	walk := s.selected
	s.mu.RUnlock()
	go s.scan(walk)
}

// scan walks one already-resolved volume root, replacing its cached snapshot
// under its own root — never a shared "current" slot, so a slow walk finishing
// after a volume switch updates only its own cache, never the displayed view —
// and persisting the result. It guards against a second concurrent walk of the
// same root (the launch crawl racing a manual rescan) so the work isn't doubled.
func (s *DiskUsageScanner) scan(walk string) {
	if walk == "" || !s.beginScan(walk) {
		return
	}
	defer s.endScan(walk)

	dirs := selectDirs(walkFiles(s.ctx, walk), walk, defaultSelectParams)
	if s.ctx.Err() != nil {
		return // shutting down mid-walk; the partial result is meaningless
	}

	s.mu.Lock()
	s.cache[walk] = dirs
	// Clone so the marshal runs outside the lock; slice values are replaced
	// wholesale (never mutated in place), so sharing backing arrays is safe.
	snapshot := maps.Clone(s.cache)
	s.mu.Unlock()

	s.persist(snapshot)
}

// beginScan claims the walk slot for walk, reporting false when one is already
// in flight so the caller backs off.
func (s *DiskUsageScanner) beginScan(walk string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scanning[walk] {
		return false
	}
	s.scanning[walk] = true
	return true
}

// endScan releases the walk slot for walk.
func (s *DiskUsageScanner) endScan(walk string) {
	s.mu.Lock()
	delete(s.scanning, walk)
	s.mu.Unlock()
}

// loadCache reads the persisted snapshot map into s.cache. Best-effort: a
// missing or corrupt file just leaves the cache empty, and the first walk
// repopulates it. Called once before the goroutine starts, so no lock is needed.
func (s *DiskUsageScanner) loadCache() {
	if s.cachePath == "" {
		return
	}
	data, err := os.ReadFile(s.cachePath)
	if err != nil {
		return
	}
	var cached map[string][]DirSize
	if err := json.Unmarshal(data, &cached); err == nil && cached != nil {
		s.cache = cached
	}
}

// persist writes the snapshot map next to the executable. Best-effort: encode
// or write failures are logged and dropped, since the cache is only a warm-start
// nicety and never required for correctness.
func (s *DiskUsageScanner) persist(cache map[string][]DirSize) {
	if s.cachePath == "" {
		return
	}
	data, err := json.Marshal(cache)
	if err != nil {
		slog.Warn("disk usage cache encode", "err", err)
		return
	}
	if err := os.WriteFile(s.cachePath, data, cacheFileMode); err != nil {
		slog.Warn("disk usage cache write", "path", s.cachePath, "err", err)
	}
}

// cachePath returns the cache file path under the OS per-user cache directory,
// creating the app's subdirectory, or "" when either is unavailable (then
// caching is silently skipped). The file must NOT live next to the executable:
// system installs (the .deb's /usr/bin) are read-only for the user, so the
// cache would silently never persist.
func cachePath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(base, cacheDirName)
	if err := os.MkdirAll(dir, cacheDirMode); err != nil {
		return ""
	}
	return filepath.Join(dir, cacheFileName)
}

// walkFiles traverses root in parallel with fastwalk, returning every regular
// file's path and size. Unreadable entries (permission denied, vanished mid-
// walk) are skipped rather than failing the walk, mirroring the collector's
// "skip unreadable mount" stance; the walk stops early if ctx is cancelled.
//
// ponytail: this materializes one fileEntry per file so selectDirs can stay a
// pure function over a slice. On a multi-million-file volume that transient
// slice is sizeable; if it ever matters, build the dirNode tree directly inside
// the walkFn and drop the slice.
func walkFiles(ctx context.Context, root string) []fileEntry {
	var (
		mu      sync.Mutex
		entries []fileEntry
		seen    = map[fileID]struct{}{}
	)
	// fastwalk runs walkFn from several goroutines, so the append must be
	// guarded; the work is I/O-bound, so the lock contention is negligible.
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entry
		}
		if ctx.Err() != nil {
			return ctx.Err() // shutting down; abandon the walk
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		// A file reachable through several hard links is one inode; count it
		// once, whichever path the walk hits first. Only files with more than
		// one link need the identity lookup, so the common case stays out of
		// the seen map.
		id, dedup := fileIdentity(path, info)
		mu.Lock()
		if dedup {
			if _, dup := seen[id]; dup {
				mu.Unlock()
				return nil // same inode already counted via another path
			}
			seen[id] = struct{}{}
		}
		entries = append(entries, fileEntry{path: path, size: info.Size()})
		mu.Unlock()
		return nil
	}

	if err := fastwalk.Walk(&fastwalk.Config{}, root, walkFn); err != nil && ctx.Err() == nil {
		slog.Warn("disk usage walk", "root", root, "err", err)
	}
	return entries
}

// selectParams tunes the directory-selection algorithm. It exists so the pure
// selection can be exercised with a low floor in tests; production uses
// defaultSelectParams (the named tuning consts above).
type selectParams struct {
	dominance         float64
	localSignificance float64
	floorFraction     float64
	minBytes          uint64
	cellBudget        int
}

var defaultSelectParams = selectParams{
	dominance:         dominanceThreshold,
	localSignificance: localSignificance,
	floorFraction:     floorFraction,
	minBytes:          minFloorBytes,
	cellBudget:        cellBudget,
}

// selectDirs turns the flat list of walked files into the set of directories to
// show as treemap tiles. It builds a size-aggregated directory tree and runs a
// hallway-collapse + greedy-budget selection over it (see dirNode.selectCells).
// It is pure (no Fyne, no OS calls) so the whole pipeline is unit-testable.
//
// Hard-link dedup happens upstream in walkFiles (one entry per inode), so the
// entries here never double-count a file reachable by several paths.
func selectDirs(entries []fileEntry, root string, p selectParams) []DirSize {
	return buildTree(entries, root).selectCells(p)
}

// dirNode is one directory in the scanned tree: bytes held directly in it
// (localSize) versus across its whole subtree (subtreeSize), plus its child
// directories keyed by base name. The tree is reconstructed from the files it
// contains, so a directory with no files anywhere in its subtree never appears
// — exactly what a size treemap wants.
type dirNode struct {
	path        string
	depth       int
	localSize   uint64
	subtreeSize uint64
	children    map[string]*dirNode
}

// buildTree reconstructs the directory tree from walked files: each file's size
// lands in its immediate parent's localSize, intermediate directories are
// created on the way down, then a post-order pass rolls localSize up into
// subtreeSize.
func buildTree(entries []fileEntry, root string) *dirNode {
	rootNode := &dirNode{path: root, children: map[string]*dirNode{}}
	for _, e := range entries {
		rel, err := filepath.Rel(root, e.path)
		if err != nil || rel == "." {
			rootNode.localSize += uint64(e.size) // file at root, or off-root fallback
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		node := rootNode
		// All but the last component are directories; the last is the file.
		for _, name := range parts[:len(parts)-1] {
			child := node.children[name]
			if child == nil {
				child = &dirNode{
					path:     filepath.Join(node.path, name),
					depth:    node.depth + 1,
					children: map[string]*dirNode{},
				}
				node.children[name] = child
			}
			node = child
		}
		node.localSize += uint64(e.size)
	}
	rootNode.rollUp()
	return rootNode
}

// rollUp fills subtreeSize for this node and all descendants (post-order).
func (n *dirNode) rollUp() uint64 {
	total := n.localSize
	for _, c := range n.children {
		total += c.rollUp()
	}
	n.subtreeSize = total
	return total
}

// nodeLess orders directory nodes largest-subtree first, ties broken by path —
// the one ordering rule behind every size-ranked decision (largest child,
// sorted children, most-expandable frontier node), so all of them stay
// deterministic.
func nodeLess(a, b *dirNode) bool {
	if a.subtreeSize != b.subtreeSize {
		return a.subtreeSize > b.subtreeSize
	}
	return a.path < b.path
}

// largestChild returns the child with the most subtree bytes (ties broken by
// path for determinism), or nil when the node has no children.
func (n *dirNode) largestChild() *dirNode {
	var best *dirNode
	for _, c := range n.children {
		if best == nil || nodeLess(c, best) {
			best = c
		}
	}
	return best
}

// sortedChildren returns the children largest-subtree first, ties broken by path.
func (n *dirNode) sortedChildren() []*dirNode {
	kids := make([]*dirNode, 0, len(n.children))
	for _, c := range n.children {
		kids = append(kids, c)
	}
	sort.Slice(kids, func(i, j int) bool { return nodeLess(kids[i], kids[j]) })
	return kids
}

// representative collapses a chain of single-child "hallway" directories down to
// the first node that either branches or holds significant bytes of its own — so
// "/ → /home → /home/joe → .cache/big" surfaces the real offender instead of a
// chain of nested ancestors.
func (n *dirNode) representative(p selectParams) *dirNode {
	for n.subtreeSize > 0 {
		largest := n.largestChild()
		if largest == nil {
			break
		}
		ratio := float64(largest.subtreeSize) / float64(n.subtreeSize)
		localShare := float64(n.localSize) / float64(n.subtreeSize)
		if ratio >= p.dominance && localShare < p.localSignificance {
			n = largest // a hallway — look through it
			continue
		}
		break // branch point or locally-heavy dir — stop
	}
	return n
}

// expandable reports whether breaking this node into its children would yield at
// least two tiles above the floor; otherwise expanding adds no information and
// the node stays a single tile (e.g. a dir that is big from one huge file).
func (n *dirNode) expandable(floor uint64, p selectParams) bool {
	meaningful := 0
	for _, c := range n.children {
		if c.representative(p).subtreeSize >= floor {
			meaningful++
			if meaningful >= minExpandTiles {
				return true
			}
		}
	}
	return false
}

// selectCells runs the budgeted greedy selection and returns the chosen
// directory tiles, largest first. It starts from the collapsed root and keeps
// breaking the largest still-meaningful directory into its children, so a few
// dominant directories give way to the specific subdirectories inside them.
// Every node entering the frontier has passed through representative(), so none
// is a hallway and none is an ancestor of another — the frontier is disjoint by
// construction, and every tile is a real directory shown at its true subtree
// size.
//
// Bytes that aren't a directory of their own are deliberately not drawn: files
// sitting directly in an expanded directory (a file is not a directory) and
// subdirectories below the noise floor (lumping them into one tile hides what
// the treemap is for). Tiles therefore need not sum to the volume size — the
// Volumes bars beside the treemap carry the used/free total.
//
// ponytail: cellBudget guards the expansion loop, not the final count — one
// expansion of a very flat directory can emit more tiles than the budget. The
// floor keeps that bounded in practice; add a hard top-N truncate only if a
// real volume ever overflows it.
func (root *dirNode) selectCells(p selectParams) []DirSize {
	total := root.subtreeSize
	if total == 0 {
		return nil
	}
	floor := p.minBytes
	if frac := uint64(p.floorFraction * float64(total)); frac > floor {
		floor = frac
	}

	frontier := []*dirNode{root.representative(p)}
	for len(frontier) < p.cellBudget {
		idx := largestExpandable(frontier, floor, p)
		if idx < 0 {
			break // nothing left worth breaking apart
		}
		f := frontier[idx]
		frontier = append(frontier[:idx], frontier[idx+1:]...)

		for _, c := range f.sortedChildren() {
			if r := c.representative(p); r.subtreeSize >= floor {
				frontier = append(frontier, r)
			}
			// Sub-floor children, and f's own loose files, are dropped rather
			// than shown — they are not directories worth a tile of their own.
		}
	}

	// Largest first; on a byte tie prefer the deeper (more specific) path, then
	// path text — fully deterministic, so the same tree always renders the same.
	sort.Slice(frontier, func(i, j int) bool {
		a, b := frontier[i], frontier[j]
		if a.subtreeSize != b.subtreeSize {
			return a.subtreeSize > b.subtreeSize
		}
		if a.depth != b.depth {
			return a.depth > b.depth
		}
		return a.path < b.path
	})

	out := make([]DirSize, len(frontier))
	for i, n := range frontier {
		out[i] = DirSize{Path: n.path, Bytes: n.subtreeSize}
	}
	return out
}

// largestExpandable returns the index of the frontier node with the most subtree
// bytes that is still worth breaking apart, or -1 if none is. Ties broken by
// path for determinism.
func largestExpandable(frontier []*dirNode, floor uint64, p selectParams) int {
	best := -1
	for i, n := range frontier {
		if !n.expandable(floor, p) {
			continue
		}
		if best < 0 || nodeLess(n, frontier[best]) {
			best = i
		}
	}
	return best
}
