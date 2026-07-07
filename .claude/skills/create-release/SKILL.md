---
name: create-release
description: >-
  Cut a new release end-to-end: confirm the version number, refresh the bundled
  What's New changelog (internal/ui/whatsnew.md) from the diff since the last tag,
  commit it, then tag vX.Y.Z and push to trigger the GitHub release workflow. Use
  when the user says "make a release", "cut a release", "release vX.Y.Z", "ship
  v0.2.0", or "new version". Pass the version as an argument to skip the prompt.
allowed-tools: Read, Write, Edit, PowerShell, Bash, AskUserQuestion
---

# Create Release

Releases are fully automated by `.github/workflows/release.yml`: pushing a
`v*.*.*` tag builds every platform and publishes the GitHub Release. This skill
just gets `whatsnew.md` right first, then pushes the tag.

## Steps

1. **Preconditions** — abort with a clear message if any fail:
   - On `master`, clean working tree, and `git fetch origin` shows HEAD ==
     `origin/master`.
   - Determine the last release: `git tag --sort=-v:refname` (first entry).

2. **Version** — if not given as an argument, ask the user with
   AskUserQuestion, recommending the next patch bump of the last tag.
   Must match `vX.Y.Z` and not already exist as a tag.

3. **Update What's New** — follow
   [update-whats-new](../update-whats-new/SKILL.md) **but diff against the last
   tag, not master** (releases are cut from master, so `master..HEAD` is empty):
   ```
   git log <lastTag>..HEAD --oneline
   git diff <lastTag>..HEAD --stat
   ```
   Rewrite `internal/ui/whatsnew.md` per that skill's structure and tone rules
   (replace, don't append; user-facing changes only). If the file already
   describes exactly this diff (e.g. it was updated on the feature branch),
   skip the rewrite and say so.

4. **Commit** — if whatsnew.md changed:
   `git add internal/ui/whatsnew.md`, commit
   (`docs: update what's new for <version>`), push to master.

5. **Tag and push**:
   ```
   git tag -a <version> -m "<version>"
   git push origin <version>
   ```

6. **Watch** — confirm the workflow started
   (`gh run list --workflow release.yml --limit 1`), then watch it in the
   background (`gh run watch <id> --exit-status`; ~8 min) and report the
   release URL (`https://github.com/<owner>/<repo>/releases/tag/<version>`)
   when it publishes. If the run fails, report the failing job's log tail.
