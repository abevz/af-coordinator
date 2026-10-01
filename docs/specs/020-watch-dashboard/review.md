# Review

Implemented afc-177: all-project inventory and event sequence metadata;
STALE detection including clock-only expiry; scrollable project and issue
navigation, details and last five notes; complete unpadded --once output.
The Source exposes only read operations. Tests cover counts, dormant sorting,
age basis, exact expiry, sequence ordering, complete output, actual CLI
--once over a Unix-socket fixture, navigation/back, and stable selection.
Focused and full build/test evidence: /tmp/afc-177-*.log.
Independent final review is recorded in the task handoff. No live service
switch or installation is part of this change.
