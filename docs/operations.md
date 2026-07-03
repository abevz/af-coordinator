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
cp contrib/systemd/af-coordinatord.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now af-coordinatord
```

### Check status

```bash
systemctl --user status af-coordinatord
```

### View logs

```bash
journalctl --user -u af-coordinatord -f
```

### Start/stop/restart

```bash
systemctl --user start af-coordinatord
systemctl --user stop af-coordinatord
systemctl --user restart af-coordinatord
```

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

### Automatic backup (cron)

Add to crontab (`crontab -e`):

```cron
0 3 * * * sqlite3 ~/.local/share/af-coordinator/af-coordinator.db "VACUUM INTO '/path/to/backups/af-coordinator-$(date +\%Y\%m\%d).db'"
```

### Restore

1. Stop the daemon:
   ```bash
   systemctl --user stop af-coordinatord
   ```
2. Replace the database:
   ```bash
   cp /path/to/backup/af-coordinator-20260703.db ~/.local/share/af-coordinator/af-coordinator.db
   ```
3. Start the daemon:
   ```bash
   systemctl --user start af-coordinatord
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

- `AF_COORDINATOR_DB_PATH` — database path
- `AF_COORDINATOR_SOCKET_PATH` — socket path
