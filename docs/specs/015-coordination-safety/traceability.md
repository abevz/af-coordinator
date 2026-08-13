# 015 Coordination Safety Traceability

Status values describe implementation, not specification completeness.

| Requirement | Primary leaves | Audit evidence | Status |
| --- | --- | --- | --- |
| R-01 authoritative restart state | `afc-114` | Current Architecture; Crash/Recovery Analysis | planned |
| R-02 one mutation authority | `afc-108` | Authoritative state; Write ownership; SQLite correctness | verified by corrected PR `#60`, automated two-process recovery, CI, and scratch installed black-box at `41d5517`; cooperative same-UID boundary remains explicit |
| R-03 atomic ready-qualified claim | `afc-106`, `afc-110` | Claim semantics; Race 1; Dependencies | implemented by `afc-106`; final multi-connection matrix pending `afc-110` |
| R-04 lease identity and fencing | `afc-103`, `afc-104`, `afc-105`, `afc-106` | Claim semantics; Races 2-4 | generation and claim fencing verified; afc-105 expiry-window correction implemented with merge pending; final cross-operation matrix remains `afc-110` |
| R-05 heartbeat and release | `afc-104`, `afc-113` | Lease/TTL; Heartbeat; Race 2 | atomic unexpired lease CAS verified by PR `#53`; retry/reconciliation contract remains `afc-113` |
| R-06 update/handoff/close | `afc-105`, `afc-113` | Handoff; Close; Races 3-5 | PR `#55` added generation fencing; reopened expiry-window and affected-row correction implemented with merge pending; retry/reconciliation remains `afc-113` |
| R-07 dependency/ready consistency | `afc-107`, `afc-110` | Dependencies / ready queue | implemented by `afc-107`: serialized add-dependency transaction, returned traversal errors, and cross-project endpoint policy; final multi-connection matrix pending `afc-110` |
| R-08 lease time semantics | `afc-104`, `afc-114` | Lease/TTL; restart failure cases | daemon-time unexpired CAS implemented by `afc-104`; restart and wall-clock robustness proof remains `afc-114` |
| R-09 idempotent mutations | `afc-111`, `afc-112`, `afc-113` | Idempotency; Race 6 | planned |
| R-10 crash/recovery | `afc-114` | Crash/Recovery Analysis; SQLite correctness | planned |
| R-11 protocol and agent decisions | `afc-109`, `afc-116` | Protocol/API; Agent UX | fail-closed `issue run` ownership-loss behavior verified by corrected PR `#58`; broader protocol hardening remains `afc-116` |
| R-12 audit/observability | `afc-115` | Auditability; Observability | planned |
| R-13 verification evidence | each behavior leaf, then `afc-110`, `afc-114` | Six races; eight failure cases | planned |

## Closure rule

A row changes to `verified` only when its primary leaves are done, packet-local
review evidence names the relevant tests/commits, and no required scenario is
`UNSAFE` or `UNKNOWN`. A passing repository-wide test command without the
required focused tests is insufficient.
