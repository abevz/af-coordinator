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
| `afc-25` | feature | P2 | open | - | Events watch API | Define `GET /v1/events?since=` cursor semantics, bounded long-poll behavior, and event payload contract tests. |
| `afc-26` | feature | P2 | open | - | External issue references | Store mirrored issue keys and Temporal workflow IDs as references while coordinator issue status stays authoritative. |
| `afc-27` | feature | P3 | open | - | Structured close resolution | Support PR URL, commit SHA, and external execution reference metadata in close output and events. |
| `afc-29` | feature | P3 | open | - | JSONL export | Export issues, notes, events, dependencies, artifacts, and references through read-only daemon/store-backed code. |
| `afc-28` | feature | P3 | open | - | MCP wrapper over daemon API | Expose stable coordinator operations through MCP without direct SQLite access. |
