# Agent Protocol v1

## Session loop

Every agent session follows this cycle:

1. **Pick ready work**
   ```
   afctl issue ready --json
   ```
   Returns the highest-priority unclaimed, unblocked issues. Epics never
   appear here and cannot be claimed — work on their children instead.
   The `issue_type` field (`task`, `bug`, `feature`, `chore`) tells you
   what kind of work it is; a `bug` starts from reproduction, a `feature`
   from the linked spec.
   The `acceptance_criteria` field, when present, lists the conditions the
   issue must meet before you close it — treat it as the definition of done
   and verify each item.
   Pick one and note its `short_id` (e.g. `afc-42`).

2. **Claim it**
   ```
   afctl issue claim <short_id> --actor <name> --ttl 900
   ```
   Exports `lease_token`. Keep it secret — it proves your right to mutate the issue.

   Default TTL is 3600s. Use `--ttl 900` for shorter leases.

3. **Heartbeat during work**
   Extend your lease every ⅓ of TTL (every 300s for 900s TTL):
   ```
   afctl issue heartbeat <short_id> --lease-token <token> --ttl 900
   ```

4. **Note progress**
   Attach findings or blockers to the issue:
   ```
   afctl issue note add <short_id> --actor <name> --body "message"
   ```
   When stopping without closing, always leave a final note starting with `HANDOFF:` so the next agent knows where to start.

5. **Close or release**
   ```
   afctl issue close <short_id> --resolution done --expected-version N --lease-token <token> \
     --branch <branch> --pr-url <url> --commit-sha <sha> --note "what was done"
   afctl issue release <short_id> --lease-token <token>
   ```

## Event ordering

Issue timelines, the global event feed, and JSONL export are ordered by the
daemon-assigned event `sequence`. Treat `created_at` as wall-clock metadata:
it can be tied and does not establish causal order. Legacy records before an
`event_ordering_enabled` marker have deterministic display order only.

## Exit codes

Commands with `--json` succeed or fail with typed exit codes so the caller can react without parsing prose:

| Code | Meaning | Reaction |
|------|---------|----------|
| 0 | Success | — |
| 1 | Hard failure (daemon down, bad syntax) | Fix and retry |
| 2 | `version_conflict` | Reread issue, retry |
| 3 | `lease_held` | Pick other ready work |
| 4 | `lease_expired` | Re-claim before continuing |
| 5 | `not_found` | Check issue ID |
| 6 | `dependency_cycle` | Fix dependency graph |

## Scope rules

- Claim an issue before mutating files that belong to it.
- One claim per agent at a time, unless the tasks are trivially coupled (same repo, same session).
- **Identity**: `afctl` automatically infers your agent name and process PID from the process tree (e.g. `agy-4725`). You may optionally override this by exporting `AF_COORDINATOR_ACTOR=<agent-name>`.
- Resolve actor from: `--actor` flag > `AF_COORDINATOR_ACTOR` env variable > process tree climbing > `USER` env variable > error.

## Worktree hygiene

- If the coordinated checkout is a read/merge anchor, do implementation in a sibling worktree, not in that coordinated checkout.
- After a non-main task worktree is merged and removed from disk, clean stale coordinator records with:
  ```
  afctl worktree prune --repo <repo-id>
  ```
- If you know the exact safe-to-delete worktree record, remove it directly with:
  ```
  afctl worktree unregister --worktree <worktree-id>
  ```
- `prune` and `unregister` only remove non-main worktrees that no longer have issue or artifact references.

## Prohibitions

- Do not open the coordinator database directly.
- Do not edit files in a coordinated repo without an active claim.
- Do not restate spec contents in issue descriptions — link to the specification file instead.
- Do not commit from within a worktree that is the coordinated checkout — use a sibling worktree.
- Do not close an issue without a note — the audit trail is for whoever comes after you.
