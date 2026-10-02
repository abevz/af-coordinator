# Requirements

- Document stable actor identities and conservative report-only normalization;
  retain raw event actors and expose aliases without inventing engine identity.
- `dibs stats --by actor` gives claim-owner counts, done close rate and median
  completed attempt duration; claim/outcome totals reconcile with global lines.
- `--top N` ranks scoped issues by claims, then completed attempt seconds, with
  releases, expiry, operator releases, notes and observed lead time.
- Define/test repeated claims released without progress. Treat legacy ordering
  as uncertain, not proof that no note occurred.
- Keep JSON field names/types compatible; additive sections only. Reuse existing
  source/window semantics and project formatting; no DB writes or migrations.
- Multi-actor, window/filter, normalization and real API/CLI tests; validate the
  historical afc-96/aion-551 leaders using an uncommitted read-only export copy.
