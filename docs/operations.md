# Operations

## Building

```bash
go build -o ~/.local/bin/af-coordinatord ./cmd/af-coordinatord
go build -o ~/.local/bin/afctl ./cmd/afctl
```

Or use the Makefile:

```bash
make build
```

## Systemd user service

### Install

```bash
make install-service
sh contrib/install/systemctl-user.sh enable --now af-coordinatord
```

### Check status

```bash
sh contrib/install/systemctl-user.sh status af-coordinatord
```

### View logs

```bash
journalctl --user -u af-coordinatord -f
```

### Start/stop/restart

```bash
sh contrib/install/systemctl-user.sh start af-coordinatord
sh contrib/install/systemctl-user.sh stop af-coordinatord
make restart-service
```

`contrib/install/systemctl-user.sh` fills `XDG_RUNTIME_DIR` and
`DBUS_SESSION_BUS_ADDRESS` from `/run/user/$(id -u)/bus` when they are missing,
which keeps service targets working from non-interactive agent environments.

## macOS LaunchAgent

Build the binaries and install the daemon as a user LaunchAgent:

```bash
make install-launchd
launchctl print gui/$(id -u)/com.abevz.af-coordinatord
```

The Makefile renders `contrib/launchd/com.abevz.af-coordinatord.plist.in` into
`~/Library/LaunchAgents/com.abevz.af-coordinatord.plist`, then bootstraps and
starts the service.

To uninstall:

```bash
make uninstall-launchd
```

Daemon logs are written to:

```text
~/Library/Logs/af-coordinatord.log
~/Library/Logs/af-coordinatord.err.log
```

## Manual daemon start

```bash
af-coordinatord
```

Default socket: `~/.local/state/af-coordinator/af-coordinator.sock`
Default database: `~/.local/share/af-coordinator/af-coordinator.db`

## Execution statistics

The daemon derives a local read-only report from its coordinator records; it
does not need Prometheus, a rollup database, or network access:

```bash
afctl stats --project afc --since 7d
afctl --json stats --project afc --repo af-coordinator --since 24h
```

Use RFC 3339 or a positive Go duration for `--since`; `--until` accepts RFC
3339 and defaults to now. JSON includes the report version, window,
denominators, percentile sample sizes, and the legacy event-ordering cutoff.

## Interacting via curl

Since the daemon listens on a Unix socket, use `curl --unix-socket`:

```bash
# Health check
curl --unix-socket ~/.local/state/af-coordinator/af-coordinator.sock http://localhost/v1/health

# Create a project
curl --unix-socket ~/.local/state/af-coordinator/af-coordinator.sock \
  -X POST http://localhost/v1/projects \
  -H 'Content-Type: application/json' \
  -d '{"name":"Test","key":"test"}'
```

## Backup

The daemon uses SQLite in WAL mode. Online backup uses `VACUUM INTO`:

### Manual backup

```bash
sqlite3 ~/.local/share/af-coordinator/af-coordinator.db \
  "VACUUM INTO '/path/to/backup/af-coordinator-$(date +%Y%m%d).db'"
```

This creates a consistent, compacted copy of the database while the daemon is running.

### Automatic backup

`make install-backup` installs the native scheduler for the current OS:

- Linux: systemd user timer.
- macOS: launchd LaunchAgent.

The job runs `VACUUM INTO` daily at 03:17, checks the integrity of the backup,
and keeps the last 14 backups in `~/backups/af-coordinator`.

#### Linux systemd timer

An automated backup script and systemd timer are provided in `contrib/systemd/`.
To install and enable them:

```bash
make install-backup
sh contrib/install/systemctl-user.sh enable --now af-coordinator-backup.timer
```

Logs are available with:

```bash
journalctl --user -u af-coordinator-backup.service
```

#### macOS launchd backup

Install the backup LaunchAgent:

```bash
make install-backup
launchctl print gui/$(id -u)/com.abevz.af-coordinator-backup
```

Run a backup immediately when needed:

```bash
launchctl kickstart -k gui/$(id -u)/com.abevz.af-coordinator-backup
```

Logs are written to:

```text
~/Library/Logs/af-coordinator-backup.log
~/Library/Logs/af-coordinator-backup.err.log
```

To uninstall:

```bash
make uninstall-backup
```

### Restore

1. Stop the daemon:
   ```bash
   sh contrib/install/systemctl-user.sh stop af-coordinatord
   ```
2. Replace the database:
   ```bash
   cp /path/to/backup/af-coordinator-20260703.db ~/.local/share/af-coordinator/af-coordinator.db
   ```
3. Start the daemon:
   ```bash
   sh contrib/install/systemctl-user.sh start af-coordinatord
   ```

## CLI usage

```bash
# List projects
afctl project list

# Create an issue
afctl issue create --project test --scope-kind project --title "My issue"

# List issues; project, type, and status accept CSV values
afctl ls --project afc --type epic,chore --status open,in_progress
afctl ls --project afc,aion --type epic,chore --status open,in_progress

# Show the complete filter contract without contacting the daemon
afctl ls --help

# Claim and work on an issue
afctl issue claim <issue-id> --holder my-agent --ttl 3600
afctl issue heartbeat <issue-id> --lease-token <token> --ttl 3600
afctl issue release <issue-id> --lease-token <token>

# Ready view
afctl issue ready --project test

# Notes
afctl issue note add <issue-id> --author me --body "Working on this"
afctl issue note list <issue-id>
```

## Agent guidance sync

`afctl protocol` is the canonical detailed agent workflow. `afctl init`
updates only the managed coordinator block in one target `AGENTS.md` (the
current directory by default, or `--path`), preserving all surrounding
repository instructions. It is not a global fan-out command.

After a protocol-summary template update, check each registered checkout first:

```bash
afctl init --dry-run
afctl init
```

The generated block points agents back to `afctl protocol` and the canonical
`docs/agent-protocol-v1.md`; it intentionally does not duplicate the full
workflow in every repository.

## Data locations

| Resource | Path |
|----------|------|
| Database | `~/.local/share/af-coordinator/af-coordinator.db` |
| Socket | `~/.local/state/af-coordinator/af-coordinator.sock` |
| Logs | `journalctl --user -u af-coordinatord` |

## Configuration

Environment variables override defaults:

- `AF_COORDINATOR_DB` — database path
- `AF_COORDINATOR_SOCKET` — socket path
