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

Logs are written to:

```text
~/Library/Logs/af-coordinatord.log
~/Library/Logs/af-coordinatord.err.log
```

The launchd path currently covers the daemon only. Automated backup jobs are
still Linux/systemd-only.

## Manual daemon start

```bash
af-coordinatord
```

Default socket: `~/.local/state/af-coordinator/af-coordinator.sock`
Default database: `~/.local/share/af-coordinator/af-coordinator.db`

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

### Automatic backup (systemd timer)

An automated backup script and systemd timer are provided in `contrib/systemd/`.
To install and enable them:

```bash
make install-backup
```

This will run a `VACUUM INTO` daily at 03:17, check the integrity of the backup, and keep the last 14 backups in `~/backups/af-coordinator`.

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

# List issues
afctl issue list --project test

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
