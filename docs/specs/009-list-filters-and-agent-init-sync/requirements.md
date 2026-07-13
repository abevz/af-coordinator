# Requirements

## R1: Multi-Value Issue List Filters

- `afctl ls` and `afctl issue list` SHALL accept comma-separated values for
  `--project`, `--type`, and `--status`.
- Values within one filter SHALL be ORed; distinct filters SHALL be ANDed. For
  example, `--project afc,aion --type epic,chore --status open,in_progress`
  returns issues matching any selected value in each category and all three
  categories together.
- Whitespace around comma-separated values SHALL be ignored. Empty values,
  including leading, trailing, or doubled commas, SHALL fail with a typed
  `validation_failed` error instead of being silently ignored.
- Existing single-value requests and clients SHALL keep their behavior.
- Type values SHALL remain validated against the public issue-type enum. A
  missing project key SHALL remain `not_found`; this packet does not invent
  wildcard project names or new status values.
- API and client requests SHALL support both repeated query parameters and
  comma-separated values for those three filters, normalizing them to the same
  result.

## R2: Strict List CLI Contract

- `afctl ls --help` and `afctl issue list --help` SHALL print the same concise
  filter usage without making a daemon request.
- Unknown list flags and flags without a value SHALL fail locally with a clear
  error. They SHALL NOT be silently ignored.
- The human table and stable JSON response shape remain unchanged. Results are
  deterministically ordered by most recently updated issue, then issue ID.

## R3: Managed Agent Guidance Sync

- `afctl protocol` remains the canonical detailed workflow and SHALL continue
  to describe atomic HANDOFF as the normal unfinished-work path.
- `afctl init` SHALL generate a concise managed summary whose session cycle is
  `ready -> claim -> heartbeat -> handoff/close`, explicitly directing
  detailed behavior to `afctl protocol` and the canonical
  `docs/agent-protocol-v1.md` rather than duplicating it.
- `afctl init --dry-run` remains non-mutating. Running `afctl init` updates
  only the target `AGENTS.md` marker block, preserving surrounding repository
  instructions.
- After the template ships, every currently registered repository SHALL first
  receive a dry-run; the real sync may update existing managed blocks or create
  a missing `AGENTS.md` block, but SHALL not modify unmanaged content.

## R4: Verification And Boundaries

- Store, API, client, CLI, and MCP-independent list regressions SHALL cover
  single values, CSV values, repeated API query parameters, intersections,
  whitespace, empty elements, invalid types, and unknown CLI flags.
- Init regressions SHALL cover dry-run, in-place managed-block replacement,
  and preservation of surrounding content.
- Documentation SHALL cover CLI usage, API query semantics, and the division
  between `afctl init` and `afctl protocol`.
- The implementation SHALL not change mutation authorization, leases, issue
  state transitions, analytics/reporting, or direct SQLite access boundaries.
