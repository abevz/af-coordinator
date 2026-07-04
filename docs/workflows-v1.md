# Workflows v1

Recipes for common situations. The other documents explain the system;
this one explains what to type. Contracts live elsewhere and win on
conflict:

- [architecture-v1.md](architecture-v1.md) — why it behaves this way
  (state machine, lease expiry, mutation matrix)
- [api-v1.md](api-v1.md) — the HTTP contract
- [agent-protocol-v1.md](agent-protocol-v1.md) — the mandatory session
  loop for AI agents

The daemon does not distinguish humans from agents: `actor` is a string,
the protocol is the same `afctl` commands. What differs is rhythm —
agents work in minutes and heartbeat; humans work in hours, get
interrupted, and come back tomorrow. These recipes cover the human
rhythm.

## The one rule that explains everything below

**`ready` is a menu, not your state.** It answers "what can be picked up
right now", recomputed on every call. When your lease expires, the issue
reappears in `ready` for everyone — but nothing about the issue itself is
lost: status stays `in_progress`, `claimed_at` stays set, the event log
keeps your claim, and your notes remain. Expiry deletes your exclusive
right to the issue, not your trail on it.

## Working solo on a backlog

```bash
export AF_COORDINATOR_ACTOR=aleksey

afctl issue create --project utils --scope-kind project \
  --title "rotate backup keys" --priority 2

afctl issue ready                      # the menu
afctl issue claim utils-7 --ttl 28800  # a working day; nobody to compete with
# ... work ...
afctl issue close utils-7 --resolution done \
  --expected-version N --lease-token <token>
```

Pick the TTL for how long you actually intend to hold the issue. Agents
use short TTLs (900s) and heartbeat; a human claiming for the day takes a
long TTL and skips heartbeats. The trade-off is symmetric: a long TTL you
forget to release blocks the issue for everyone until it expires.

## Switching away mid-task

Three exits, in order of preference:

1. **Note + release** — the clean one:

   ```bash
   afctl issue note add utils-7 --body "HANDOFF: parser done, DB write remains"
   afctl issue release utils-7 --lease-token <token>
   ```

   Status returns to `open`, the issue is honestly back in the pool, and
   the `HANDOFF:` note tells the next holder (possibly future you) where
   to start. This is the same discipline the agent protocol mandates.

2. **Park it** — you will come back, and nobody else should take it:

   ```bash
   afctl issue update utils-7 --status deferred --expected-version N \
     --lease-token <token>
   ```

   `deferred` is excluded from `ready` entirely until you unpark it
   (`--status open`). Use this for "not now", not for "blocked by
   another issue" — that is what a `blocks` dependency is for.

3. **Just walk away** — the lease expires on its own and the issue
   reappears in `ready` with status still `in_progress`. Nothing breaks;
   this is the designed behavior for crashed agents. But you left no
   note, so the trail says *what* and *when*, never *how far you got*.

## Coming back: where is my desk?

Do not look in `ready` — look at status:

```bash
afctl ls --status in_progress          # everything started and unfinished
afctl show utils-7                     # details, current lease if any
afctl issue events list utils-7        # who claimed it, when
afctl issue note list utils-7          # the last HANDOFF note
```

Then re-claim and continue. If an agent picked the issue up while you
were away, `claim` fails with `lease_held` (exit code 3) and the events
show who has it.

## Parked vs abandoned

| | deliberately parked | abandoned |
|---|---|---|
| status | `deferred` | `in_progress` |
| in `ready` | no | yes, once the lease expires |
| others can claim | no | yes |
| comes back via | you: `--status open` | anyone: `claim` |

If you want the issue held for you, park it. If you don't mind whoever
gets to it first, walk away — the system self-heals.

## Humans and agents on one backlog

Everything above holds with agents in the pool; the only change is that
"someone else takes it" becomes likely instead of hypothetical.

- The event log already separates humans from agents: agents use stable
  tool names (`claude-code`, `codex`, `codewhale-1`), humans use their
  own name. No extra marking needed for "who did what".
- `assignee` exists (`afctl issue update --assignee aleksey`,
  `afctl ls --assignee aleksey`) but is **advisory in v1**: the `ready`
  view ignores it, so agents still see assigned issues as claimable. To
  actually reserve an issue, park it (`deferred`) or hold a lease.
