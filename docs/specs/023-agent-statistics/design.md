# Design

Extend the existing single event scan and matched attempt records. Attribute
outcomes/durations to the normalized claim actor, including claims before the
selected window. Claim counts use claim timestamps; completed counts use end
timestamps, so done/claims can exceed 100% in a window (not a cohort score).
Operator releases with matched attempt IDs contribute a new `operator_released`
outcome to global and agent counts. No-attempt-ID events cannot yield measurable
attempt durations; existing valid-attempt claim counting is retained.

Recommended identity is stable `agent` or `agent@host` in DIBS_ACTOR/holder;
instance/task/PID belongs in the existing claim session_id. Normalization strips
host/instance suffixes, folds explicit engine aliases/PID/task labels for known
codex/agy/claude/opencode families and the known aion worker aliases. Standalone
versions/empty actors become unknown. Issue-role labels (afc/aion/utils/piac/jsb)
become task/<role>, not an inferred engine; other labels remain unchanged.
Raw actors remain in events and in sorted report alias lists.

Optional by_actor and top_issues sections share global scope/window. Top ranks
claims descending, completed duration descending, short ID/UUID ascending; lead
time is creation to latest terminal close at/before until, or observed age at
until. Notes count note_added events in the window. A progress-free release is
an exactly ordered matched plain release with no note or commit/PR evidence
between its claim and release. Flag an issue at three such releases; legacy
pairs remain unclassified. Progress uses causal sequence order for exact attempts,
including clock-skewed note/SCM timestamps. A note without attempt_id belongs
to the active attempt of its issue; explicit attempt IDs must match it. Missing
or non-forward sequence evidence makes the attempt unclassified. Evidence
before since can explain an in-window release, but events after until remain
excluded; timestamp-based counts and duration validation are unchanged.
Handoffs are excluded. No agent ranking is implied.

Top claim counts include raw claim events; tracked_claims distinguishes valid
attempt IDs. Raw release/expiry/operator counts can exceed matched completions.
