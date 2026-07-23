# Operator Ergonomics

Status: open; `afc-63` through `afc-66` are the active leaves.

This packet improves the interactive operator experience with `afctl` based
on friction observed during multi-day production use of the coordinator
across the Aion Forge project (200+ issues, 50+ closed in 3 days).

The track has four ordered outcomes:

1. **afc-63**: make `operator-close` a complete single-command closure with
   PR/SHA/note metadata and optional auto-version resolution;
2. **afc-64**: eliminate the mandatory `--expected-version` round-trip for
   all interactive operator mutation commands;
3. **afc-65**: suppress status-flapping noise from agents/workers with a
   cooldown guard and optional `--reason` field;
4. **afc-66**: support bulk issue IDs in operator mutation commands.

## Motivation

During a housekeeping session on 2026-07-22, closing a single already-merged
task (`aion-235`) required three sequential commands: unblock (update status
to open with `--expected-version`), claim (to obtain a lease token), and
close (with `--lease-token`, `--expected-version`, and all PR metadata).
Cancelling two obsolete tasks (`aion-248`, `aion-249`) doubled the ceremony.

Meanwhile, the audit trail for several tasks showed 20-40 meaningless
`open↔blocked` status transitions per day from worker agents, making the
event timeline unreadable.

## Discovery

`operator-close` already exists (`cmd/afctl/cmd_issue.go` L683-736) but
lacks metadata fields (`--branch`, `--pr-url`, `--commit-sha`, `--note`)
and still requires `--expected-version`. The implementation plan enriches
the existing path rather than creating a new one.
