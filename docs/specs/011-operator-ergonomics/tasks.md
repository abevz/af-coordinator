# 011 Operator Ergonomics Tasks

Coordinator IDs own live status. This table is a historical reconciliation, not
an execution queue.

| Original | Replacement | Final disposition | Evidence |
| --- | --- | --- | --- |
| `afc-63` | `afc-67` | original cancelled; replacement done | commit `92812f2`, merged by `71a9fd0`; operator-close metadata tests |
| `afc-64` | `afc-68` | original cancelled; replacement done | commit `892a59c`, merged by `68f4bc6`; live status done |
| `afc-65` | `afc-69` | both cancelled/stale | live coordinator terminal status; no accepted cooldown/lock design |
| `afc-66` | `afc-70` | original cancelled; replacement carried forward | `afc-70` rewritten as P3, blocked by safety epic `afc-102` |

## Completion rule

Packet 011 has no remaining executable leaf. `afc-70` may proceed only after a
future packet re-accepts its retry-safe bulk semantics; its existence does not
keep this historical packet active.
