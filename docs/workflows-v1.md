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
right to the issue, not your trail on it. Read event history in `sequence`
order: timestamps can be tied and do not establish causality.

## Working solo on a backlog

```bash
export AF_COORDINATOR_ACTOR=aleksey

afctl issue create --project utils --scope-kind project \
  --title "rotate backup keys" --priority 2

afctl issue ready                      # the menu
afctl issue claim utils-7 --ttl 28800  # a working day; nobody to compete with
# ... work ...
afctl issue close utils-7 --resolution done \
  --expected-version N --lease-token <token> \
  --branch codex/utils-7 --pr-url https://example/pr/7 --commit-sha abc1234
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

## Inspecting project flow

Use the read-only report when you need a project or repository-level view,
not a list of actions to claim:

```bash
afctl stats --project afc --since 7d
afctl stats --project afc --repo af-coordinator --since 2026-07-01T00:00:00Z
afctl --json stats --project afc --since 24h | jq '.flow, .attempts, .coverage'
```

The human view shows current inventory plus flow and coverage summaries. JSON
is a versioned report for automation. `since` is RFC 3339 or a positive Go
duration; `until` is RFC 3339 and defaults to now. Inventory and note/spec
coverage are current snapshots, while creation and transition metrics use the
selected inclusive time window. Read the report's data-quality fields before
making causal claims about events before the sequence cutoff. The report is
local and read-only: it does not claim work, require a lease token, or rank
agents.

## Issue types and epics

Every issue has an `issue_type`: `task` (the default), `bug`, `feature`,
`epic`, or `chore`. Type is classification, not workflow — status
transitions, leases, and dependencies work identically for all types,
with one exception: **epics**.

```bash
afctl issue create --project utils --scope-kind project \
  --type bug --title "backup timer silently skips weekends" --priority 2

afctl ls --type bug                    # everything filed as a bug
afctl issue update utils-9 --type feature --expected-version N   # reclassify
```

Pick the type by what the work *is*, not by size: a `bug` fixes wrong
behavior, a `feature` adds behavior, a `chore` keeps the lights on
(deps, CI, cleanup), a `task` is anything else. Types are also the
routing key for future agent pipelines (a bug agent reproduces first;
a feature agent starts from the spec), so honest classification pays
off later.

### The epic flow

An epic is a container, not a unit of work. Two rules are enforced by
the daemon:

- an epic **cannot be claimed** (`validation_failed`) — you work on its
  children, never on the epic itself
- an epic **never appears in `ready`** — the menu only offers real work

The flow:

```bash
# 1. Create the umbrella
afctl issue create --project utils --scope-kind project \
  --type epic --title "Migrate backups to restic"

# 2. Create children and attach them via a parent dependency
afctl issue create --project utils --scope-kind project \
  --type task --title "inventory current backup jobs"
afctl issue dependency add utils-11 --depends-on utils-10 --kind parent

# 3. Order the children with ordinary blocks dependencies where needed
afctl issue dependency add utils-12 --depends-on utils-11 --kind blocks

# 4. Track progress through the children
afctl ls --project utils --status open     # what remains
afctl show --full utils-10                 # epic with its trail

# 5. When the last child is done, explicitly close the unclaimable epic
afctl issue operator-close utils-10 --resolution done --expected-version N \
  --reason "all children done; restic in production since afc-…"
```

`parent` links are structure only — they do not block anything. A child
is claimable even if its siblings are open; use `blocks` when order
actually matters. The daemon does not auto-close an epic when the last
child closes: closing the umbrella is a deliberate human statement that
the goal, not just the task list, is complete.

## Acceptance criteria

Every issue can carry `acceptance_criteria` — the checkable conditions
for calling it done, separate from the `description`. This is the
queryable counterpart to the `## Verification` section of an SDD leaf:
keep the criteria here (not buried in prose) so "is this really done?"
has one place to look before you close.

```bash
# On create
afctl issue create --project utils --scope-kind project \
  --title "rotate backup keys" --priority 2 \
  --acceptance $'- new keys in vault\n- old keys revoked\n- restore test passes'

# Add or revise later (optimistic version, like any metadata edit)
afctl issue update utils-7 --expected-version N \
  --acceptance $'- new keys in vault\n- old keys revoked'

# Prefer the guided form for multi-line entry
afctl issue create-form            # includes an "Acceptance criteria" field
```

Criteria render in `afctl show <id> --full` (and the daily-check board's
Issue Details pane). Free text — a Markdown bullet list is the norm.
When an issue implements a spec, mirror the leaf's Verification section
here so the tracker and the spec agree.

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

## Attaching specs and documents

When a feature involves a specification or design document, you can link the markdown file to the issue tracking it. The daemon tracks the file's repository path and ties it to the issue.

```bash
# Assuming utils-7 is a repository-scoped issue:
afctl issue link utils-7 --path docs/specs/042-new-parser/design.md \
  --kind spec --relation implements
```

If the document doesn't exist in the daemon's artifact registry yet, this command registers it (upserts it) and links it in one step.

To see what documents are attached to an issue, use `--full`:

```bash
afctl show --full utils-7
```

If your issue is project-scoped, you must provide the repository context explicitly:

```bash
afctl issue link utils-7 --path docs/design.md --repo backend-repo --kind design
```
