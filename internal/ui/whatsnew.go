package ui

// "What's New" page (BZS253-78): a fullscreen overlay shown once on the first
// launch after the app version changes, rendering a bundled changelog. It reuses
// the self-update build version (internal/update carries it into ui.Run) and the
// typed settings layer — one lastSeenVersion key drives the whole "don't show
// again until updated" behavior. A single compare (current != lastSeen) is the
// mechanism; dismissing records the current version.
//
// The changelog ships in the binary as a bundled asset (whatsnew.md, compiled by
// tools/genassets like the fonts/icons) and renders via Fyne's Markdown rich
// text — no network fetch, no new dependency, no HTML.

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// whatsNewAsset is the bundled changelog's key under internal/ui (loaded through
// resource(), like every other bundled asset).
const whatsNewAsset = "whatsnew.md"

const labelWhatsNewClose = "Close"

// whatsNewBullet is the indented bullet prefix that gives the changelog the
// left-indented list look of a conventional markdown viewer — Fyne's built-in
// markdown renders top-level bullets flush with the body margin. The leading
// spaces are the indent (the body face is proportional, so this is approximate
// but reads clearly); "• " is the marker.
const whatsNewBullet = "    • "

// whatsNewEnv force-shows the page on every launch when set, ignoring the
// version compare and without recording lastSeenVersion — a dev affordance to
// preview the overlay without bumping the build version (mirrors SYSMON_MEMSTATS).
const whatsNewEnv = "SYSMON_WHATSNEW"

// changelogMarkdown returns the bundled changelog text, trimmed. An empty result
// (a build that shipped an empty file) reads as "no changelog" so the page is
// skipped rather than showing a blank overlay.
func changelogMarkdown() string {
	return strings.TrimSpace(string(resource(whatsNewAsset).Content()))
}

// whatsNewDecision decides, at launch, whether to show the "What's New" overlay
// and whether to record the current version now:
//
//   - unchanged since last launch (current == lastSeen) → neither.
//   - fresh install (lastSeen unset) → record silently, never show, so a
//     first-ever launch doesn't greet the user with a changelog.
//   - updated but no changelog bundled → skip (nothing to show), don't record.
//   - updated with a changelog → show; the caller records on dismiss.
func whatsNewDecision(current, lastSeen string, hasChangelog bool) (show, record bool) {
	switch {
	case current == lastSeen:
		return false, false
	case lastSeen == "":
		return false, true
	case !hasChangelog:
		return false, false
	default:
		return true, false
	}
}

// maybeShowWhatsNew shows the fullscreen changelog overlay on the first launch
// after a version change, per whatsNewDecision. On a fresh install it silently
// records the version; on a real update it shows the overlay and records the
// version when the user closes it. Called from the composition root (ui.Run),
// the only place that holds the build version, the settings, and the canvas.
//
// version is the same build-time string ui.Run hands the update controller
// (main.version), so "seen this version" here and "already on the latest
// release" there stay in lock-step. Both store/compare the raw string; if the
// updater ever normalizes tags (e.g. trims a "v" prefix) before comparing, this
// compare must match, or a launch could misfire the page.
func maybeShowWhatsNew(c fyne.Canvas, s settings, version string, getenv func(string) string) {
	md := changelogMarkdown()

	// Dev override: force the page every launch (as long as a changelog exists),
	// leaving lastSeenVersion untouched so it keeps showing without a version bump.
	if getenv(whatsNewEnv) != "" {
		if md != "" {
			showWhatsNewOverlay(c, md, func() {})
		}
		return
	}

	show, record := whatsNewDecision(version, s.lastSeenVersion(), md != "")
	if record {
		s.setLastSeenVersion(version)
	}
	if show {
		showWhatsNewOverlay(c, md, func() { s.setLastSeenVersion(version) })
	}
}

// whatsNewTextScale enlarges the changelog body over the app's dense default
// text sizes, so the one-time release notes read comfortably. It applies only to
// the overlay body (via a theme override); the rest of the app keeps its compact
// scale, and the overlay's own title/Close (canvas.Text with explicit sizes)
// are unaffected.
const whatsNewTextScale = 1.3

// whatsNewHeadingSize is the descending point scale for markdown heading levels
// h1..h6 (index = level-1), so the hierarchy reads at a glance — h1 largest,
// each level smaller down to a sub-body h6. Fyne's markdown only distinguishes
// h1/h2 (h3..h6 collapse to one bold style), so these are applied per level
// after re-deriving the level from the source (see standardViewerSegments). The
// base sizes are scaled by whatsNewTextScale like the body, and each level's
// token is resolved back to its size by whatsNewTheme.Size.
var whatsNewHeadingSize = [6]struct {
	name fyne.ThemeSizeName
	base float32
}{
	{"monitor.whatsnew.h1", 26},
	{"monitor.whatsnew.h2", 21},
	{"monitor.whatsnew.h3", 17},
	{"monitor.whatsnew.h4", 14},
	{"monitor.whatsnew.h5", 12},
	{"monitor.whatsnew.h6", 11},
}

// whatsNewTheme is the app theme with the text-role sizes scaled up for the
// changelog body, plus the per-level heading tokens above. Every other token
// defers to the wrapped app theme — the same per-subtree override pattern
// flatFocus uses.
type whatsNewTheme struct{ fyne.Theme }

func (t *whatsNewTheme) Size(name fyne.ThemeSizeName) float32 {
	for _, h := range whatsNewHeadingSize {
		if h.name == name {
			return h.base * whatsNewTextScale
		}
	}
	switch name {
	case theme.SizeNameText, theme.SizeNameHeadingText, theme.SizeNameSubHeadingText:
		return t.Theme.Size(name) * whatsNewTextScale
	}
	return t.Theme.Size(name)
}

// whatsNewReadingWidth caps the changelog to a centered column so long lines
// stay readable, like a standard markdown viewer, instead of stretching the
// full canvas width. On a narrower window the column just fills the space.
const whatsNewReadingWidth = 720

// readingColumnLayout centers its single child and caps its width, letting the
// child's height pass through so it scrolls naturally inside a VScroll. Fyne has
// no built-in max-width-centered layout (NewCenter doesn't cap width), so this
// is the minimum needed to get the reading-column look.
type readingColumnLayout struct{ maxWidth float32 }

func (l readingColumnLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return objs[0].MinSize()
}

func (l readingColumnLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := min(l.maxWidth, size.Width)
	objs[0].Resize(fyne.NewSize(w, size.Height))
	objs[0].Move(fyne.NewPos((size.Width-w)/2, 0))
}

// markdownHeadingLevels returns the heading levels (1..6) in document order, so
// the collapsed h3..h6 levels Fyne drops can be re-applied to the parsed
// segments. Only ATX headings ("# ".."###### ") are recognized; a "#" inside a
// fenced code block would be miscounted, which the controlled changelog avoids.
func markdownHeadingLevels(md string) []int {
	var levels []int
	for line := range strings.SplitSeq(md, "\n") {
		s := strings.TrimLeft(line, " ")
		n := 0
		for n < len(s) && s[n] == '#' {
			n++
		}
		if n >= 1 && n <= len(whatsNewHeadingSize) && n < len(s) && s[n] == ' ' {
			levels = append(levels, n)
		}
	}
	return levels
}

// isHeadingSegment reports whether a parsed text segment is a rendered heading.
// Fyne renders every heading level as a stand-alone (non-inline) bold run with
// text; inline bold (**strong**) stays inline, so this distinguishes the two.
func isHeadingSegment(seg *widget.TextSegment) bool {
	return !seg.Style.Inline && seg.Style.TextStyle.Bold && seg.Text != ""
}

// standardViewerSegments rewrites Fyne's markdown segments toward a conventional
// markdown-viewer look, which the built-in renderer doesn't produce: a per-level
// heading size (h1 largest … h6 smallest, from whatsNewHeadingSize), a
// horizontal rule under h1/h2 (matching how viewers underline top-level
// headings), and left-indented list bullets. levels carries the source heading
// levels in order (see markdownHeadingLevels) because Fyne collapses h3..h6.
// Ordered lists aren't special-cased — the bundled changelog uses only bullets.
func standardViewerSegments(segs []widget.RichTextSegment, levels []int) []widget.RichTextSegment {
	out := make([]widget.RichTextSegment, 0, len(segs))
	headingIdx := 0
	for _, s := range segs {
		switch seg := s.(type) {
		case *widget.TextSegment:
			if isHeadingSegment(seg) && headingIdx < len(levels) {
				level := levels[headingIdx]
				headingIdx++
				seg.Style.SizeName = whatsNewHeadingSize[level-1].name
				out = append(out, seg)
				if level <= 2 {
					out = append(out, &widget.SeparatorSegment{})
				}
				continue
			}
			out = append(out, seg)
		case *widget.ListSegment:
			// Flatten each item into an indented bullet line. RichText only ends a
			// row after a non-inline segment, so each item's inline text is followed
			// by an empty paragraph segment to force the line break (the same device
			// the markdown renderer uses between paragraphs); without it the bullets
			// and the next heading run together on one line.
			for _, item := range seg.Items {
				para, ok := item.(*widget.ParagraphSegment)
				if !ok {
					continue
				}
				out = append(out, &widget.TextSegment{Text: whatsNewBullet, Style: widget.RichTextStyleStrong})
				out = append(out, para.Texts...)
				out = append(out, &widget.TextSegment{Style: widget.RichTextStyleParagraph})
			}
		default:
			out = append(out, s)
		}
	}
	return out
}

// showWhatsNewOverlay adds a full-canvas overlay (not a second window) rendering
// md as Markdown, with a Close link that removes it and runs onClose. The opaque
// background rectangle hides the tabs behind it; Fyne sizes the overlay to the
// whole canvas. The body scrolls vertically when the changelog overflows. The
// changelog's own H1 is the page title, so the header carries only Close.
func showWhatsNewOverlay(c fyne.Canvas, md string, onClose func()) {
	body := widget.NewRichTextFromMarkdown(md)
	body.Segments = standardViewerSegments(body.Segments, markdownHeadingLevels(md))
	body.Wrapping = fyne.TextWrapWord
	// Scope the enlarged text to the body only, then scroll it on overflow.
	scaled := container.NewThemeOverride(body, &whatsNewTheme{newTheme()})
	column := container.New(readingColumnLayout{maxWidth: whatsNewReadingWidth}, scaled)
	scroll := container.NewVScroll(column)

	var overlay fyne.CanvasObject
	dismiss := func() {
		c.Overlays().Remove(overlay)
		onClose()
	}

	head := container.NewHBox(
		layout.NewSpacer(),
		vCenter(newJumpLink(labelWhatsNewClose, dismiss)),
	)
	inner := container.NewBorder(head, nil, nil, nil, scroll)
	padded := container.New(layout.NewCustomPaddedLayout(tabPad, tabPad, tabPad, tabPad), inner)

	overlay = container.NewStack(canvas.NewRectangle(palette.BG), padded)
	c.Overlays().Add(overlay)
}
