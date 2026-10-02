# Changelog

## Unreleased

### Breaking changes — afc-142

- Installs expose only `dibs`, `dibsd`, and `dibs-mcp`. The `afctl`,
  `af-coordinatord`, and `afc-mcp` aliases and their deprecation warnings are
  removed. Upgrade installers remove only their own compatibility symlinks;
  independent executables or symlinks to other targets remain untouched.
- Replace `AF_COORDINATOR_DB`, `AF_COORDINATOR_SOCKET`,
  `AF_COORDINATOR_LOG_LEVEL`, `AF_COORDINATOR_ACTOR`, `AF_OPERATOR_TOKEN`,
  `AF_LEASE_TOKEN`, and installer `AF_COORDINATOR_REPO` with the corresponding
  canonical `DIBS_DB`, `DIBS_SOCKET`, `DIBS_LOG_LEVEL`, `DIBS_ACTOR`,
  `DIBS_OPERATOR_TOKEN`, `DIBS_LEASE_TOKEN` (or its private token file), and
  `DIBS_REPO`. Legacy environment fallback is removed.
- `issue run` exports only `DIBS_LEASE_TOKEN`, `DIBS_LEASE_GENERATION`,
  `DIBS_ATTEMPT_ID`, `DIBS_ISSUE_ID`, and `DIBS_EXPECTED_VERSION` lifecycle
  metadata. Update child scripts before upgrading.
- The old daemon systemd unit template is retired. Existing legacy data and
  socket paths remain supported to preserve database identity. Backup unit
  names remain unchanged. No installer switches or restarts the live service.

The owner approved removal before v0.1.0 final on 2026-10-01, superseding the
original plan to remove aliases after v0.1.0. Known Aion Forge source already
uses canonical DIBS interfaces; the companion daily-check change migrates its
runner and create-form command to `dibs` and its actor to `DIBS_ACTOR`.
