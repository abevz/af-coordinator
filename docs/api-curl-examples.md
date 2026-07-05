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
Supports query parameters: `?project=afc&status=open`
```bash
curl -s --unix-socket $AFC_SOCK "http://localhost/v1/issues?project=afc&status=open" | jq
```

### Get a single issue by Short ID
```bash
curl -s --unix-socket $AFC_SOCK http://localhost/v1/issues/afc-15 | jq
```

### Get the next "ready" issue
Returns the highest-priority open issue that has no unresolved blockers.
```bash
curl -s --unix-socket $AFC_SOCK "http://localhost/v1/issues/ready?project=afc" | jq
```

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
    "title": "Investigate database locks",
    "description": "We are seeing intermittent SQLite locks.",
    "priority": 2,
    "actor": "'"$AFC_ACTOR"'"
  }' \
  http://localhost/v1/issues | jq
```

### Update an issue (PATCH)
Use this to update descriptions, titles, or priorities. If the issue is `in_progress`, you must also provide the `lease_token`.
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
    "actor": "'"$AFC_ACTOR"'"
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
