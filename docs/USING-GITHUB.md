# Using GitHub

## Read this first

**Personal preferences override this document.** Before applying any rule below, check the developer's personal Claude config at `~/.claude/CLAUDE.md` and `~/.claude/projects/<project-id>/memory/`. If a personal preference applies, follow it; on the first relevant operation per session, briefly tell the user.

If neither this document nor the personal config has an answer, ask the user.

## Always

- Prefer the `gh` CLI for GitHub operations. Fall back to a GitHub MCP server only when `gh` can't do the job.
- Authentication is the user's job — never store, paste, or generate tokens.
- Read-only operations (viewing branch / commit / issue / PR / file state) are always free.

---

## Mode: team

This project uses team-wide GitHub workflow rules. Personal preferences still override.

### Branching (when branches are made)

- Feature work happens on a branch off `main`. Use a clear prefix (`feature/`, `fix/`, `chore/`).
- Don't commit directly to `main`.

### Pull requests (when PRs are opened)

- Open a PR when the work is complete, tested, and pushed.
- Title: imperative, under 70 chars.
- Body: short summary + Test Plan.

### Autonomy

- **BizStream mode**: Read-only by default.
  - Claude doesn't create branches, commits, pushes, PRs, issues, or resolve threads.
  - If on `main`, `master`, or `develop`, surface that and offer to create a feature/fix branch — wait for explicit yes.
  - No `git worktree` usage. *(Subject to change.)*
  - Claude may recommend writes at logical breakpoints.
  - **Explicit user approval overrides any rule above, per operation.**

### Never

- Force-push to shared branches.
- Skip pre-commit hooks unless the user explicitly asks.
- Delete branches, close PRs, or dismiss reviews without explicit instruction.
- Modify the git config.
- Amend commits that have been pushed.
