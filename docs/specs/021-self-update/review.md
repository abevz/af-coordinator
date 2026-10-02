# Review

Implementation matches afc-178 requirements: explicit semantic release checks,
checksum-verified publication through one shared binary pointer, retained
predecessor metadata for offline rollback, separately authorized breaking
changes/restart, daemon-only bounded daily refresh, cached terminal notices and
doctor integration. Coordinator storage is never opened by the updater.

Verified locally: focused fake-server tests; `go test -race ./...`;
`go build ./...`; `go vet ./...`; `golangci-lint run --disable errcheck,staticcheck`;
Darwin arm64 cross-compilation of updater, doctor and both entrypoints; command
help and diff/format checks. Fake releases cover newer/same/older, prerelease
channels, breaking-first changelog, checksum and interrupted download, atomic
publication, failed adoption rename with complete staged version metadata,
stale concurrent updater refusal, failed subsequent upgrade preserving rollback, restart consent
and failure, Homebrew Cellar refusal before mutation, offline rollback, notifier suppression/dedup/concurrency and
cached offline latency. Existing doctor version-mismatch tests remain green.

Independent final-content review approved the complete implementation after
correcting watch --once stdout contamination, adoption metadata ordering, and
Homebrew ownership detection. PR CI remains pending. The existing release
verification matrix now runs updater tests natively on Linux/macOS amd64/arm64. No live updater or
restart from the new command has been run. Real release --check/notifier
verification follows publication of rc.5 under the owner release procedure.
