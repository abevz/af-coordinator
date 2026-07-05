alter table issues add column issue_type text not null default 'task'
  check (issue_type in ('task', 'bug', 'feature', 'epic', 'chore'));

create index idx_issues_project_type_status
  on issues(project_id, issue_type, status);
