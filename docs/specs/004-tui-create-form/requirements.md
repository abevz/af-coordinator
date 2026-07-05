# Requirements

## Context
In the previous tracking system (`bd`), operators heavily used `bd create-form` to quickly scaffold tasks interactively rather than memorizing CLI flags or writing shell scripts. `af-coordinator` v1 initially excluded TUI elements, but the lack of an interactive flow has proven disruptive to human workflows.

## Goals
- Provide a guided, multi-step TUI wizard for creating issues.
- Expose all essential fields natively supported by `af-coordinator`.
- Support navigating screens with standard keys (up, down, `/` for filter, `shift+tab` for back, `enter` for select).

## Non-Goals
- Full TUI for navigating the backlog or dashboard (this is scoped only to *creation*).
- Introducing new fields to the daemon schema (e.g., labels) — the form will strictly use existing system fields.

## Use Cases
- A user wants to create a repository-scoped issue, assign it, set its priority, and immediately link it to an existing specification artifact and dependencies.
