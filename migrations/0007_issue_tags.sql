-- Namespaced tags classify/route issues (mirrors the vault's type/*,
-- status/*, source/* convention) without carrying state: status, lease, and
-- version stay first-class columns on `issues`. A tag is a plain
-- (issue_id, tag) fact with no other payload.
create table issue_tags (
  issue_id   text not null references issues(id) on delete cascade,
  tag        text not null,
  created_at text not null,
  primary key (issue_id, tag)
);

create index idx_issue_tags_tag
  on issue_tags(tag);
