# 011 Operator Ergonomics Design

## Delivered design

Operator ergonomics stayed in the CLI and existing explicit operator endpoints:

- metadata and notes flow through `operator-close` without fabricating a lease;
- CLI-side `latest`/`--force` resolution supplies a concrete version to the
  unchanged optimistic-concurrency API/store boundary;
- claim output remains the source of version for agent lease lifecycle calls.

Implementation commits are `92812f2` (`afc-67`) and `892a59c` (`afc-68`).

## Rejected/carried design

A generic cooldown/operator-lock mechanism was not accepted. Bulk mutation was
not implemented because independent multi-call behavior needs explicit
idempotency and partial-result semantics. Rewritten `afc-70` is gated behind
packet 015 and requires a new active SDD packet before implementation.
