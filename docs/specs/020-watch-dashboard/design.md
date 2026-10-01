# Design

Reuse GET /v1/stats for all-project inventory. Add optional last_event_at to
its project rows, selected by event sequence (timestamp is metadata), without
schema changes. watch snapshots reuse the counts and include sorted project
rows; dormant rows sort last then project key. Project issue lanes continue
using issue/ready reads and cursor-based recent events.

The read-only watch Source includes stats and detail (issue and notes) APIs.
Issue details are loaded on selection, not as an N+1 call on every refresh.
A selected project's lane list is scrollable; stable issue IDs retain cursor
selection across refresh. Details have independent scrolling and five latest
notes. Navigation while a snapshot request runs waits for its completion,
so a prior scope's reply cannot replace a new view. Errors preserve the last
complete snapshot, and detail failures remain visible until refresh/back.

--once uses a separate unbounded text renderer; the interactive renderer
retains terminal bounds and an explicit footer. Clock-only updates recalculate
lease TTL and move expired leases to STALE without writes. No PID liveness
inference. Lazy-expiry API responses hide expired leases, so stale age then
uses updated_at and is labelled "since update", not claimed as expiry age.
