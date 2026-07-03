# Review

Status: in progress

## AFC-SDD-0004 — Project, repository, and worktree registration APIs

Shipped.

## AFC-SDD-0005 — Artifact-root and artifact registration APIs

### What shipped

- **Core types** (`internal/core/artifact.go`): `ArtifactRoot`, `Artifact`,
  `CreateArtifactRootRequest`, `CreateArtifactRequest`, validation functions
- **SQLite store** (`internal/store/sqlite/artifacts.go`): `CreateArtifactRoot`,
  `ListArtifactRoots`, `GetArtifactRoot`, `CreateArtifact`, `ListArtifacts`,
  `GetArtifact`
- **API handlers** (`internal/api/artifacts.go`): `POST /v1/artifact-roots`,
  `GET /v1/artifact-roots`, `POST /v1/artifacts`, `GET /v1/artifacts`
- **Client methods** (`internal/client/client.go`): `CreateArtifactRoot`,
  `ListArtifactRoots`, `CreateArtifact`, `ListArtifacts`
- **CLI commands** (`cmd/afctl/main.go`): `afctl artifact-root add|list`,
  `afctl artifact register|list`

### What was verified

- `go build ./...` — compiles clean
- `go vet ./...` — no issues
- `go test ./...` — passes (no test files yet; existing suites unaffected)

### Open

- No dedicated tests for the new handlers or store functions yet
- The `ValidateArtifactKind` helper is defined but not yet enforced on write;
  validation only checks required fields
- CLI help text updated in the usage block

## AFC-SDD-0006 — Issue create/get/list/ready APIs

### What shipped

- **Core types** (`internal/core/issue.go`): `Issue`, `IssueLease`,
  `CreateIssueRequest`, `IssueListParams`, `ValidateCreateIssue`
- **SQLite store** (`internal/store/sqlite/issues.go`): `CreateIssue` (with
  short-id allocation via `next_issue_seq`), `GetIssue` (by id or short_id,
  with active lease lookup), `ListIssues` (dynamic filters), `ListReadyIssues`
  (excludes terminal statuses and leased issues)
- **API handlers** (`internal/api/issues.go`): `POST /v1/issues`,
  `GET /v1/issues/{issue_id}`, `GET /v1/issues`, `GET /v1/issues/ready`
- **Client methods** (`internal/client/client.go`): `CreateIssue`, `GetIssue`,
  `ListIssues`, `ListReadyIssues`
- **CLI commands** (`cmd/afctl/main.go`): `afctl issue create|get|list|ready`

### What was verified

- `go build ./...` — compiles clean
- `go vet ./...` — no issues
- Run `gofmt -w .` — all formatting correct

### Open

- Dependency filtering in `ListReadyIssues` (from `dependencies` table) is
  deferred to SDD-0008
- No dedicated tests for the new handlers or store functions yet

## AFC-SDD-0007 — Lease claim/release/heartbeat flow

### What shipped

- **Error codes** (`internal/core/errors.go`): `ErrLeaseHeld` (409) and
  `ErrLeaseExpired` (410)
- **Core types** (`internal/core/issue.go`): `ClaimRequest`, `ClaimResponse`,
  `HeartbeatRequest`, `ReleaseRequest`
- **SQLite store** (`internal/store/sqlite/issues.go`): `ClaimIssue` (acquires
  lease, moves open → in_progress), `HeartbeatLease` (extends TTL on existing
  lease), `ReleaseLease` (deletes lease, returns in_progress → open unless
  blocked)
- **API handlers** (`internal/api/issues.go`): `POST /v1/issues/{issue_id}/claim`,
  `POST /v1/issues/{issue_id}/heartbeat`, `POST /v1/issues/{issue_id}/release`
- **Client methods** (`internal/client/client.go`): `ClaimIssue`,
  `HeartbeatLease`, `ReleaseLease`
- **CLI commands** (`cmd/afctl/main.go`): `afctl issue claim|heartbeat|release`

### What was verified

- `go build ./...` — compiles clean
- `go vet ./...` — no issues
- `gofmt -w .` — all formatting correct

### Open

- No dedicated tests for the new handlers or store functions yet

## AFC-SDD-0008 — Issue update/close and dependency management

### What shipped

- **Core types** (`internal/core/issue.go`): `UpdateIssueRequest`,
  `CloseIssueRequest`, `AddDependencyRequest`, `RemoveDependencyRequest`,
  `ValidateStatusTransition`
- **Error codes** (`internal/core/errors.go`): `ErrDependencyCycle`
- **SQLite store** (`internal/store/sqlite/issues.go`): `UpdateIssue` (dynamic
  SET clause, version/lease checks, event logging), `CloseIssue` (resolution
  check, lease cleanup, event logging), `AddDependency` (BFS cycle detection
  for `blocks` kind), `RemoveDependency`, `wouldCreateCycle`
- **API handlers** (`internal/api/issues.go`): `PATCH /v1/issues/{issue_id}`,
  `POST /v1/issues/{issue_id}/close`, `POST /v1/issues/{issue_id}/dependencies`,
  `DELETE /v1/issues/{issue_id}/dependencies/{depends_on}`
- **Client methods** (`internal/client/client.go`): `UpdateIssue`, `CloseIssue`,
  `AddDependency`, `RemoveDependency`
- **CLI commands** (`cmd/afctl/main.go`): `afctl issue update`, `afctl issue close`,
  `afctl issue dependency add`, `afctl issue dependency remove`

### What was verified

- `go build ./...` — compiles clean
- `go vet ./...` — no issues
- `gofmt -w .` — all formatting correct

### Open

- No dedicated tests for the new handlers or store functions yet
- `ListReadyIssues` dependency filtering from the `dependencies` table
  (deferred from SDD-0006) is not yet implemented

Use this file to capture:

- what shipped
- what was verified
- which tasks remain open
- whether implementation still matches requirements and design
