# Design

`internal/report.Build` already loads projects, issues, ready issues, and
events for the global report. It groups those records by project ID and emits
`by_project` keyed by project key without another store query or migration.
Current status counts are snapshots; ready is a subset of open. The trailing
seven-day counters use the inclusive interval `[until - 7 days, until]`
independently of the report's `since` filter. Close counts include normal and
operator close events, following the existing flow event semantics.

The JSON field is an object for an unfiltered report, including `{}` when no
projects exist, and `null` for a project-filtered report. Other JSON fields
are unchanged. The CLI uses an aligned table and computes each row's share
from its total divided by `inventory.total`. It formats percentile seconds
only at the presentation boundary.

The optional grouping flag and project-list counts in the issue proposal are
outside this packet; the required default view uses the project breakdown.
