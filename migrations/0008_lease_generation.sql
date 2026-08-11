-- Each fresh claim advances an issue-local fencing generation. Existing
-- active leases become generation 1; issues without a lease remain at 0 so
-- their first claim also receives generation 1.
alter table issues add column lease_generation integer not null default 0
  check (lease_generation >= 0);

alter table leases add column lease_generation integer not null default 0
  check (lease_generation >= 0);

update issues
set lease_generation = 1
where exists (select 1 from leases where leases.issue_id = issues.id);

update leases
set lease_generation = 1;
