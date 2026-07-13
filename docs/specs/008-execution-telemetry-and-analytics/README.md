# Execution Telemetry And Analytics Hardening

Status: in progress; `afc-49`, `afc-50`, and `afc-51` are implemented, with
the remaining slices deferred.

This packet hardens the audit contract that already backs af-coordinator and
then adds a small, local execution report over trustworthy coordinator data.
It is grounded in the first ten days of live multi-project use rather than in
an assumed observability stack.

The track has three ordered outcomes:

1. preserve causal event order and enforce close authorization/state
   transitions;
2. record lease-attempt outcomes and make HANDOFF plus release atomic;
3. expose the resulting project statistics through `afctl` and the daemon API.

The packet does not turn af-coordinator into an execution engine. A lease
attempt describes ownership of coordinator work. Temporal, Aion Forge, or
another runner still owns workflow retries, process state, and execution
internals.

The operator selected this track and `afc-49` through `afc-51` are completed
delivery slices. The remaining implementation issues remain deferred. Live status,
claims, dependencies, notes, and closure audit remain in af-coordinator; the
files in this packet own scope and design.

Supporting evidence is recorded in `evidence.md`. The canonical task slices
and dependency order are in `tasks.md`.
