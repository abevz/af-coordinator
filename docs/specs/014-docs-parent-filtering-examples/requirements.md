# 014 Docs Parent Filtering Examples Requirements

## User story

As a developer or operator managing epics and parent-child task hierarchies in `af-coordinator`, I want explicit documentation and examples in `docs/workflows-v1.md` and `docs/operations.md` showing how to list and filter all child tasks associated with a given parent issue ID (e.g., `aion-500` or `utils-10`), using both human-readable text CLI output (`grep`) and programmatic JSON output (`jq`), so that I can easily track child task completion without guessing CLI flags or jq filters.

## Requirements

### R-01: Workflow guidance for querying child tasks
`docs/workflows-v1.md` MUST include concrete examples of finding all child tasks for a parent issue/epic using:
1. Standard CLI output filtered by `grep "parent:<short_id>"`.
2. Programmatic JSON output filtered by `jq` on `dependencies` with `kind == "parent"`.

### R-02: Operations reference update
`docs/operations.md` MUST clarify how non-blocking `parent` dependencies are represented in the `DEPS` column and show filtering examples for CLI table output and `--json` output.
