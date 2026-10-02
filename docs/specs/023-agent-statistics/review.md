# Review

## Current resumed run — 2026-10-02 (clock-skew fix)

Explicit owner authorization resumed the prior stop; its failures do not count
against this run. HEAD and branch remain unchanged; previously written changes
were preserved, with a fresh claim before edits. Main thread is the sole writer.

Evidence: /home/abevz/t175-evidence-20261002 (separate from both earlier runs).
Only this run's commands use TMPDIR=GOTMPDIR=/home/abevz/t175, a private short
directory on the home filesystem (310 GB free at preparation). A representative
long firstuse test socket path was 93 bytes and successfully bound before tests.
No global environment changes or deletion of unrelated files.

TestAgentProgressUsesSequenceDespiteClockSkew reproduces production AddNote's
payload without attempt_id, with positive sequences and a note timestamp before
its claim. Before the fix, both whole-window and since-filter cases failed with
no-progress=3, unclassified=0, flag=true (regression-red.log). Both pass after
the fix (regression-green.log), with unchanged note window counts and durations.

Attempts retain claim sequence. Progress follows exact causal order in the
active attempt of the same issue; an explicit event attempt_id must match it.
Legacy/missing/non-forward sequence evidence remains conservative. Events after
until are still excluded, evidence before since can explain a release, and
negative-duration end events remain unmeasurable. No migrations or synthetic
markers were added. TestAgentProgressEvidenceBoundaries covers note and both
SCM fields under clock skew, other attempt/issue, until, missing sequence and
legacy behavior.

Formatting, git diff --check, go build ./..., go vet ./... and
/usr/bin/golangci-lint run --disable errcheck,staticcheck passed (0 issues).
The two disabled analyzers match repository CI; no new lint exclusions.
Focused tests passed: go test ./internal/report ./internal/api ./internal/client
./cmd/dibs (focused.log). Full go test -race ./... passed with exit 0 (race.log)
using the prepared short temporary paths. This run had no final-check failures.

Rebuilt all three binaries and repeated scratch installation under isolated
HOME/DB/socket. scratch-build.log and scratch.log passed, with standalone
issue-create JSON parsing, normalized codex/claude actors, all actor claim,
completion and outcome totals reconciled with global values, and no-progress
flag in JSON/human tables. Scratch daemon stopped in finally. Synthetic reports
and private tokens remain outside Git. No live DB access, daemon switch or user
PATH installation, and no commit/push/PR/merge.

Whole accumulated diff received renewed independent read-only APPROVE: reporting,
API/client/CLI, tests, public docs and SDD were reviewed. The reviewer authored
none of these changes, ran no tests and made no edits, and confirmed the prior
clock-skew finding resolved with no remaining findings. Subsequent changes are limited to recording completed verification and the
review decision in tasks.md/review.md; production code and tests are unchanged.

All requested implementation verification is complete; no known implementation
blockers remain. Publication is still pending and outside this authorization:
commit, push, PR and CI/merge were not performed.

## Historical stopped run — 2026-10-02 (first resume)

Worktree: afc-175-agent-stats; branch: feat/afc-175-agent-stats;
base/HEAD: 5ce806f5c4d6e93e47d55934cebb29263185131a. All feature changes
remain uncommitted. Existing changes were preserved; no commit, push, PR,
merge, live install, live daemon restart or direct live database access.

Migration 0005 intentionally omits event_ordering_enabled for fresh databases:
cutoff 0 means exact ordering from the first positive sequence. Attempt
classification now requires a positive sequence and either cutoff 0 or sequence
at/after the legacy cutoff. Missing sequences and pre-cutoff legacy events
remain unclassified. No schema or marker creation changes.

TestStatsFreshDatabaseClaimWithoutProgress uses real embedded migrations and
store create/claim/release operations on an empty in-memory database. It asserts
no ordering marker and positive event sequences. Before the fix it failed with
no-progress=0, unclassified=3, flag=false; this isolates the classification bug.
TestAgentMissingSequencesRemainUnclassified covers missing sequence evidence;
the existing legacy marker/window tests retain their conservative behavior.

New evidence is separate: /tmp/dibs-afc-175-resume-20261002.
regression-red.log records the expected failure. Focused tests passed: go test ./internal/report ./internal/api ./internal/client
./cmd/dibs (focused.log). Formatting, git diff --check, go build ./...,
go vet ./... and golangci-lint run --disable errcheck,staticcheck passed
(fmt.log, diff.log, build.log, vet.log, lint.log; linter 0 issues).
The first go test -race ./... failed at CLI linking with disk quota exceeded
(race.log); the other packages passed. /tmp was 88% full, whereas the home
filesystem had 310 GB available. One retry uses a task-private GOTMPDIR under
~/.cache/dibs-afc-175-gotmp; no unrelated temporary files were removed.
Retry exited 1 (race-retry.log): TestStopDaemonDoesNotMistakeUnhealthySocketForStopped
and TestEnsureDaemonRefusesUnverifiableReachableSocket failed with Unix socket
bind: invalid argument because the task GOTMPDIR prefix made the socket paths
too long. CLI race tests passed on the retry. This is the second consecutive
failure of the full-suite step in this run: stopped without a third attempt.
Do not report the full suite as green. On a separately authorized resume, use
a short private GOTMPDIR on the home filesystem (for example ~/g175),
then rerun the final suite after addressing review findings.

Rebuilt dibs, dibsd and dibs-mcp were installed only under a fresh scratch HOME,
with isolated DB/socket and scrubbed DIBS environment. scratch.log records PASS:
three codex instance claims normalize to codex, claude-code normalizes to claude,
actor claim/completion totals equal global totals, and three plain releases
without progress set claim_without_progress in JSON and human tables.
issue-create is parsed as a standalone JSON object. Scratch daemon stopped
in the harness finally block. scratch-report.json and scratch-stats.txt contain
synthetic data only and stay outside Git.

Independent read-only review of the whole accumulated diff completed.
Approval is withheld for the correctness finding below. The reviewer authored
none of the feature changes and ran no tests or edits; final documentation
accurately records incomplete validation and pending publication.
A verified finding remains in internal/report/breakdown.go: progress evidence
is gated by wall-clock timestamps even for exactly sequence-ordered events.
A later-sequence note/SCM event with an earlier timestamp can be ignored and
produce a false no-progress flag. No fix was attempted after the stop rule.
Resume with a production-contract clock-skew regression, then use causal event
order to recognize progress conservatively and obtain renewed final review.
Publication remains outside this request.

## Historical evidence — prior run, not rerun here

/tmp/dibs-afc-175 holds the prior focused tests, full race suite, build, vet,
fmt/diff and system linter (0 issues), and private historical export verification.
That report surfaced afc-96 (177 claims) and aion-551 (160 claims, 964 notes),
with actor/global reconciliation. Private runtime data remains outside Git.

The prior scratch attempt stopped after two failures: an incorrect issue-create
JSON envelope assumption, then the cutoff-0 classification defect. This resumed
run uses the corrected standalone-object harness and fresh scratch state.

## Historical stop handoff (first resume)

Root remains the only writer. No feature changes were discarded. The fresh-DB
cutoff fix and regressions pass focused checks, but the complete feature is
not approved for publication. Remaining work: address all independent-review
findings, rerun focused/scratch checks appropriate to those changes, get a green
full suite with a short temporary path, and renew independent review of the final
revision. Then commit/PR/CI would require separate authorization.
