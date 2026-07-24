# ADR-029: af-coordinator as Control Plane, SCM as Adapter

> Reference copy. Source of truth: `aion-forge/main/docs/decisions/ADR-029-af-coordinator-as-control-plane.md`.
> Kept here for local citation by af-coordinator spec packets (e.g. afc-91's
> tags-never-carry-state invariant). Do not edit this copy independently of
> the source.

## Status

Accepted

Amends ADR-001 (issue-driven stateless control plane) and rescopes ADR-002
(Gitea vs GitHub).

## Context

ADR-001 adopted an Issue-Driven Architecture in which SCM/tracker issues are
the backlog, labels are state-machine markers, and state snapshots live as
machine envelopes in issue comments. ADR-002 chose Gitea as the primary
platform for production autonomous execution, largely because the tracker was
also the state transport and therefore carried high-frequency machine traffic.

Since then, af-coordinator has become a working system: a local daemon with a
SQLite ledger, lease-based claims, optimistic versioning, typed notes, and an
append-only event log. Track 010 (Harness v2) already builds on it and states
the new reality directly:

> af-coordinator is the control plane and task ledger. GitHub/Gitea may be a
> configured SCM/PR adapter, but GitHub Issues are not a Harness v2 runtime
> dependency.

The coordinator project registry records the same posture ("Control plane =
af-coordinator, SCM = GitHub"). This ADR closes the gap between that practice
and the still-Accepted ADR-001/ADR-002 wording.

## Decision

- af-coordinator is the control plane and task ledger for Aion Forge work.
  Issues, claims/leases, dependencies, notes, and the audit event log live in
  the coordinator, not in SCM issue metadata.
- GitHub or Gitea is a pluggable SCM adapter for branches, pull requests, and
  checks — transport for code review and integration evidence, not the task
  ledger and not a runtime dependency of the agent harness.
- Issue labels and issue-comment state envelopes are no longer the state
  transport. The audit chain is owned by coordinator notes/events plus artifact
  references; envelope schemas remain as artifact contracts where used.

## What survives from ADR-001

The architectural properties ADR-001 was chosen for are kept, with the
coordinator as the new carrier:

- auditable workflow decisions -> append-only coordinator event log and notes
- deterministic recovery -> state derivable from the coordinator ledger
- stateless, event-driven orchestration -> unchanged as a principle
- lock TTL and safe unlock -> coordinator leases with TTL and heartbeats
- no chain-of-thought in durable artifacts -> unchanged (see ADR-024)

## What changes in ADR-002 scope

ADR-002's main driver — API quota pressure from machine-to-machine state
traffic through the tracker — largely disappears once state traffic moves to
the local coordinator. Gitea remains a valid self-hosted SCM option for data
residency, but the Gitea-vs-GitHub choice is now an adapter configuration
decision, not an architecture decision. Current default posture is GitHub as
the SCM adapter.

## Consequences

Positive:

- no third-party API quotas or webhook latency in the control loop
- task state queries are local SQL instead of tracker API pagination
- SCM outages degrade PR flow, not task claiming and audit

Negative:

- af-coordinator availability and backup become critical-path concerns
- task state is no longer visible in the SCM UI by default; visibility needs
  coordinator tooling (`afctl`) or an export path

## Follow-up

- Keep `AGENTS.md` Documentation Rules aligned with this ADR where they still
  describe issue comments as the envelope transport.
- Review ADR-005 (dual envelope) wording for the audit-envelope carrier when
  the Harness v2 audit path is implemented.

## Alternatives Considered

- Keep issue-driven state transport (ADR-001 as written): rejected — track 010
  already depends on coordinator semantics (leases, versions, typed notes)
  that trackers do not provide, and quota/latency costs return.
- Run both carriers in parallel: rejected — two sources of truth for task
  state is the exact drift this ADR removes.
