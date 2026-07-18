# 010 Dependency Direction Ergonomics

Make `blocks` dependencies unambiguous to author and to read.

- Directional CLI flags `--blocked-by` / `--blocks` with plain-language
  confirmation, keeping `--depends-on/--kind` for compatibility.
- Detail view shows blocking only as `Blocked By:` / `Blocks:`, never an
  ambiguous raw `blocks <target>` line.
- Reverse `Blocks` list so a blocker sees what it blocks; status-only blocks
  stay a distinct, labelled state.

See `requirements.md`, `design.md`, `tasks.md`.
