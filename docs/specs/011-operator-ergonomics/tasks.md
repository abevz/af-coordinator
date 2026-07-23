# 011 Operator Ergonomics — Tasks

Coordinator IDs are canonical; `afctl` owns live status.

## Leaves

| ID | Type | Pri | Blocked by | Title |
| --- | --- | ---: | --- | --- |
| `afc-63` | feature | P1 | — | Operator-close: metadata fields + optional auto-version |
| `afc-64` | feature | P2 | — | Auto-resolve `--expected-version` for operator commands |
| `afc-65` | feature | P3 | — | Status flap guard: cooldown + `--reason` field |
| `afc-66` | feature | P3 | afc-63 | Bulk operator mutations: multi-ID close/cancel/update |

## afc-63 — Operator-close: metadata fields + optional auto-version

**Status**: open · **Priority**: P1

### Scope

Enrich the existing `afctl issue operator-close` command with:
- `--branch`, `--pr-url`, `--commit-sha`, `--note` metadata (matching `afctl issue close` surface)
- Optional `--expected-version` (auto-resolved from current issue version when omitted)

### Implementation

1. `internal/core/issue.go`: add `Branch`, `PRURL`, `CommitSHA`, `Note` fields to `OperatorCloseIssueRequest` (L227-234).
2. `internal/store/sqlite/issues.go` `OperatorCloseIssue()` (L992-1036): pass metadata into `CloseIssueResult` via `updateTerminalIssue`. Insert note if `req.Note` is non-empty.
3. `internal/api/issues.go` `handleOperatorCloseIssue()` (L468-490): populate result metadata from request.
4. `cmd/afctl/cmd_issue.go` `runIssueOperatorClose()` (L683-736): add flag parsing for `--branch`, `--pr-url`, `--commit-sha`, `--note`. When `ExpectedVersion == -1`, auto-fetch via `client.GetIssue()`.
5. Tests: operator-close with metadata, close without `--expected-version` (auto-resolve), version conflict.

### Acceptance

`afctl issue operator-close <id> --resolution done --reason "merged" --pr-url <url> --commit-sha <sha> --note <text>` closes in one command. `--expected-version` is optional. Metadata recorded in `CloseIssueResult` and audit event. `go test ./... -count=1` passes.

---

## afc-64 — Auto-resolve `--expected-version` for operator commands

**Status**: open · **Priority**: P2

### Scope

Make `--expected-version` optional for all interactive operator mutations (`update`, `close`, `operator-close`, `operator-reopen`). When omitted, the CLI auto-fetches the current version.

### Implementation

1. `cmd/afctl/cmd_issue.go`: in `runIssueUpdate` (L510+), `runIssueClose` (L594+), `runIssueOperatorClose` (L683+), `runIssueOperatorReopen` (L738+) — when `ExpectedVersion == -1`, call `client.GetIssue()` and use its `Version`.
2. Accept `--expected-version latest` as an alias.
3. Accept `--force` as a shorthand for `--expected-version latest`.
4. No API or store changes — resolution is CLI-only. API still requires a concrete version.

### Acceptance

`--expected-version` is optional for operator commands. `--force` accepted as alias. Concurrent modification still produces `version_conflict`. `go test ./... -count=1` passes.

---

## afc-65 — Status flap guard: cooldown + `--reason` field

**Status**: open · **Priority**: P3

### Scope

Three incremental improvements (implement in order):

**A. `--reason` field** — add an optional reason to status updates, recorded in the audit event.
**B. Cooldown** — reject rapid status reversals within a configurable window (default 60s).
**C. Operator lock** (optional) — `--operator-lock` prevents non-operator actors from changing status.

### Implementation

**Part A:**
1. `internal/core/issue.go` `UpdateIssueRequest`: add `Reason string` field.
2. `internal/store/sqlite/issues.go` `UpdateIssue()`: include reason in `issue_updated` event payload.
3. `cmd/afctl/cmd_issue.go` `runIssueUpdate()`: add `--reason` flag.
4. `internal/api/issues.go`: pass `Reason` from JSON.

**Part B:**
1. `internal/store/sqlite/issues.go` `UpdateIssue()`: query latest `issue_updated` event with status change. Reject if < cooldown seconds ago AND is a reversal.
2. Operator-close/reopen bypass cooldown.

**Part C (optional):**
1. Add `operator_locked` boolean column (migration).
2. When locked, only `operator-*` commands change status.

### Acceptance

Part A: `--reason` recorded in audit event. Part B: rapid reversal within 60s rejected. `go test ./... -count=1` passes.

---

## afc-66 — Bulk operator mutations: multi-ID close/cancel/update

**Status**: open · **Priority**: P3 · **Blocked by**: afc-63

### Scope

Accept multiple positional issue IDs in operator mutation commands. Each issue processed independently; partial failures reported without aborting.

### Implementation

1. `cmd/afctl/cmd_issue.go` `runIssueOperatorClose()`: collect args until first `--flag` as issue IDs. Loop over each, calling `client.OperatorCloseIssue()`.
2. Error handling: collect per-issue results. Print `afc-42: closed` or `afc-42: error: <reason>`. Exit 1 if any fail.
3. `--json` mode: output JSON array of per-issue results.
4. Same pattern for `runIssueOperatorReopen` and optionally `runIssueUpdate`.

### Acceptance

`afctl issue operator-close <id1> <id2> --resolution cancelled --reason "batch"` closes both. Per-issue results reported. Partial failures don't abort. `go test ./... -count=1` passes.
