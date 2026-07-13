-- Events used to be ordered by (created_at, id). Timestamps have second
-- precision and UUIDs do not describe causal order, so rebuild the table with
-- a daemon-assigned sequence as its row identity and public ordering key.
create table events_v2 (
  sequence integer primary key autoincrement,
  id text not null unique,
  issue_id text references issues(id) on delete set null,
  actor text not null,
  event_type text not null,
  payload_json text not null default '{}',
  created_at text not null
);

-- The old order is deterministic, but timestamps tied within that history are
-- not evidence of causal order. New inserts receive their sequence from
-- SQLite in mutation transaction order.
insert into events_v2 (id, issue_id, actor, event_type, payload_json, created_at)
select id, issue_id, actor, event_type, payload_json, created_at
from events
order by created_at asc, id asc;

-- Do not create a synthetic event for a fresh database: with no legacy rows,
-- the exact-order cutoff is sequence 0. For upgraded databases the marker is
-- the first event whose order is causally exact.
insert into events_v2 (id, issue_id, actor, event_type, payload_json, created_at)
select lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
       substr(lower(hex(randomblob(2))), 2, 3) || '-a' ||
       substr(lower(hex(randomblob(2))), 2, 3) || '-' || lower(hex(randomblob(6))),
       null, 'system', 'event_ordering_enabled',
       '{"legacy_ordering":"deterministic_not_causal"}',
       strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
where exists (select 1 from events_v2);

drop table events;
alter table events_v2 rename to events;

create index idx_events_issue_sequence
  on events(issue_id, sequence);

create index idx_events_sequence
  on events(sequence);
