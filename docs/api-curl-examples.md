# API curl Examples

The `af-coordinator` daemon exposes a local HTTP API over a Unix domain socket. This allows any standard HTTP client (like `curl`) to query and mutate state without using the `afctl` CLI.

> **Note**: For `curl`, use the `--unix-socket` flag. The base URL hostname is ignored by curl when using a Unix socket, so `http://localhost` is conventionally used.

## Configuration

Set your socket path as an environment variable to make copying these examples easier:

```bash
export AFC_SOCK=~/.local/state/af-coordinator/af-coordinator.sock
export AFC_ACTOR="my-curl-script"
```

---

## 1. Diagnostics & Health

### Check daemon health
```bash
curl -s --unix-socket $AFC_SOCK http://localhost/v1/health | jq
```

---

## 2. Reading Issues

### List all issues
Supports query parameters: `?project=afc&status=open&type=bug`
```bash
curl -s --unix-socket $AFC_SOCK "http://localhost/v1/issues?project=afc&status=open" | jq

# Only bugs:
curl -s --unix-socket $AFC_SOCK "http://localhost/v1/issues?project=afc&type=bug" | jq
```

### Get a single issue by Short ID
```bash
curl -s --unix-socket $AFC_SOCK http://localhost/v1/issues/afc-15 | jq
```

Dependencies in the issue payload expose UUID and short ID explicitly:

```json
{
  "issue": {
    "id": "9342e277-7d81-4eca-bad2-b31bccc67c86",
    "short_id": "afc-15",
    "dependencies": [
      {
        "issue_id": "9342e277-7d81-4eca-bad2-b31bccc67c86",
        "issue_short_id": "afc-15",
        "depends_on_id": "9f03193f-3044-42a2-ae9c-a4756ec1f78d",
        "depends_on_short_id": "afc-12",
        "kind": "blocks"
      }
    ]
  }
}
```

### List "ready" issues
Returns actionable issues that are not leased and not blocked by unfinished
`blocks` dependencies.
```bash
curl -s --unix-socket $AFC_SOCK "http://localhost/v1/issues/ready?project=afc" | jq

# Scope repository lookup by project when logical names are reused across projects:
curl -s --unix-socket $AFC_SOCK "http://localhost/v1/issues/ready?project=afc&repo=main" | jq
```

If you do not provide `project`, use the repository UUID rather than an
ambiguous logical name.

---

## 3. Creating & Modifying Issues

> **Important**: Mutating endpoints require an `actor` field to ensure audit traceability. When modifying existing entities, `expected_version` is required for optimistic concurrency control.

### Create a new issue
```bash
curl -s -X POST --unix-socket $AFC_SOCK \
  -H "Content-Type: application/json" \
  -d '{
    "project": "afc",
    "scope_kind": "project",
    "issue_type": "bug",
    "title": "Investigate database locks",
    "description": "We are seeing intermittent SQLite locks.",
    "acceptance_criteria": "- Root cause identified\n- Repro no longer locks under concurrent writers",
    "priority": 2,
    "actor": "'"$AFC_ACTOR"'"
  }' \
  http://localhost/v1/issues | jq
```

### Update an issue (PATCH)
Use this to update descriptions, acceptance criteria, titles, or priorities. If the issue is `in_progress`, you must also provide the `lease_token`.
```bash
curl -s -X PATCH --unix-socket $AFC_SOCK \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Updated context based on new logs.",
    "expected_version": 1,
    "actor": "'"$AFC_ACTOR"'"
  }' \
  http://localhost/v1/issues/afc-15 | jq
```

---

## 4. The Agent Lifecycle (Claim, Heartbeat, Close)

### Claim an issue
Claiming transitions the issue to `in_progress` and returns a secret `lease_token`.
```bash
curl -s -X POST --unix-socket $AFC_SOCK \
  -H "Content-Type: application/json" \
  -d '{
    "holder": "'"$AFC_ACTOR"'",
    "ttl_seconds": 3600
  }' \
  http://localhost/v1/issues/afc-15/claim | jq
```

*Extract the token from the response, e.g., `export TOKEN="uuid-from-response"`.*

### Heartbeat (Renew Lease)
```bash
curl -s -X POST --unix-socket $AFC_SOCK \
  -H "Content-Type: application/json" \
  -d '{
    "lease_token": "'"$TOKEN"'",
    "ttl_seconds": 3600
  }' \
  http://localhost/v1/issues/afc-15/heartbeat | jq
```

### Release an issue (Back to Open)
Releases the lease without closing the issue.
```bash
curl -s -X POST --unix-socket $AFC_SOCK \
  -H "Content-Type: application/json" \
  -d '{
    "lease_token": "'"$TOKEN"'"
  }' \
  http://localhost/v1/issues/afc-15/release | jq
```

### Close an issue
Resolves the issue (e.g., `done`, `cancelled`). Requires a final note.
```bash
curl -s -X POST --unix-socket $AFC_SOCK \
  -H "Content-Type: application/json" \
  -d '{
    "resolution": "done",
    "branch": "codex/afc-27",
    "pr_url": "https://github.com/abevz/af-coordinator/pull/27",
    "commit_sha": "ba6d011",
    "expected_version": 2,
    "lease_token": "'"$TOKEN"'",
    "actor": "'"$AFC_ACTOR"'",
    "note": "Fixed the locking issue by adjusting WAL parameters."
  }' \
  http://localhost/v1/issues/afc-15/close | jq
```

---

## 5. Notes, Dependencies & Links

### Add a note to an issue
```bash
curl -s -X POST --unix-socket $AFC_SOCK \
  -H "Content-Type: application/json" \
  -d '{
    "body": "Found a workaround for the bug.",
    "author": "'"$AFC_ACTOR"'"
  }' \
  http://localhost/v1/issues/afc-15/notes | jq
```

### Read notes
```bash
curl -s --unix-socket $AFC_SOCK http://localhost/v1/issues/afc-15/notes | jq
```

### Add a dependency (Blocker)
Mark `afc-15` as blocked by `afc-12`.
```bash
curl -s -X POST --unix-socket $AFC_SOCK \
  -H "Content-Type: application/json" \
  -d '{
    "depends_on": "afc-12",
    "kind": "blocks",
    "actor": "'"$AFC_ACTOR"'"
  }' \
  http://localhost/v1/issues/afc-15/dependencies | jq
```

### Add a dependency (Parent)
Mark `afc-15` as a child of `afc-10`.
```bash
curl -s -X POST --unix-socket $AFC_SOCK \
  -H "Content-Type: application/json" \
  -d '{
    "depends_on": "afc-10",
    "kind": "parent",
    "actor": "'"$AFC_ACTOR"'"
  }' \
  http://localhost/v1/issues/afc-15/dependencies | jq
```

### Add a link (Artifact)
Link an external document, URL, or local path to the issue.
```bash
curl -s -X POST --unix-socket $AFC_SOCK \
  -H "Content-Type: application/json" \
  -d '{
    "artifact": "https://github.com/my/repo/pull/1",
    "relation": "implements",
    "actor": "'"$AFC_ACTOR"'"
  }' \
  http://localhost/v1/issues/afc-15/links | jq
```

### Read Audit Events
Get the full chronological audit trail of an issue.
```bash
curl -s --unix-socket $AFC_SOCK http://localhost/v1/issues/afc-15/events | jq
```
