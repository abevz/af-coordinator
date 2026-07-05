# Design

## UX Flow
The command `afctl issue create-form` will present a 3-stage form using `github.com/charmbracelet/huh`.

### Screen 1: Context & Basics
- **Project**: Select list (dynamic from `GET /v1/projects`).
- **Scope**: Select list (`project`, `repository`, `worktree`).
- **Repository**: Select list (dynamic from `GET /v1/repos?project=X`), shown if scope is `repository` or `worktree`.
- **Worktree**: Select list (dynamic from `GET /v1/worktrees?repo=Y`), shown if scope is `worktree`.
- **Title**: Text input (required).
- **Priority**: Select list or Integer input (e.g. 1, 2, 3, 4).

### Screen 2: Details
- **Description**: Text area (multiline).
- **Assignee**: Text input (optional). Since assignee requires an update, it will trigger a `PATCH` after creation.

### Screen 3: Dependencies & Links
- **Depends On**: Text input for short IDs (comma-separated, e.g. `afc-4, utils-2`).
- **Artifact Link**: Text input (relative path or UUID).
- **Confirmation**: Confirm boolean (Create / Cancel).

## Implementation Details
1. **Library**: Use `github.com/charmbracelet/huh` for composing the form. It supports groups (screens), validation, dynamic options, and the required keybindings natively.
2. **Execution**:
   - Step 1: Collect all inputs.
   - Step 2: Call `POST /v1/issues`.
   - Step 3: If `Assignee` is set, call `PATCH /v1/issues/{id}`.
   - Step 4: If `Depends On` is set, parse short IDs, resolve them, and call `POST /v1/issues/{id}/dependencies`.
   - Step 5: If `Artifact Link` is set, call `POST /v1/issues/{id}/links`.
3. **Error Handling**: Output standard errors if any API call fails. Rollbacks for partial failures are explicitly omitted for v1 (the issue remains created).
