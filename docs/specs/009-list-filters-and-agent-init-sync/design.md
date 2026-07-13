# Design

## 1. Filter Representation And Compatibility

Keep the existing `IssueListParams` scalar fields for source compatibility and
add normalized value slices for project, status, and issue type. The transport
normalizes repeated query keys and comma-separated elements into those slices;
the legacy scalar path remains valid for direct callers that pass exactly one
value.

The client keeps its existing single-string `ListIssues` method and adds an
explicit filter request method for callers that already have value slices. The
CLI uses the filter request method after parsing CSV values. This avoids a
surprise signature break for existing Go callers while making the multi-value
contract explicit.

## 2. API And Store Semantics

`GET /v1/issues` accepts either of these equivalent forms:

```text
?project=afc,aion&type=epic,chore&status=open,in_progress
?project=afc&project=aion&type=epic&type=chore&status=open&status=in_progress
```

Transport parsing trims values and rejects empty CSV segments. It validates
each type value using the existing public enum. The SQLite store resolves every
project key to an ID and emits one `IN (...)` predicate per multi-value filter.
An unknown project retains the current typed `not_found` behavior. Values within
one predicate are ORed; predicates combine with `AND`.

Repository, worktree, assignee, and external-key filters remain scalar in this
slice. Their current behavior is preserved; no wildcard or partial matching is
introduced. Result ordering becomes `updated_at DESC, id ASC` for reproducible
multi-project views.

## 3. CLI Parsing

The `ls` shortcut delegates to the same parser as `issue list`. The parser has
one usage string, recognizes `--help`, requires a value after every value flag,
and rejects unknown flags. It parses only project/type/status as CSV; the other
existing filters stay scalar. JSON still emits the existing issue array and the
human table is unchanged.

## 4. Managed Instruction Block

`cmd/afctl/init-snippet.md` remains a short bootstrap block and explicitly
points readers to `afctl protocol` and the canonical
`docs/agent-protocol-v1.md`. Update its session-cycle wording from
`note -> close` to `handoff/close`; do not copy detailed token, event, or
operator semantics into every repository's `AGENTS.md`.

The existing markers remain v1 because their structure and replacement
algorithm are unchanged. After installing the updated CLI, run dry-run first
for each registered canonical checkout, then run `afctl init` only where that
report confirms the intended managed-block change. The command's existing
marker replacement preserves text outside that block.

## 5. Out Of Scope

- report/statistics work from packet 008;
- multi-value repository, worktree, assignee, or external-key filters;
- pagination behavior beyond the existing API;
- a central fan-out command that mutates every repository automatically;
- changes to lease, HANDOFF, close, or operator authorization.
