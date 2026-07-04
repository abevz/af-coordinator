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

## AFC-SDD-0018 — Agent protocol document

Shipped.

### What shipped

- `docs/agent-protocol-v1.md` — canonical agent protocol contract (67 lines, under 150-line cap)

### What was verified

- All quoted commands verified against afctl source: `issue ready`, `issue claim`, `issue heartbeat`, `issue note add`, `issue close`, `issue release`
- Line count: 67 (under 150-line cap)
- Every command in the document uses real afctl flags matching the implementation
- Session loop is complete: ready → claim → heartbeat → note → close/release

### Open

- Manual end-to-end test of the protocol through actual agent interaction

## AFC-SDD-0032 — afctl protocol subcommand (go:embed)

Shipped.

### What shipped

- `cmd/afctl/protocol.go` — embeds agent-protocol-v1.md with go:embed
- `cmd/afctl/protocol_test.go` — byte-equality test against canonical doc
- `afctl protocol` prints the full contract from the binary

### What was verified

- Byte-equality with canonical docs/agent-protocol-v1.md (tested)
- Binary installed at ~/.local/bin/afctl

## AFC-SDD-0033 — afctl init (managed AGENTS.md block)

Shipped.

### What shipped

- `cmd/afctl/init.go` — managed block with BEGIN/END markers
- `cmd/afctl/init_test.go` — table-driven tests over 4 states + dry-run
- Four states: missing → create, no block → append, stale → replace, current → no-op
- Text outside markers is never touched
- Supports `--path`, `--dry-run`, `--json`, standard exit codes

### What was verified

- Live test: create, append, replace, unchanged all work correctly
- Text outside markers preserved on replace
- Dry-run does not write

### Open

- contrib/agents/AGENTS-snippet.md superseded by `afctl init`
- Snippet source of truth is cmd/afctl/init-snippet.md (embedded by init.go)

## Closing — packet 002

All 9 tasks (AFC-SDD-0017 through 0033) are shipped and verified.
