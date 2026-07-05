# Review

Status: completed

## AFC-SDD-0059, AFC-SDD-0060, AFC-SDD-0061 — Interactive TUI for issue creation

### What shipped

- **CLI**: Added `afctl issue create-form` subcommand using `charmbracelet/huh`.
- **Flow**: Implements a 3-stage form:
  - Screen 1: Project, Scope, Repository (dynamic), Worktree (dynamic), Title, Priority.
  - Screen 2: Description, Assignee.
  - Screen 3: Depends On, Artifact Link.
- **Integration**: The command invokes `c.CreateIssue` followed by conditional updates for `Assignee` (`c.UpdateIssue`), `Depends On` (`c.AddDependency`), and `Artifact Link` (`c.LinkArtifact`).

### What was verified

- Form correctly fetches projects via API before presenting selection.
- Builds successfully and dependencies cleanly resolved.
