# Claude Code lease check hook

1. Copy `settings-snippet.json` into your `.claude/settings.json`:
   ```json
   {
     "preToolUse": {
       "matcher": { "type": "any", "matchers": [
         {"type": "tool", "name": "Edit"},
         {"type": "tool", "name": "Write"},
         {"type": "tool", "name": "Bash"}
       ]},
       "command": "${PROJECT_DIR}/contrib/hooks/claude-code/check-lease.sh",
       "mode": "warn"
     }
   }
   ```
2. Export `AF_COORDINATOR_ACTOR=<your-agent-name>`.
3. Set `AF_HOOK_MODE=block` to block instead of warn.
