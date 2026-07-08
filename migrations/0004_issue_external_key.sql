alter table issues add column external_key text not null default '';

create index idx_issues_external_key
  on issues(external_key);
