# Requirements

## R1: Project breakdown

When no project filter is set, the read-only statistics report shall include
one row for every registered project, including empty projects. Each row shall
contain total, current status counts, ready count, and created and closed
counts for the trailing seven days ending at `until`. Project totals shall
sum to the report inventory total. A project-filtered report shall have no
breakdown in human output. Existing JSON fields and their units shall remain
unchanged.

## R2: Human output

The default CLI output shall show the project count, a table ordered by open
and then ready count descending, and each project's share of the inventory.
Inventory, flow, attempts, and quality shall have visible headings. Durations
shall be rendered in readable seconds, minutes, hours, or days in human output;
JSON durations shall remain seconds.

## R3: Verification

Multi-project, empty-project, filter, time-boundary, API response, and CLI
formatting behavior shall be tested. A scratch daemon and CLI shall verify the
human and JSON reports without touching live coordinator state.
