# Tasks

Task IDs are the live af-coordinator short IDs in project `afc`. One ID scheme,
not two. Use `afctl issue get afc-<N>` for current status, claims, dependencies,
notes, and closure audit.

## Parent epic

**`afc-24`** (epic, P2, open) - Aion Forge integration readiness

## Leaves

| ID | Type | Pri | Status | Blocked by | Title | Scope |
|----|------|-----|--------|------------|-------|-------|
| `afc-35` | chore | P2 | done | - | Refresh SDD active packet metadata | Updated packet 003 status, moved active packet guidance to 005, and created this packet. |
| `afc-37` | chore | P2 | done | - | Align 005 SDD task IDs with afc issue IDs | Replace the previous parallel spec-task names with the live `afc-N` IDs. |
| `afc-30` | bug | P1 | done | - | Dependency response identity semantics | Define explicit UUID and short id response fields; add API and CLI regression tests. |
| `afc-36` | bug | P1 | done | - | Scope repository resolution by project | Remove ambiguous unscoped logical-name resolution from multi-project paths; keep exact UUID lookup. |
| `afc-25` | feature | P2 | done | - | Events watch API | Added global `GET /v1/events` with opaque `since` cursor, `limit`, bounded `wait_ms` long-poll, and contract tests for pagination and payload JSON validity. |
| `afc-26` | feature | P2 | done | - | External issue references | Added optional `external_key` on issues with exact-match query filtering so mirrored issue keys and Temporal workflow IDs can be stored as references while coordinator status stays authoritative. |
| `afc-27` | feature | P3 | done | - | Structured close resolution | Added structured close metadata (`branch`, `pr_url`, `commit_sha`) in close request/response and `issue_closed` events, and included issue `external_key` there when present. |
| `afc-29` | feature | P3 | done | - | JSONL export | Added `internal/export` plus `GET /v1/export/jsonl` and `afctl export jsonl` for normalized read-only export of projects, repos, remotes, worktrees, artifact roots, artifacts, issues, dependencies, references, notes, and events. |
| `afc-28` | feature | P3 | done | - | MCP wrapper over daemon API | Added `afc-mcp` stdio server exposing health, get/ready, claim, heartbeat, note, events, and close tools backed only by the daemon API client. |
