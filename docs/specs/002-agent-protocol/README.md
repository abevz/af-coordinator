# 002 — Agent protocol

One canonical working contract that lets any coding agent (Claude Code,
Codex, CodeWhale, future ones) coordinate through `af-coordinatord`, plus
the machinery that makes following the contract cheap and skipping it
loud.

Deliverables:

- `docs/agent-protocol-v1.md` — the canonical contract
- `afctl` `--json` output and typed exit codes (machine-readable CLI)
- `contrib/hooks/` — ready-made enforcement hook snippets
- `contrib/agents/` — per-repo adapter snippet template

Packet flow:

```text
requirements.md -> design.md -> tasks.md -> implementation -> review.md
```

Prerequisite: 001-foundation closure punch list (AFC-SDD-0013..0016) —
done.
