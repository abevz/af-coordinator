# Requirements

## R1 - Stable identifiers for external consumers

Dependency and issue responses must expose identifiers consistently enough for
external consumers to store references without guessing whether a field contains
a UUID or a short id.

## R2 - Scoped repository lookup

Repository lookup by logical name must be safe in a multi-project installation.
An unqualified logical name must not silently resolve to the wrong repository
when multiple projects use the same name.

## R3 - Watchable event stream

External consumers must be able to watch coordinator events through the daemon
API with a monotonic cursor and bounded long-poll behavior.

## R4 - External execution references

Issues must support references to external execution systems such as Temporal
workflow ids, mirrored issue keys, PR URLs, and commit SHAs without making those
systems the source of truth for coordinator issue state.

## R5 - Structured close resolution

Closing an issue must support machine-readable resolution references so an
agent can tell which PR, commit, or external execution completed the work.

## R6 - Export before new integration surfaces

A JSONL export path must exist before introducing broader agent-facing
integration surfaces, so audit, backup, and bridge use cases remain possible
without direct SQLite reads.

## R7 - MCP stays a wrapper

The MCP server must call the daemon API and must not mutate SQLite directly or
define a second coordinator protocol.
