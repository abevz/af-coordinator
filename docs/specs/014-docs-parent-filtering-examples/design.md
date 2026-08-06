# 014 Docs Parent Filtering Examples Design

## Overview

Update existing documentation files (`docs/workflows-v1.md` and `docs/operations.md`) with explicit examples for listing and filtering child tasks linked to a parent epic or parent issue.

## Document Changes

### 1. `docs/workflows-v1.md`
In the **The epic flow** section, add query examples after step 4:
- Filtering CLI list: `afctl issue list | grep "parent:<parent-id>"`
- Programmatic JSON filtering:
  ```bash
  afctl issue list --json | jq -r '.[] | select(.dependencies[]? | .kind == "parent" and (.depends_on_short_id == "<parent-id>" or .depends_on_id == "<parent-id>")) | "\(.short_id)\t[\(.status)]\t\(.title)"'
  ```

### 2. `docs/operations.md`
Under the table formatting description in `Issue listing and filtering`, add explicit examples for filtering by `parent` dependency relationship in both text mode and JSON mode.
