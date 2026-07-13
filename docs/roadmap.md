# Roadmap

Direction for af-coordinator beyond v1. The operational source of truth for
this work is the coordinator itself (project `afc`); this document records
the intent and the reasoning so the issues stay short.

## Planned target: execution telemetry and analytics hardening (`afc-47`)

Packet `docs/specs/008-execution-telemetry-and-analytics/` captures gaps found
from the first ten days of live multi-project use. It first makes causal event
order and close authorization trustworthy, then adds lease-attempt outcomes,
atomic HANDOFF, and a local project statistics report.

`afc-49` delivered causal event order, `afc-50` delivered close/reopen
authorization, and `afc-51` delivered lease-attempt outcomes. The remaining
implementation issues (`afc-52` and `afc-53`) remain deliberately deferred
while the operator compares this track with Aion Forge Harness v2 Phase 4.

Delivery order is:

```text
event sequence + close authorization
    -> lease attempts + atomic HANDOFF
    -> project statistics
```

The coordinator remains the issue/lease/audit control plane. It does not absorb
Temporal or Aion Forge workflow execution state, and the first report does not
add Prometheus, remote analytics, or a mutable rollup database.

## Completed target: public readiness (`afc-39`-`afc-44`)

Packet `docs/specs/006-public-readiness/` completed the public README refresh,
API endpoint map, safe worktree cleanup commands, install preflight, macOS
daemon install path, Linux service helper, and API-facing store boundary
cleanup. SQLite remains the only storage backend; the boundary keeps transport
code separate from persistence details without adding multi-database support.

## Completed target: release and backup readiness (`afc-45`-`afc-46`)

Packet `docs/specs/007-release-and-backup-readiness/` added tagged GitHub
release packaging, checksum manifests, release install docs, tag-driven version
injection, and macOS launchd backup parity with `afctl doctor` coverage.

## Completed target: Aion Forge integration (epic `afc-24`)

af-coordinator now has the local tracker / control-surface primitives needed
by the [Aion Forge](https://github.com/abevz/aion-forge) agent factory:

```text
issue (af-coordinator) -> Temporal workflow -> isolated runner -> branch/PR -> checks -> merge -> issue closed
```

Division of responsibility stays strict:

- af-coordinator is the single write authority over issue state
  (status, leases, notes, audit trail)
- Temporal owns execution truth (retries, workflow progress, runner state)
- the coordinator stores references to execution (workflow IDs, PR URLs),
  never execution state itself

Delivered work:

| Issue | Type | What |
|---|---|---|
| `afc-30` | bug | Dependency response identity semantics: stop mixing UUID and short_id in dependency payloads; return explicit fields and cover the contract with regression tests |
| `afc-36` | bug | Scope repository resolution by project: remove ambiguous unscoped logical-name resolution before broader multi-project use |
| `afc-25` | feature | Events watch API: `GET /v1/events?since=` with long-poll, so consumers react to new ready issues without tight polling |
| `afc-26` | feature | External issue references on issues: start with mirrored issue keys / workflow IDs while keeping coordinator issue status authoritative |
| `afc-27` | feature | Structured resolution: PR/commit references on close |
| `afc-29` | feature | JSONL export (`internal/export`), backup + interim bridge |
| `afc-28` | feature | MCP server wrapper over the daemon API for Claude-based agents |

Packet `docs/specs/005-aion-forge-integration-readiness/` is complete. The
coordinator has no active roadmap target until the next direction is written
as a new SDD packet and corresponding `afc` issues.

The unclaimable-epic closure gap is resolved by explicit local operator
close/reopen commands with a reason and audit event; ordinary agent close
continues to require its active lease token.

## Issue classification (done)

`issue_type` shipped in migration `0002`: `task` (default), `bug`,
`feature`, `epic`, `chore`.

Design decisions, recorded here so they are not re-litigated:

- a single type column instead of free-form labels; labels wait until a
  concrete need appears
- `epic` is a container, not a unit of work: it cannot be claimed and is
  excluded from the ready view; children attach via the existing `parent`
  dependency kind
- type is the routing key for future agent pipelines
  (bug -> bugfix flow, feature -> SDD flow, chore -> lightweight flow)

## Non-goals (unchanged from v1)

- web UI
- distributed cluster mode / multi-node replication
- GitHub as source of truth
- embedded scripting

## Working agreement

New roadmap items start as issues in project `afc` (`afctl issue create
--project afc --type ...`). This file is updated only when direction
changes, not per issue.
