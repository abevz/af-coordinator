# Review

Status: complete; `afc-54` is ready for closure.

## Planning Review

- The live CLI rejected comma-separated issue types and represented every list
  filter as one scalar value.
- `afctl ls --help` was silently ignored by the permissive list parser and
  made a daemon list request instead of printing usage.
- The canonical protocol already routes unfinished work through atomic
  HANDOFF, while the managed init snippet still described `note -> close`.
- Existing `afctl init --dry-run` checks proved that sync is per target path,
  not a global fan-out operation.

## Design Conclusions

- CSV input belongs at the CLI boundary, while the API accepts both CSV and
  repeated parameters for direct consumers.
- New slice fields plus a new client filter method preserve old scalar caller
  signatures.
- A concise init block should point to the canonical protocol rather than
  duplicate detailed workflow rules across repositories.

## Implementation Review

- `IssueListParams` preserves its scalar fields and adds normalized project,
  status, and issue-type slices. The existing `Client.ListIssues` signature is
  retained; `Client.ListIssuesWithFilters` emits repeated query keys for new
  callers.
- The API accepts repeated and CSV values, trims whitespace, rejects empty
  elements as `validation_failed`, and validates every selected issue type.
  SQLite resolves each project key, uses parameterized `IN (...)` predicates,
  and orders views by `updated_at DESC, id ASC`.
- `afctl ls` and `afctl issue list` share a strict parser and help text.
  Unknown or value-less flags now fail locally. Main-command version probing
  skips either help form, so help is usable without a daemon.
- The managed init block now identifies `afctl protocol` and
  `docs/agent-protocol-v1.md` as canonical and states the concise
  `ready -> claim -> heartbeat -> handoff/close` path. The detailed protocol
  itself remains canonical and was not duplicated per project.

## Verification

- Focused CLI, core, client, API, and SQLite tests passed; new regressions
  cover CSV, repeated values, whitespace, empty elements, invalid types,
  intersections, ordering, and local help/flag errors.
- `go test ./...`, `go build -buildvcs=false ./...`, and `make test`
  (`go test -race ./...`) passed.
- A scratch daemon with isolated `AF_COORDINATOR_DB` and
  `AF_COORDINATOR_SOCKET` accepted both requested list forms, returned only
  the intended issue types, and produced identical help for `ls` and
  `issue list`.
- `make restart-service` installed the binaries and the live daemon reported
  healthy. Live `afctl ls` returned 1 match for the `afc` form and 3 for the
  `afc,aion` form.
- All eight registered canonical checkouts had an `afctl init --dry-run`
  before the real sync. Each real sync reported `updated`; an immediate second
  dry-run reported `unchanged`, preserving content outside the managed block.

## Scope Review

Implementation matches requirements and design. No mutation authorization,
lease semantics, issue-state transitions, pagination, reporting read model,
or direct SQLite access boundary changed.
