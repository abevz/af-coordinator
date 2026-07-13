-- A lease is the current ownership record, while lifecycle events preserve its
-- history. Existing live leases receive deterministic legacy identifiers so
-- release and close remain auditable after an upgrade.
alter table leases add column attempt_id text not null default '';
alter table leases add column session_id text not null default '';

update leases
set attempt_id = 'legacy-' || issue_id
where attempt_id = '';

create unique index idx_leases_attempt_id
  on leases(attempt_id);
