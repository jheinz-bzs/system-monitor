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

**The whatsnew commit is never pushed to `master`.** `master` is a protected,
PR-only branch — a direct push is rejected (`Changes must be made through a pull
request`). So the release lives entirely on the tag: commit the whatsnew update
on top of `master` locally, tag *that commit*, and push **only the tag**. The
commit rides along with the tag (its objects are uploaded) but `master` is not
updated and no PR is opened. This is how every whatsnew-bearing release has been
cut — e.g. `v0.2.0` (`f40e604`) is the tagged whatsnew commit, sitting on top of
its era's `master` tip but present on no branch. Do **not** open a PR for the
whatsnew change; that's just noise to close later.

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

4. **Commit on top of master (do NOT push to master)** — if whatsnew.md changed:
   ```
   git add internal/ui/whatsnew.md
   git commit -m "docs: update what's new for <version>"
   ```
   The commit stays local; it will be reachable only through the tag. Never
   `git push origin master` (the protected branch rejects it) and never open a
   PR for it. If whatsnew.md didn't change, tag `HEAD` (== `origin/master`) as-is.

5. **Tag that commit and push only the tag**:
   ```
   git tag -a <version> -m "<version>"   # tags HEAD = the whatsnew commit
   git push origin <version>             # pushes the tag ref only, not master
   ```
   Pushing a tag doesn't touch `master`, so it isn't blocked by the branch
   protection. Afterward your local `master` sits one commit (the whatsnew
   commit) ahead of `origin/master` — that's expected; leave it, or reset it to
   `origin/master` if you prefer a clean branch.

6. **Watch** — confirm the workflow started
   (`gh run list --workflow release.yml --limit 1`), then watch it in the
   background (`gh run watch <id> --exit-status`; ~8 min) and report the
   release URL (`https://github.com/<owner>/<repo>/releases/tag/<version>`)
   when it publishes. If the run fails, report the failing job's log tail.
