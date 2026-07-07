# Review

Status: active

## 2026-07-07 - Packet initialized

### What shipped

- Created the Aion Forge integration readiness SDD packet.
- Moved active packet guidance away from completed packet 003.
- Updated packet 003 README status to match its completed review outcome.

### What was verified

- `docs/roadmap.md` lists epic `afc-24` and ready issues `afc-25` through
  `afc-30` as the planned work.
- `docs/specs/003-beads-migration/review.md` declares packet 003 completed.

### Open items

- Implementation tasks `afc-25` through `afc-30`, plus `afc-36`, remain open.
- Each implementation task must update this review with shipped behavior,
  verification, and remaining risks.

## 2026-07-07 - Task IDs aligned with af-coordinator

### What shipped

- Replaced the previous parallel spec-task names with the live `afc-N` issue IDs
  used by the coordinator.
- Added `afc-36` for scoped repository resolution, which was previously present
  in this packet but missing from the live `afc` issue set.
- Kept `afc-37` as the cleanup task for this alignment work.

### What was verified

- `aion-forge` uses the same convention in `docs/specs/010-harness-v2`: task
  IDs are the af-coordinator short IDs, with one ID scheme rather than a
  parallel spec-task namespace.
- `afc-36` and `afc-37` exist in the live coordinator.
- `afc-30`, `afc-36`, and `afc-37` are parent-linked to `afc-24`.
- `afc-30` priority is P1 to match the packet ordering.

### Open items

- Keep `tasks.md` aligned with `afctl issue list --project afc` when new leaves
  are added or closed.
