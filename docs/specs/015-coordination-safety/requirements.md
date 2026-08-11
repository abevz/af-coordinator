# 015 Coordination Safety Requirements

## Goal

Make `af-coordinator` a reliable local coordination authority for multiple
concurrent, cooperative AI coding agents, including daemon and client crashes,
without replacing SQLite or widening into a network service.

## Safety vocabulary

- **authoritative state**: durable coordinator state whose canonical copy is in
  SQLite;
- **lease attempt**: one ownership interval identified by a secret token, a
  non-secret attempt ID, and a monotonically increasing generation;
- **fenced mutation**: a mutation that commits only while the same lease
  generation is still active at the mutation's serialization point;
- **ambiguous outcome**: the client timed out or disconnected and cannot know
  whether the daemon committed;
- **ready**: an executable, non-epic issue with an eligible status, no active
  lease, and no unfinished `blocks` dependency.

## Functional requirements

### R-01: Authoritative state and restart recovery

SQLite MUST contain all state needed to reconstruct issues, dependencies,
leases, attempts, notes, links, tags, events, and idempotency outcomes after a
daemon restart. No correctness decision may depend only on daemon memory, a
socket inode, CLI output, or a worker-local file.

### R-02: One mutation authority

The supported write path MUST be `agents/CLI -> daemon -> SQLite`. The daemon
MUST refuse to start as a second writer for the same database. SQLite connection
settings that affect correctness MUST apply to every connection used by the
daemon. Runtime directories and files MUST use least-privilege modes. The
same-UID local process boundary MUST be documented honestly: hostile direct DB
access is an OS-isolation problem, not something HTTP can prevent.

### R-03: Atomic ready-qualified claim

A claim MUST decide issue eligibility, dependency readiness, current lease,
new lease insertion, issue status/version change, and audit event in one
serialized transaction. Two distinct workers racing for the same issue MUST
produce exactly one valid owner. Direct claim MUST NOT bypass the predicate used
by the ready view.

### R-04: Lease identity and fencing

Every newly acquired lease MUST have a secret unguessable token, a non-secret
attempt ID, and a monotonically increasing issue-local generation. Claim MUST
return all three. Every lease-bound mutation MUST validate token, generation,
and expiration atomically with its write. A stale attempt MUST be unable to
mutate coordinator state after expiry, release, handoff, close, or reclaim.

### R-05: Heartbeat and release

Only the current unexpired lease may heartbeat or release. A heartbeat racing
with expiry/reclaim MUST either renew the still-current attempt or fail; it MUST
never resurrect or report success for a replaced attempt. Release MUST fail for
an expired or replaced lease. Repeated requests MUST follow R-09.

### R-06: Update, handoff, and close

Lease-bound update, handoff, and close MUST combine ownership, expiry, expected
version where applicable, state-transition validation, mutation, lease removal,
and audit writes in one transaction. A stale owner MUST NOT close or update work
owned by a later generation. Handoff MUST keep its required note and lease
release atomic. A duplicate close or handoff with the same operation ID MUST
return the original committed outcome; a different operation against terminal
state MUST fail explicitly.

### R-07: Dependency graph and ready queue

Adding a `blocks` edge MUST validate both endpoints, test for cycles, insert the
edge, and append its event in one serialized transaction. Storage errors during
cycle traversal MUST be returned, never interpreted as no cycle. Closing a
blocker MUST make dependants visible through the computed ready view without a
separate mutable ready flag. Missing dependencies and cross-project policy MUST
be rejected consistently.

### R-08: Lease time semantics

Only daemon time may create or evaluate lease expiration. Persisted expirations
MUST survive restart. Backward wall-clock movement may conservatively delay
reclaim but MUST NOT create a second owner; forward movement may expire a lease
early, after which fencing MUST reject the old worker. Expiry may remain lazy if
ready and claim semantics are correct and the expiry outcome is auditable.

### R-09: Idempotent mutations

Create, claim, heartbeat, update, handoff, close, and release MUST accept a
client operation ID. The daemon MUST persist the operation identity, request
fingerprint, and committed outcome in the same transaction as the mutation.
Retrying the same operation after an ambiguous outcome MUST return the original
result without another side effect. Reusing an operation ID with a different
request MUST fail with a typed conflict.

### R-10: Crash and recovery behavior

Committed transactions MUST survive daemon restart and host reboot under the
documented SQLite durability setting. Interrupted mutations MUST be wholly
committed or wholly absent. Startup MUST fail clearly on migration failure,
failed integrity checks required by policy, or inability to establish the
single-writer invariant. Backup/restore verification MUST cover WAL state and
the schema migration ledger.

### R-11: Protocol and agent decisions

The v1 HTTP+JSON contract MUST bound request bodies and server timeouts, reject
malformed or unsupported fields consistently, and expose typed retryable versus
terminal errors. Agents MUST be told when ownership is lost and what to do after
an ambiguous outcome. `afctl issue run` MUST terminate or cancel its child when
heartbeat proves the lease is lost; logging and continuing is forbidden.

### R-12: Auditability and observability

The durable history MUST identify every claim attempt, renewal summary or
counter, expiry/reclaim, release/handoff, close, and rejected stale mutation
without recording lease tokens. Operators MUST be able to observe daemon/DB
health, active leases, expirations, claim conflicts, transaction failures, and
mutation latency. This does not require event sourcing or a remote metrics
stack.

### R-13: Verification evidence

Tests MUST use the embedded migrations and production-like SQLite connections.
They MUST deterministically cover the six races in `audit.md`, ambiguous client
retries, daemon termination at transaction boundaries, restart with active and
expired leases, migration failure, WAL recovery, and backup restore. Each fixed
behavior ships with its own regression test; the final matrix is additional
cross-operation evidence.

## Non-goals

- replacing SQLite merely to gain distributed-system terminology;
- multiple active coordinator nodes or replicated consensus;
- authenticated TCP, proxy, or LAN transport;
- GitHub or other tracker synchronization;
- hostile-process isolation between programs running as the same OS user;
- web UI, TUI polish, or bulk operator convenience;
- event sourcing as the primary persistence model.
