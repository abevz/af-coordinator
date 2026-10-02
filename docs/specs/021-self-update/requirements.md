# Requirements

- R1: Explicit release check compares semantic versions, selects stable/prerelease channels, prints the intervening CHANGELOG with breaking changes first, and distinguishes exit 0 (current), 10 (available), 1 (failed).
- R2: Explicit upgrade downloads the existing dibs_OS_ARCH.tar.gz and checksums.txt assets, verifies SHA-256, safely publishes all three canonical binaries together, retains previous binaries and supports rollback. Breaking changes require confirmation or --yes; restart independently requires confirmation or --restart.
- R3: Passive notices read only cache, follow normal output, require stderr TTY, suppress JSON/hooks/MCP/issue-run children, appear in watch footer, deduplicate per day/version, and honor DIBS_NO_UPDATE_NOTIFIER=1. Refresh at most once per 24 hours in the daemon; no auto-upgrade.
- R4: Doctor reports cached updates and CLI/daemon mismatch; network failure does not alter ordinary command latency or output.
- R5: Existing bindir, artifact/checksum naming, license location, database and socket identity remain. Updater never opens coordinator storage. Docs and Unreleased changelog cover operation and opt-out.
- R6: Fake HTTP release/download tests cover versions/channels, checksum and interruption, atomic publication and rollback, notice suppression/dedup/offline latency. Full checks, independent review and CI required before merge. Live verification is read-only --check/notifier only; owner release prompt separately authorizes install script dogfood.
