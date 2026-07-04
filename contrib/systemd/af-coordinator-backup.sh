#!/bin/bash
set -euo pipefail

DB_PATH="${AF_COORDINATOR_DB:-$HOME/.local/share/af-coordinator/af-coordinator.db}"
BACKUPDIR="${BACKUPDIR:-$HOME/backups/af-coordinator}"
TIMESTAMP=$(date +"%Y%m%d-%H%M")
BACKUP_FILE="$BACKUPDIR/af-coordinator-$TIMESTAMP.db"

mkdir -p "$BACKUPDIR"

echo "Starting backup of $DB_PATH to $BACKUP_FILE"
sqlite3 "$DB_PATH" "VACUUM INTO '$BACKUP_FILE'"

echo "Running integrity check on backup file..."
CHECK=$(sqlite3 "$BACKUP_FILE" "PRAGMA integrity_check")

if [ "$CHECK" != "ok" ]; then
    echo "ERROR: Integrity check failed on backup file $BACKUP_FILE"
    echo "Output: $CHECK"
    rm -f "$BACKUP_FILE"
    exit 1
fi

echo "Integrity check passed."

echo "Pruning old backups (keeping last 14)..."
ls -t "$BACKUPDIR"/af-coordinator-*.db | tail -n +15 | xargs -r rm -f

echo "Backup complete."
