# Review

Status: not started

Use this file to capture:

- what shipped
- what was verified
- which tasks remain open
- whether implementation still matches requirements and design

## AFC-SDD-0017 — afctl --json output and typed exit codes

Shipped.

### What shipped

- `internal/client/client.go`: `ClientError` type carries structured API error codes
- `cmd/afctl/main.go`: global `--json` flag, fail() error handler with typed exit codes
  (0 success, 1 hard failure, 2 version_conflict, 3 lease_held, 4 lease_expired,
  5 not_found, 6 dependency_cycle), JSON success output on stdout for all commands,
  JSON error envelope on stderr, actor resolution (--actor flag > AF_COORDINATOR_ACTOR env),
  updated usage text
- `cmd/afctl/main_test.go`: table-driven tests for exit-code mapping

### What was verified

- `go build ./...` — passes
- `go vet ./...` — passes
- `go test ./...` — all tests pass
- `--json` flag works with all afctl commands
- Actor identity resolves from --actor flag and AF_COORDINATOR_ACTOR env

### Open

- `gofmt` formatting not checked; should run before merge
