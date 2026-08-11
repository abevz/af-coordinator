# af-coordinator Coordination Safety Audit

Audit date: 2026-08-11

Audited revision: `68f4bc67e6ca7067d13bc1c75c7ec3f4204df613` (`main`)

Scope: repository code, migrations, tests, SDD/docs, git history, installed
daemon health, runtime file modes, and live project `afc` backlog.

## 1. Executive Summary

**Verdict: do not yet entrust `af-coordinator` with unsupervised coordination
of multiple concurrently working AI agents.** It is already a useful local
execution ledger and has several strong transactional paths, but its safety
contract is inconsistent across mutations.

The highest-risk defect is not SQLite. It is that some ownership/version checks
are separated from the writes they authorize:

- `UpdateIssue` reads version and lease before its transaction, then updates by
  issue ID only (`internal/store/sqlite/issues.go:803-975`);
- heartbeat reads token/expiry and later updates without expiry in the write
  predicate or affected-row validation (`565-601`);
- release does not require an unexpired lease (`604-660`);
- direct claim does not enforce the dependency-ready predicate and holder-only
  reattach returns the existing secret token (`406-562`).

In contrast, fresh claim, handoff, and close already use transactions and have
valuable constraints/tests. The event sequence, attempt IDs, backup automation,
operator recovery paths, and Unix-socket API are solid foundations. The
repository-wide race-enabled test command passed during the audit, but the main
store/API fixtures force one SQLite connection, so that green result does not
exercise the production concurrency model.

The minimum path to concurrent-agent trust is:

1. one lease identity and atomic fencing on every lease-bound mutation;
2. a provable single-daemon/serialized-SQLite boundary plus atomic dependency
   and ready-qualified claim;
3. durable operation IDs for ambiguous retries and black-box crash/restart
   proof;
4. lease-loss agent behavior and minimum operational signals.

SQLite should remain. There is no evidence that Dolt, PostgreSQL, or consensus
replication is required for the stated local-first, one-daemon topology.

## 2. Current Architecture

### Authoritative state

The canonical execution state is the SQLite database selected by
`AF_COORDINATOR_DB`, defaulting to
`~/.local/share/af-coordinator/af-coordinator.db`
(`internal/config/config.go:10-25`). `cmd/af-coordinatord/main.go:25-48` opens
that database, runs embedded migrations, constructs the SQLite store, and then
serves the API.

Durable state in SQLite includes projects, repositories, worktrees, artifacts,
issues, dependencies, leases, notes, events, and tags
(`migrations/0001_schema_v1.sql`, `0002`-`0007`). The Unix socket, process
identity, in-flight HTTP requests, CLI stdout, and worker environment are not
authoritative and disappear on restart. SDD files are authoritative for scope
and design, not execution status.

The daemon can reconstruct current issue, dependency, event, and lease state
after restart because expirations and attempts are stored in the DB. It does
not need an in-memory lease table. An expired lease is handled lazily: the ready
query ignores it and the next claim records `lease_expired` before replacement
(`issues.go:319-340`, `488-512`).

There is no supported CLI/helper mutation that bypasses the daemon; `afctl`
uses `internal/client` over the Unix socket, and repository instructions forbid
direct DB mutations. This is convention plus filesystem boundary, not a hard
same-UID security boundary. Runtime inspection found parent directories mode
`0755`, DB/WAL/SHM `0644`, and socket `0660`; a process with the operator's UID
can open the DB directly. That must be documented or isolated at the OS level
for untrusted workers.

### Write ownership and serialization

The intended model is explicit in `docs/architecture-v1.md:9-20,78-102`:

```text
agents / afctl -> one daemon -> store boundary -> SQLite
```

HTTP handlers may run concurrently. SQLite supplies the actual single-writer
serialization, while store functions choose their own transaction boundaries.
The implementation does not yet establish the documented dedicated write
connection. `sqlite.Open` executes WAL, foreign-key, and busy-timeout PRAGMAs
through `*sql.DB` but neither configures the pool nor sets
`synchronous=NORMAL` (`internal/store/sqlite/sqlite.go:15-34`). Foreign keys
and busy timeout are connection-local, so executing them on an arbitrary pooled
connection is insufficient evidence for all later connections.

Daemon singleton enforcement is also incomplete. `removeStaleSocket` unlinks
any socket path without checking whether a live daemon owns it
(`internal/api/daemon.go:154-173`). A second process can orphan the first
listener and leave two processes connected to the same DB. SQLite still
serializes file writes, but the architectural ownership claim is false and
clients split across processes/revisions.

## 3. Task State Machine

The real persisted statuses are defined by the database CHECK in
`migrations/0001_schema_v1.sql:77-96`:

```text
open, in_progress, blocked, deferred, done, cancelled
```

The observed service transitions are:

| Source | Operation | Target/effect | Authorization |
| --- | --- | --- | --- |
| `open` or eligible `in_progress` | fresh claim | `in_progress` + lease | no active lease; epic rejected |
| active leased issue | heartbeat | same status; expiry extended | matching, currently unexpired token intended |
| `in_progress`/leased | release | `open`, or remains `blocked`; lease removed | matching token, currently expiry not enforced |
| leased issue | handoff | required note + release atomically | active token and unexpired lease |
| nonterminal | update | another nonterminal status/metadata | expected version; token if active lease |
| nonterminal | close | `done` or `cancelled`; lease removed | active token, unexpired lease, expected version |
| nonterminal/epic | operator-close | `done` or `cancelled` | local operator token path, reason/version |
| `done`/`cancelled` | operator-reopen | `open` | local operator token path, reason/version |
| stuck `in_progress` | operator-release | `open`; lease removed | local operator token path, reason/version |

`ready`, `claimed`, `expired`, and `handed_off` are derived conditions/events,
not statuses. Ready is computed from status, type, active lease, and unfinished
`blocks` dependencies (`issues.go:319-356`). Because `in_progress` is not
excluded, an `in_progress` issue with only an expired lease becomes ready even
before lazy cleanup.

`core.ValidateStatusTransition` allows broad nonterminal and terminal edges
(`internal/core/issue.go:464-483`), but generic update separately forbids close
or reopen (`issues.go:821-835`). Database constraints reject unknown enum
values but do not encode the transition graph, ownership, version, or
dependency readiness. Therefore invalid histories are prevented primarily by
service code; direct DB access can create states the API would not generate.

## 4. What Is Already Working

- **Fresh claim has one DB winner.** Claim wraps lease inspection, expired
  attempt replacement, lease insert, issue update, and event in one transaction
  (`issues.go:412-562`). `leases.issue_id` is a primary key and
  `lease_token` is unique (`0001_schema_v1.sql:115-122`). A conflicting insert
  maps to `lease_held`.
- **Ready is computed, not cached.** It excludes epics, active leases,
  blocked/deferred/terminal statuses, and issues with unfinished `blocks`
  dependencies (`issues.go:319-356`). Closing a blocker needs no ready-state
  recalculation job.
- **Handoff is atomic.** Active lease validation, required `HANDOFF:` note,
  note event, lease delete, issue transition, and release event share one
  transaction (`issues.go:663-748`). Rollback/failure tests exist at
  `internal/store/sqlite/issues_test.go:2384+` and `2485+`.
- **Agent close is strongly grouped.** Version/status, active lease, optional
  note, terminal issue update, lease removal, and close event are one
  transaction (`issues.go:1010-1096`). Expired/wrong-token tests exist at
  `issues_test.go:1673+`.
- **Operator recovery paths are separate and audited.** Operator close,
  reopen, and release do not masquerade as ordinary lease ownership
  (`issues.go:1099-1288`; API docs `docs/api-v1.md:255-269`).
- **Audit order is durable.** Migration `0005_event_sequence.sql` gives events
  a daemon-assigned sequence; migration `0006_lease_attempts.sql` adds attempt
  and session correlation. Tokens are intentionally absent from events.
- **Migrations are transactional and ordered.** Each embedded migration is
  applied and recorded in one transaction
  (`internal/store/sqlite/sqlite.go:37-87`).
- **Backup has real operational support.** The packaged job uses SQLite
  `VACUUM INTO`, runs `PRAGMA integrity_check` on the backup, and retains 14
  copies (`contrib/systemd/af-coordinator-backup.sh`; `docs/operations.md:122-199`).
- **Protocol has a usable base.** HTTP provides framing over the Unix socket;
  `/v1/` paths, JSON error codes, CLI exit-code mapping, five-second client
  timeout, graceful HTTP shutdown, and an installed revision health field are
  implemented (`internal/client/client.go:21-53,606-647`,
  `internal/api/daemon.go:47-150`).
- **Tests cover many single-operation invariants.** Claim, reattach, expiry,
  close, handoff rollback, ready filtering, dependency cycles, tags, API error
  envelopes, backups, and operator paths have focused tests.

These strengths make hardening incremental. They do not yet compose into the
required concurrent/crash-safe guarantee.

## 5. Concurrency Safety Analysis

### Claim semantics

For two distinct holders in one daemon, the lease primary key and transaction
produce one winner. The issue status/version and claim event commit with the
lease. However:

- claim checks only status/type, not the ready dependency predicate;
- it accepts `in_progress` to support reclaim after expiry;
- same-holder reattach uses caller-provided holder equality as proof and
  returns the existing secret token;
- there is no monotonic lease generation for external fencing;
- claim retry after a lost response is not correlated by operation ID.

Thus exclusive DB ownership is stronger than the public identity/retry model.

### Lease and heartbeat

Expiration is persisted as daemon-generated RFC 3339 UTC text and evaluated
with daemon wall clock. Client clock skew is irrelevant. Backward host-clock
movement delays reclaim; forward movement may expire early. With correct
fencing those are liveness/availability effects, not dual-ownership effects.

There is no background reaper. Ready treats expired rows as inactive; fresh
claim lazily logs and replaces them. That approach is sufficient for
correctness if every mutation uses the same expiry predicate.

Heartbeat currently does not. It selects the row, parses expiry, then issues a
separate update. If reclaim/handoff deletes or replaces the row between those
steps, `Exec` can affect zero rows and heartbeat still returns success. This is
an agent-safety failure even when the DB ends with only one owner.

### Handoff and close

Handoff and ordinary close require an active unexpired token inside their
transactions and are materially stronger than update/heartbeat. A stale close
after a later claim observes the later token and fails. Double close with a new
request fails because the issue is terminal. That prevents a second state
transition but is not idempotent: after a response timeout, the caller cannot
distinguish its committed close from someone else's terminal state.

### Mandatory race results

| Race | Result | Evidence |
| --- | --- | --- |
| 1. Two workers claim one ready task | **SAFE** within one DB | Claim transaction plus `leases(issue_id)` PK; conflicting insert maps to `lease_held` (`issues.go:412-562`, migration 0001). No production multi-connection proof yet. |
| 2. Old heartbeat vs expiry/reclaim | **UNSAFE** | Heartbeat ownership/expiry read and update are separate; update omits expiry and does not verify rows (`565-601`). |
| 3. Old owner close after new claim | **SAFE** for coordinator state | Close transaction selects current unexpired lease and compares token before terminal update (`1040-1065`); old token cannot match replacement. |
| 4. Handoff vs heartbeat | **UNSAFE** at protocol level | Handoff atomically deletes lease, but an already-read heartbeat can update zero rows and still report renewed. No dual DB owner, but false ownership success. |
| 5. Daemon dies between ownership check and save | **UNKNOWN** as an operational guarantee | SQLite statements/transactions are atomic, but update validates outside its transaction and there is no process-kill failpoint matrix proving every path/commit boundary. |
| 6. Client times out after commit and retries | **UNSAFE** | No operation/request ID or stored result exists; create duplicates, heartbeat changes expiry again, terminal retry reports current-state failure. |

`SAFE` here is narrow: it means the inspected coordinator-state race has one
valid owner. It does not claim exactly-once external Git/filesystem work.

## 6. Crash/Recovery Analysis

| Failure | Current behavior | Assessment |
| --- | --- | --- |
| 1. Worker dies immediately after claim | Lease/attempt remain durable; issue is hidden until expiry, then appears ready and next claim records expiry. | Ownership safe; bounded liveness delay. |
| 2. Worker dies after doing work but before close | Coordinator remains nonterminal. After expiry another worker may repeat external work; no durable operation/result proves whether publish completed. | **UNSAFE** for duplicate-work avoidance. |
| 3. Daemon dies during claim | SQLite transaction is either committed as lease+issue+event or rolled back. Client may not know which. | Atomic state; **UNSAFE** blind retry. |
| 4. Daemon dies during heartbeat | Single UPDATE is atomic, so old or new expiry persists, but split check/update and lost response make the client's belief ambiguous. | **UNKNOWN/UNSAFE** until CAS + operation ID. |
| 5. Daemon dies during handoff | Note, events, issue transition, and lease removal share one transaction: all or none. A committed lost response cannot be replayed as success. | Atomic state; ambiguous retry. |
| 6. Restart with active leases | Lease rows and absolute expirations reload from SQLite; no in-memory reconstruction needed. | **SAFE** under same host clock and DB. |
| 7. Host reboot | WAL recovery is expected from SQLite; service restarts on failure. `synchronous` is not explicitly configured/verified and no power-loss-equivalent test exists. | **UNKNOWN** durability claim. |
| 8. Client retries after timeout | No general deduplication. Create can duplicate; claim behavior depends on holder/current state; heartbeat extends again; close/handoff may return terminal/expired. | **UNSAFE** as a protocol. |

The existing backup/doctor path reduces recovery risk but does not replace a
black-box crash matrix. Database corruption is detected for backup files by
doctor; daemon startup itself performs no explicit integrity policy beyond
open/migrate/ping.

## 7. Incomplete or Fragile Areas

### Correctness and durability

- no common atomic lease predicate or monotonic generation;
- update version/ownership TOCTOU;
- heartbeat false success and release-after-expiry;
- direct claim bypass of dependencies and unsafe holder-only token recovery;
- dependency cycle check outside write transaction with swallowed traversal
  errors;
- no operation IDs or outcome reconciliation;
- no daemon singleton lock and no verified per-connection PRAGMAs;
- no failure-injection/crash/restart matrix;
- no startup integrity/known-schema policy.

### Protocol/API

HTTP correctly frames partial socket I/O, but the daemon sets only
`ReadHeaderTimeout`; write/idle/body limits are not a uniform contract
(`internal/api/daemon.go:120-123`). Some handlers call
`DisallowUnknownFields`, but malformed/trailing/oversized request behavior is
not centralized. `/v1/` provides path versioning, yet additive/breaking lease
contract compatibility and retry semantics are undocumented. Local socket
permissions authorize ordinary calls; the operator bearer token gates only
operator operations.

The five-second client timeout is appropriate for responsiveness but creates
ambiguous commit outcomes without idempotency. Error codes tell agents about
version/lease/dependency conflicts, not whether a timed-out mutation committed.

### Agent UX

`issue ready`, structured issue/acceptance/dependency fields, claim output,
heartbeat cadence, handoff format, exit codes, and `issue run` are good agent
surfaces. Ambiguities remain:

- claim can accept work that `ready` says is blocked;
- holder appears stronger than attribution because it can recover a token;
- agents receive no generation for external fencing;
- heartbeat success is not authoritative under races;
- `issue run` logs heartbeat loss and lets work continue
  (`cmd/afctl/cmd_issue_run.go:127-146`);
- no decision tree exists for timeout, daemon restart, or uncertain remaining
  lease time.

### Auditability and observability

The append-only event sequence is worth keeping; event sourcing is unnecessary.
Current history can reconstruct claim, attempt, release/handoff, close, and lazy
expiry. Heartbeat renewals leave no durable count/summary, stale mutation
rejections are not first-class operational evidence, and artifact link creation
does not use the same event coverage as core lifecycle paths. Health reports DB
ping and revision, while existing stats report workflow outcomes; neither gives
active leases, claim conflicts, DB busy/errors, or mutation latency.

## 8. Backlog Audit

This classification covers every nonterminal `afc` issue present when the audit
started (`afc-70`, `afc-89`, `afc-90`, `afc-97`, `afc-98`, `afc-99`) plus the
stale packet-011 lineage that made active-packet selection incorrect. Completed
tasks in packets explicitly reviewed complete are historical evidence, not
backlog, and are not re-triaged one by one.

| Task | Category | Evidence and decision |
| --- | --- | --- |
| `afc-63` | **DUPLICATE** | Cancelled live; replaced by `afc-67`. Packet 011 still named it active. |
| `afc-64` | **DUPLICATE** | Cancelled live; replaced/rescoped as `afc-68`. |
| `afc-65` | **DUPLICATE** | Cancelled live; replacement `afc-69` was later cancelled as stale. |
| `afc-66` | **DUPLICATE** | Cancelled live; replacement is open `afc-70`. |
| `afc-67` | **DONE IN PRACTICE** | Live status `done`; implementation commit `92812f2`, merged by `71a9fd0`; operator-close metadata tests exist at `issues_test.go:1866+`. |
| `afc-68` | **DONE IN PRACTICE** | Live status `done`; implementation commit `892a59c`, merged in `68f4bc6`; tagged `exec/auto`. |
| `afc-69` | **STALE** | Live status `cancelled`; the broad cooldown/operator-lock proposal no longer has an accepted active design. |
| `afc-70` | **REWRITE** | Still useful P3 ergonomics, but its old plan ignored request idempotency and implied ad hoc multi-call semantics. Rewritten and blocked by `afc-102`. |
| `afc-89` | **REWRITE** | Import remains a valid later feature; old wording was GitHub-first at the core boundary and claimed idempotency before such a contract exists. Rewritten provider-neutral and blocked by `afc-102`. |
| `afc-90` | **REWRITE** | Writeback remains reporting-only. Rewritten with authority/idempotency rules and blocked by `afc-89`. |
| `afc-97` | **REWRITE** | `CreateNote` has no terminal-status guard (`issues.go:1691+`), so the claim that no note can be appended to a closed issue is stale. The real missing feature is typed referential supersession. |
| `afc-98` | **BLOCKED** | Authenticated network transport widens the trust boundary before local correctness is proven. Deferred behind `afc-102` and a new accepted threat-model packet. |
| `afc-99` | **BLOCKED** | Its source ADR was Proposed and it is an alternative proxy-fronted design, not an executable leaf. Deferred; related to `afc-98`, not made its child. |

No live task was declared done merely because code with a similar title exists.
The two `DONE IN PRACTICE` decisions have live terminal status, commits, and
tests. No product task was deleted during cleanup.

## 9. Missing Tasks

Each missing task below is tied to a concrete audited risk and now exists under
epic `afc-102`. Full scope and acceptance are canonical in `tasks.md`.

| New leaf | Concrete missing protection |
| --- | --- |
| `afc-103` / AFC-SDD-0151 | monotonic lease generation and public fencing identity |
| `afc-104` / AFC-SDD-0152 | atomic heartbeat/release token+generation+expiry CAS |
| `afc-105` / AFC-SDD-0153 | transaction-local version/lease CAS for update/handoff/close |
| `afc-106` / AFC-SDD-0154 | ready-qualified claim and removal of holder-only token recovery |
| `afc-107` / AFC-SDD-0155 | serialized dependency cycle check and write |
| `afc-108` / AFC-SDD-0156 | daemon singleton, writer serialization, per-connection PRAGMAs, file modes |
| `afc-109` / AFC-SDD-0157 | fail-closed `issue run` behavior after ownership loss |
| `afc-110` / AFC-SDD-0158 | deterministic multi-connection proof for all six races |
| `afc-111` / AFC-SDD-0159 | durable operation ledger and idempotency conflict contract |
| `afc-112` / AFC-SDD-0160 | retry-safe create and claim |
| `afc-113` / AFC-SDD-0161 | retry-safe heartbeat/release/update/handoff/close |
| `afc-114` / AFC-SDD-0162 | crash/restart/WAL/migration/restore matrix |
| `afc-115` / AFC-SDD-0163 | minimum safety telemetry and audit closure |
| `afc-116` / AFC-SDD-0164 | agent decision protocol for lease loss and ambiguous retry |

There is no task to replace SQLite, add a background reaper, introduce event
sourcing, or open a network listener because the audit found no correctness
need for those changes.

## 10. Maturity Assessment

| Level | Status | Reason |
| --- | --- | --- |
| 1. Single agent local use | **READY** | Core CRUD, claim/close/handoff, migrations, backup, and recovery UX are usable when concurrency/ambiguous retry is controlled by one operator. |
| 2. Multiple agents sequentially | **MOSTLY READY** | Leases and audit work, but a crashed worker and lost response still require manual reconciliation and may duplicate external work. |
| 3. Multiple concurrent agents | **NOT READY** | Heartbeat/update fencing, direct-claim readiness, dependency serialization, and real concurrency proof are incomplete. |
| 4. Crash/restart-safe coordination | **NOT READY** | Transactions help, but operation IDs, power/restart matrix, and explicit SQLite durability contract are missing. |
| 5. Long-running unattended daemon | **NOT READY** | Singleton enforcement, fail-closed runner behavior, safety telemetry, and startup integrity policy are missing. |
| 6. Shared coordination service for an agent factory | **NOT READY** | Levels 3-5 are prerequisites; remote identity/transport is additionally deferred by design. |

The target of packet 015 is levels 3 and 4, followed by the minimum local
requirements for level 5. Level 6 still requires a separate accepted transport
and authorization packet if workers are off-host.

## 11. Top Risks

1. **Stale-owner mutation:** `UpdateIssue` can validate an old version/lease and
   later update by ID only.
2. **False heartbeat ownership:** a deleted/replaced lease can yield heartbeat
   success, causing an agent to continue after losing ownership.
3. **Unenforced writer boundary:** a second daemon can unlink the active socket;
   pooled SQLite connection settings do not match the architecture contract.
4. **Ambiguous committed mutations:** no operation ID makes network timeout
   indistinguishable from failed commit.
5. **Claim/ready mismatch:** blocked work can be claimed directly; holder text
   can recover a token.
6. **Dependency-cycle race:** validation and insert do not share a serialized
   transaction; traversal errors are treated as no edge.
7. **Runner continues after lease loss:** correct server rejection is not enough
   if the launcher keeps the child alive.
8. **False-green test model:** key suites serialize through one SQLite
   connection and have no deterministic crash points.
9. **Unproven reboot/restore behavior:** backup support exists, but runtime WAL,
   migration, and restore are not an automated end-to-end gate.
10. **Same-UID trust ambiguity:** file modes and docs can imply stronger
    protection than the OS boundary actually provides.

## 12. Recommended Roadmap

The roadmap has four stages and keeps correctness ahead of features.

### Stage 1 — Ownership and serialization correctness

Implement `afc-103` through `afc-109`: lease generation, atomic lease-bound
mutations, ready-qualified claim, dependency serialization, daemon/SQLite
single-writer enforcement, and fail-closed runner behavior. Each leaf includes
its own regression tests. `afc-107` and `afc-108` can proceed in parallel with
the lease track after planning closes.

### Stage 2 — Concurrency proof

Run `afc-110` only after the component leaves. It creates the production-like
multi-connection harness and proves all six required races under deterministic
interleavings and `go test -race`.

### Stage 3 — Durability and retry recovery

Implement the operation ledger (`afc-111`), adopt it for entry operations
(`afc-112`) and lifecycle mutations (`afc-113`), then execute black-box
crash/restart/WAL/migration/restore proof (`afc-114`). This is the gate for
safe autonomous retries and daemon restarts.

### Stage 4 — Operability and agent protocol

Add only the minimum safety audit/health signals (`afc-115`) and publish the
machine-usable lease-loss/retry decision contract (`afc-116`). Reassess maturity
before unblocking bulk UX, tracker adapters, or any network transport.

## 13. NEXT 3

Exactly three leaves should be taken next:

1. **`afc-103` — Add monotonic lease generation.** Problem/evidence:
   migrations and claim expose token/attempt/version but no lease generation.
   Why now: it is the common contract for all fencing predicates. Scope,
   out-of-scope, acceptance, and dependency are in AFC-SDD-0151; it depends only
   on planning `afc-101`.
2. **`afc-104` — Make heartbeat and release atomic CAS operations.**
   Problem/evidence: `issues.go:565-660` separates validation from mutation and
   does not verify affected rows/expiry. Why now: heartbeat is the worker's
   ownership signal. The bounded scope and tests are AFC-SDD-0152; it depends on
   `afc-103`.
3. **`afc-105` — Fence update, handoff, and close.** Problem/evidence:
   `UpdateIssue` has the highest-impact TOCTOU at `issues.go:803-975`; the two
   stronger terminal paths still need generation. Why now: this blocks stale
   workers from modifying or closing a reclaimed task. The bounded scope is
   AFC-SDD-0153; it depends on `afc-103`.

`afc-108` is the highest-priority parallel architecture leaf after these three;
it is not smuggled into one of them because daemon locking and SQLite connection
ownership require a separate review surface.

## 14. START HERE

**Start only `afc-103` / AFC-SDD-0151.** It defines and migrates the ownership
identity that heartbeat, update, handoff, close, claim, events, CLI output, and
factory publishers must agree on. It unlocks `afc-104`, `afc-105`, and
`afc-106`; without it, independent fixes would likely encode different fencing
keys or require repeated migrations. This is the smallest leaf with the largest
dependency-unblocking effect. It changes no external transport or product UX.

## 15. Backlog Cleanup Proposal

The proposal was applied deliberately through the daemon; no SQLite file was
edited and no old product issue was silently closed.

1. **Reconcile packet 011.** Mark its original IDs as historical, record
   replacement outcomes (`afc-67`/`afc-68` done, `afc-69` cancelled), and carry
   only rewritten `afc-70` forward. Add the missing packet artifacts so active
   packet selection is no longer stuck on a stale README.
2. **Gate non-safety work.** Add `blocks afc-102` to `afc-70`, `afc-89`,
   `afc-97`, `afc-98`, and `afc-99`; add `afc-90 blocks afc-89`. This removes
   them from ready without falsifying their status.
3. **Rewrite inaccurate tasks.** `afc-70` now requires retry-safe per-item
   semantics; `afc-89`/`afc-90` preserve coordinator authority and a
   provider-neutral boundary; `afc-97` asks for typed supersession rather than
   claiming notes on closed issues are impossible.
4. **Defer unaccepted transport choices.** `afc-98` and `afc-99` are `deferred`
   until local safety and an accepted threat-model/transport packet exist.
5. **Create executable safety leaves.** Epic `afc-102` and `afc-103`-`afc-116`
   mirror `tasks.md`; all are initially blocked by planning issue `afc-101`.
   Autonomous tags appear only on bounded implementation/test/doc leaves.
6. **Keep one clean handoff to execution.** After this planning PR merges and
   `afc-101` closes, manual `START HERE` is `afc-103`; the only independent
   factory-ready safety leaf is `afc-107`, while `afc-108` is a manual parallel
   architecture leaf. Later leaves unlock from live dependency state.

The cleanup intentionally does not reactivate deferred network work, close the
safety epic, or claim any implementation leaf. Those actions require later
evidence and explicit execution.
