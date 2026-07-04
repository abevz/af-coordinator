# Codex lease check hook

1. Copy or symlink `check-lease.sh` (already done).
2. Add to your Codex project config:
   ```json
   "hooks": {
     "preToolUse": {
       "tools": ["Edit", "Write", "Bash"],
       "command": "${workspaceFolder}/contrib/hooks/codex/check-lease.sh",
       "mode": "warn"
     }
   }
   ```
3. Export `AF_COORDINATOR_ACTOR=<your-agent-name>` (unique per concurrent instance, e.g. session-PID suffix: `codex-$$`).
4. Set `AF_HOOK_MODE=block` to block instead of warn.
