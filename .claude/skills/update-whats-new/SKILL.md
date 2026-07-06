---
name: update-whats-new
description: >-
  Rewrite the bundled What's New changelog (internal/ui/whatsnew.md) to describe the
  current release from the branch's diff. Use when the user says "update the what's new",
  "write the changelog for this release/branch/version", "refresh whatsnew.md", or is
  prepping a branch for release and wants release notes. Reads the diff against master and
  writes user-facing notes only.
allowed-tools: Read, Write, Edit, Bash
---

# Update What's New

Rewrite `internal/ui/whatsnew.md` so it describes the release currently on this
branch, derived from the diff against `master`.

## What this changelog is

`whatsnew.md` is bundled into the binary (via `tools/genassets`) and shown **once**
in a fullscreen overlay on the first launch after the app version changes
(`internal/ui/whatsnew.go`). It is not a scrolling history — each release **replaces**
the content. Describe *this* release only; do not accumulate old entries.

## Steps

1. Get the release's changes:
   ```bash
   git log master..HEAD --oneline
   git diff master..HEAD --stat
   ```
   For anything unclear, read the specific file diff (`git diff master..HEAD -- <file>`).
   Focus on what a **user** sees — a new tab/panel, a new setting, a visible behavior
   change. Ignore refactors, tests, and internal-only changes.

2. Rewrite `internal/ui/whatsnew.md`, matching the existing structure:
   - `# What's New` → `## System Monitor` → one-line greeting.
   - One `###` section per user-facing feature area, short bullets under each.
   - Bold the UI location a user would click (e.g. **Settings**).
   - Tone: plain, present-tense, no marketing. Mirror the sentences already in the file.

3. Do **not** add a "This page" / meta section unless it's genuinely new behavior.

## Don't

- Don't list refactors, test changes, dep bumps, or file-level detail.
- Don't keep prior releases' notes — replace, don't append.
- Don't bump the app version here; the version compare is what triggers the overlay,
  and that lives elsewhere (`internal/update` → `ui.Run`).
