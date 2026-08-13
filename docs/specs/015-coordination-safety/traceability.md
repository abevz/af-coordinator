# 015 Coordination Safety Traceability

Status values describe implementation, not specification completeness.

| Requirement | Primary leaves | Audit evidence | Status |
| --- | --- | --- | --- |
| R-01 authoritative restart state | `afc-114` | Current Architecture; Crash/Recovery Analysis | planned |
| R-02 one mutation authority | `afc-108` | Authoritative state; Write ownership; SQLite correctness | verified by PR `#50`, CI, and installed two-daemon/restart black-box at `28b0f80` |
| R-03 atomic ready-qualified claim | `afc-106`, `afc-110` | Claim semantics; Race 1; Dependencies | implemented by `afc-106`; final multi-connection matrix pending `afc-110` |
| R-04 lease identity and fencing | `afc-103`, `afc-104`, `afc-105`, `afc-106` | Claim semantics; Races 2-4 | generation and secret-safe claim implemented by `afc-103`/`afc-106`; mutation fencing pending `afc-104`/`afc-105` |
| R-05 heartbeat and release | `afc-104`, `afc-113` | Lease/TTL; Heartbeat; Race 2 | planned |
| R-06 update/handoff/close | `afc-105`, `afc-113` | Handoff; Close; Races 3-5 | planned |
| R-07 dependency/ready consistency | `afc-107`, `afc-110` | Dependencies / ready queue | implemented by `afc-107`: serialized add-dependency transaction, returned traversal errors, and cross-project endpoint policy; final multi-connection matrix pending `afc-110` |
| R-08 lease time semantics | `afc-104`, `afc-114` | Lease/TTL; restart failure cases | planned |
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
