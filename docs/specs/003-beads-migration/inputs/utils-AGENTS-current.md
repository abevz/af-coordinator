# Utils Repository - AI Agent Guide

This repository is a multi-tool workspace. Use the repo-root Beads workspace for
persistent task tracking.

## Beads Workflow

- Use `bd` for work that may span sessions, involve dependencies, or need
  recovery after compaction.
- Before starting substantial work, run `bd ready --json` and pick an
  unblocked task.
- For a specific task, run `bd show <id> --long` before making changes.
- When starting work, claim it with `bd update <id> --claim --json`.
- Record meaningful progress in Beads, not only in chat history.
- Use notes for decisions, partial progress, blockers, and next steps.
- If new work is discovered, create a linked issue with
  `bd create ... --deps discovered-from:<current-id>`.
- Respect dependency direction: if A depends on B, B must finish before A.
- On completion, close with `bd close <id> --reason "<what was done>" --json`.
- If work is blocked, update the task status and add a note describing the
  blocker.
- Prefer `bd` over ephemeral todo lists for multi-step or multi-day work.
- Use session-local checklists only for short-lived execution steps.
- This repo is one Beads workspace at the repo root; do not initialize separate
  `.beads/` directories in subtools.
- Distinguish tool-specific work with labels rather than separate Beads
  workspaces.
- For new worktrees, prefer `bd worktree create`; if a worktree already exists,
  ensure it points at the shared repo-root `.beads/`.
- Treat `bd` as the source of truth for task state.

## Workspace Model

- Canonical model: one git repo = one Beads workspace = one Beads database.
- For this repo, the canonical workspace is the repo-root `.beads/`.
- Keep subtools inside the same workspace; use labels instead of per-subtool
  `.beads/` directories.
- New worktrees should share that same workspace; prefer `bd worktree create`.
- Do NOT run `bd init` separately inside subdirectories or each worktree.
- If a worktree was created with plain `git worktree add`, ensure its
  `.beads/redirect` points at the canonical repo workspace before using `bd`.

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
