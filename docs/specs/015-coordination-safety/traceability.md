# 015 Coordination Safety Traceability

Status values describe implementation, not specification completeness.

| Requirement | Primary leaves | Audit evidence | Status |
| --- | --- | --- | --- |
| R-01 authoritative restart state | `afc-114` | Current Architecture; Crash/Recovery Analysis | planned |
| R-02 one mutation authority | `afc-108` | Authoritative state; Write ownership; SQLite correctness | planned |
| R-03 atomic ready-qualified claim | `afc-106`, `afc-110` | Claim semantics; Race 1; Dependencies | planned |
| R-04 lease identity and fencing | `afc-103`, `afc-104`, `afc-105`, `afc-106` | Claim semantics; Races 2-4 | planned |
| R-05 heartbeat and release | `afc-104`, `afc-113` | Lease/TTL; Heartbeat; Race 2 | planned |
| R-06 update/handoff/close | `afc-105`, `afc-113` | Handoff; Close; Races 3-5 | planned |
| R-07 dependency/ready consistency | `afc-107`, `afc-110` | Dependencies / ready queue | planned |
| R-08 lease time semantics | `afc-104`, `afc-114` | Lease/TTL; restart failure cases | planned |
| R-09 idempotent mutations | `afc-111`, `afc-112`, `afc-113` | Idempotency; Race 6 | planned |
| R-10 crash/recovery | `afc-114` | Crash/Recovery Analysis; SQLite correctness | planned |
| R-11 protocol and agent decisions | `afc-109`, `afc-116` | Protocol/API; Agent UX | planned |
| R-12 audit/observability | `afc-115` | Auditability; Observability | planned |
| R-13 verification evidence | each behavior leaf, then `afc-110`, `afc-114` | Six races; eight failure cases | planned |

## Closure rule

A row changes to `verified` only when its primary leaves are done, packet-local
review evidence names the relevant tests/commits, and no required scenario is
`UNSAFE` or `UNKNOWN`. A passing repository-wide test command without the
required focused tests is insufficient.
