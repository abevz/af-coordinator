# 012 Issue Tags — Design

## Schema (R1)

New migration `migrations/0007_issue_tags.sql`:

```sql
create table issue_tags (
  issue_id   text not null references issues(id) on delete cascade,
  tag        text not null,
  created_at text not null,
  primary key (issue_id, tag)
);

create index idx_issue_tags_tag on issue_tags(tag);
```

The composite primary key gives per-issue uniqueness for free (same shape as
`dependencies`' `(issue_id, depends_on_issue_id, kind)` PK). The `tag` index
is unused by part 1 but sets up afc-92's future filter joins without another
migration. Glob-embedded by `migrations/embed.go`; no code change there.

## Core model and validation (R1, R2, R3)

`internal/core/issue.go`:

```go
type Issue struct {
    ...
    Blocks []string `json:"blocks,omitempty"`
    Tags   []string `json:"tags,omitempty"`
}

type CreateIssueRequest struct {
    ...
    Tags []string `json:"tags,omitempty"`
}

// AddTagRequest is the JSON body for POST /v1/issues/{issue_id}/tags.
type AddTagRequest struct {
    Tag   string `json:"tag"`
    Actor string `json:"actor,omitempty"`
}
```

Validation follows the accumulate-then-join style of `ValidateCreateProject`
(`internal/core/project.go`), not a single early-return:

```go
var validTag = regexp.MustCompile(`^[a-z0-9-]+/[a-z0-9-]+$`)

const maxTagLength = 64

// reservedTagNamespaces are namespaces that would let a tag masquerade as
// first-class issue state. Status, lease, and version stay in their own
// columns — see docs/decisions/ADR-029-af-coordinator-as-control-plane.md
// for the framing (the coordinator ledger is the single source of state).
var reservedTagNamespaces = map[string]bool{
    "open": true, "blocked": true, "done": true, "in_progress": true,
    "status": true, "state": true,
}

func ValidateTag(tag string) error {
    var errs []string
    switch {
    case tag == "":
        errs = append(errs, "tag is required")
    case len(tag) > maxTagLength:
        errs = append(errs, fmt.Sprintf("tag must be at most %d characters", maxTagLength))
    case !validTag.MatchString(tag):
        errs = append(errs, "tag must be 'namespace/value' using lowercase letters, digits, and '-' only")
    default:
        ns := tag[:strings.IndexByte(tag, '/')]
        if reservedTagNamespaces[ns] {
            errs = append(errs, fmt.Sprintf("tag namespace %q is reserved for issue state, not tags", ns))
        }
    }
    if len(errs) > 0 {
        return fmt.Errorf("validation_failed: %s", strings.Join(errs, "; "))
    }
    return nil
}
```

The regex's per-segment character classes (`[a-z0-9-]+`) already exclude
`/`, so a second `/` fails the match — no separate double-slash check
needed. `ValidateCreateIssue` calls `ValidateTag` for each `req.Tags` entry.

`internal/core/errors.go` gains one code, since reusing `ErrConflict`
(`"version_conflict"`) for a duplicate tag would be misleading to API
consumers — that code means an optimistic-concurrency mismatch, not a
duplicate resource:

```go
ErrAlreadyTagged = "already_tagged"
```

(mirrors the existing `ErrAlreadyLinked` precedent for artifact links).

## Store layer (R1, R5, R8)

`internal/store/store.go` — add to `CoordinatorStore`:

```go
AddTag(ctx context.Context, issueID string, req core.AddTagRequest) error
RemoveTag(ctx context.Context, issueID, tag, actor string) error
```

New `internal/store/sqlite/issues_tags.go` (mirrors `issues_deps.go`):

```go
func populateTags(ctx context.Context, db *sql.DB, issues []core.Issue) ([]core.Issue, error) {
    // batch SELECT issue_id, tag FROM issue_tags WHERE issue_id IN (...) ORDER BY tag
    // append into issues[idx].Tags, same shape as populateDependencies' batch load
}

func AddTag(ctx context.Context, db *sql.DB, issueID string, req core.AddTagRequest) error {
    if _, _, err := GetIssue(ctx, db, issueID); err != nil {
        return err
    }
    if err := core.ValidateTag(req.Tag); err != nil {
        return err // already validation_failed
    }
    // BEGIN; INSERT INTO issue_tags; isSQLiteConstraintError -> core.ErrAlreadyTagged;
    // insertEvent(ctx, tx, issueID, req.Actor, "issue_tagged", map[string]any{"tag": req.Tag}, now);
    // COMMIT
}

func RemoveTag(ctx context.Context, db *sql.DB, issueID, tag, actor string) error {
    // BEGIN; DELETE FROM issue_tags WHERE issue_id=? AND tag=?; RowsAffected==0 -> core.ErrNotFound;
    // insertEvent(ctx, tx, issueID, actor, "issue_untagged", map[string]any{"tag": tag}, now);
    // COMMIT
}
```

`AddTag`/`RemoveTag` reuse the existing `insertEvent` helper
(`internal/store/sqlite/issues.go:1316`) exactly as `AddDependency`/
`RemoveDependency` do, and `isSQLiteConstraintError`
(`internal/store/sqlite/issues.go:1887`) for the duplicate-tag case.

`internal/store/sqlite/issues.go` changes:
- `CreateIssue`: insert one `issue_tags` row per `req.Tags` entry inside the
  existing create transaction, after the issue row insert and before the
  `issue_created` event insert; fold `"tags": req.Tags` into that existing
  event's payload map (no separate `issue_tagged` events at create time —
  keeps create atomic, matches how `external_key` is already folded in, and
  avoids event-stream spam for N tags at once).
- Call `populateTags(ctx, db, issues)` right after each existing
  `populateDependencies(ctx, db, issues)` call, in `GetIssue`, `ListIssues`,
  and `ListReadyIssues` (three call sites — R1, R8: existing issues have
  zero tags by default, no backfill needed since the join simply returns no
  rows).

`internal/store/sqlite/store.go`: thin `(*Store)` wrapper methods, matching
`AddDependency`/`RemoveDependency`.

## API (R5)

`internal/api/daemon.go`, routes registered near the dependency/link routes:

```go
mux.HandleFunc("POST /v1/issues/{issue_id}/tags", handleAddTag(st, logger))
mux.HandleFunc("DELETE /v1/issues/{issue_id}/tags", handleRemoveTag(st, logger))
```

**Routing decision**: the remove endpoint takes `tag` as a **query
parameter**, not a path segment, even though the tag value contains `/`
(`namespace/value`). A literal `DELETE /v1/issues/{id}/tags/{tag}` would
need either a Go 1.22 `{tag...}` trailing wildcard plus `url.PathEscape` on
the client (fragile across proxies/curl/other HTTP tooling that don't all
agree on `%2F` handling in a path segment) or double-segment parsing. The
existing `DELETE /v1/issues/{issue_id}/links` endpoint
(`internal/api/daemon.go:110`) already established the query-param-for-DELETE
pattern for an identifier that isn't a clean single path segment
(`artifact`/`relation`/`actor` are all query params there); tags follow the
same precedent for maximum compatibility: `DELETE
/v1/issues/{issue_id}/tags?tag=ns/val&actor=...`.

`internal/api/issues.go`:

- `handleAddTag` (template: `handleAddDependency`, line 605): resolve
  issue, decode `core.AddTagRequest`, require `Tag`/`Actor` non-empty, call
  `st.AddTag`, map `ErrValidationFailed`→400, `ErrNotFound`→404,
  `ErrAlreadyTagged`→409, else 500 via `logger.Error` + `writeError`;
  `w.WriteHeader(http.StatusCreated)` on success.
- `handleRemoveTag` (template: `handleUnlinkArtifact`, line 723): resolve
  issue, `tag := r.URL.Query().Get("tag")` (400 if empty), `actor :=
  r.URL.Query().Get("actor")`, call `st.RemoveTag`, map `ErrNotFound`→404,
  else 500; `w.WriteHeader(http.StatusNoContent)` on success.
- `handleGetIssue`/`handleCreateIssue`: unchanged — `core.Issue`/
  `CreateIssueRequest` serialize `Tags` automatically once the fields exist.

## Client (R6)

`internal/client/client.go`, mirroring `AddDependency`/`UnlinkArtifact`:

```go
func (c *Client) AddTag(ctx context.Context, issueID string, req core.AddTagRequest) error {
    return c.doJSON(ctx, http.MethodPost, "/v1/issues/"+issueID+"/tags", req, nil)
}

func (c *Client) RemoveTag(ctx context.Context, issueID, tag, actor string) error {
    query := url.Values{"tag": {tag}}
    if actor != "" {
        query.Set("actor", actor)
    }
    return c.doJSON(ctx, http.MethodDelete, "/v1/issues/"+issueID+"/tags?"+query.Encode(), nil, nil)
}
```

## CLI (R4, R6)

`cmd/afctl/cmd_issue.go`:

- `runIssueCreate`'s flag loop gains a repeatable, simple-append case (no
  comma-splitting needed — one `--tag` per occurrence):
  ```go
  case "--tag":
      if i+1 < len(args) {
          req.Tags = append(req.Tags, args[i+1])
          i++
      }
  ```
- New dispatcher (template: `runIssueNote`/`runIssueDependency`):
  ```go
  const issueTagUsage = "Usage: afctl issue tag <add|remove|list>"

  func runIssueTag(ctx context.Context, c *client.Client, args []string) error {
      // same shape as runIssueNote: len==0 -> usageErr; help; switch add/remove/list
  }
  ```
  Registered in `runIssue`'s switch as `case "tag": return runIssueTag(ctx, c, args[1:])`.
  - `runIssueTagAdd(issueID, --tag ns/val)` → `c.AddTag`, prints `Tag added: ns/val`.
  - `runIssueTagRemove(issueID, --tag ns/val)` → `c.RemoveTag`, prints `Tag removed: ns/val`.
  - `runIssueTagList(issueID)` → `c.GetIssue(ctx, issueID)`, prints one tag
    per line (`No tags.` when empty) or the JSON array under `--json`.
  Each subcommand follows the afc-76 convention: `hasHelpFlag` checked
  before positional args, `usageErr` on missing/invalid input, actor
  resolved via the existing `resolveActor("")` helper.
- `cmd/afctl/main.go`: `printIssueDetailed` and `printIssueFull` gain a
  `Tags:` line (only when non-empty), placed after `Assignee` — same
  position/style as the existing conditional `External:`/`External Key:`
  lines, using `strings.Join(i.Tags, ", ")` (already imported for
  `Blocks`/`BlockedBy`).

## MCP (R7)

`internal/mcp/server.go`:

- `CoordinatorClient` interface gains:
  ```go
  AddTag(ctx context.Context, issueID string, req core.AddTagRequest) error
  RemoveTag(ctx context.Context, issueID, tag, actor string) error
  ```
  `*client.Client` satisfies this automatically once the client methods
  above exist (structural typing).
- `get_issue` needs no handler change — `core.Issue.Tags` serializes as
  part of the existing response.
- New tools, following the `add_note` case/schema shape exactly:
  ```go
  case "add_tag":
      var args struct {
          IssueID string `json:"issue_id"`
          Tag     string `json:"tag"`
          Actor   string `json:"actor"`
      }
      if err := unmarshalArgs(params.Arguments, &args); err != nil { return nil, err }
      if args.IssueID == "" || args.Tag == "" {
          return nil, fmt.Errorf("issue_id and tag are required")
      }
      actor, err := s.resolveActor(args.Actor, "")
      if err != nil { return nil, err }
      if err := s.client.AddTag(ctx, args.IssueID, core.AddTagRequest{Tag: args.Tag, Actor: actor}); err != nil {
          return nil, err
      }
      return map[string]any{"status": "ok"}, nil
  case "remove_tag":
      // same args shape minus a body wrapper; calls s.client.RemoveTag
  ```
  Schema entries added to `tools()` via `toolDefinition("add_tag", ..., objectSchema([]schemaField{...}))`
  and `toolDefinition("remove_tag", ...)`, matching the `add_note`/`list_notes` entries.

## Compatibility (R8)

Additive only: new table, new optional fields, new endpoints, new CLI
subcommand. No existing route, request shape, CLI flag, or MCP tool
changes behavior. Every existing issue has zero tags until explicitly
tagged — no migration backfill, no behavior change for untagged issues.
