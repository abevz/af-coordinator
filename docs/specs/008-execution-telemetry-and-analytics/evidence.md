# Evidence Baseline

Snapshot time: 2026-07-13. Data window: 2026-07-04 through 2026-07-13 UTC.

The baseline was computed through `afctl export jsonl`, `afctl issue ready`,
and read-only source inspection. The coordinator database was not opened
directly. Note bodies and lease tokens were not exported into this document.

## Fleet Snapshot

| Metric | All projects | Aion Forge |
|--------|-------------:|-----------:|
| Projects | 8 | 1 |
| Issues | 163 | 70 |
| Done | 130 | 51 |
| Cancelled | 3 | 1 |
| Deferred | 5 | 5 |
| Open | 25 | 13 |
| Issues with `closed_at` | 133 | 52 |
| Notes | 282 | 190 |
| Claims | 159 | 71 |
| Releases | 16 | 14 |

The export contained 1,086 events, 175 current dependency records, 81 spec
references, and 76 registered artifacts. Aion Forge had five ready issues at
the snapshot.

## Aion Forge Flow

- 42 of 52 terminal closes occurred from 2026-07-11 through 2026-07-13.
- Among 37 closed issues with exactly one claim, median claim-to-close time was
  18 minutes 30 seconds; p90 was 41 minutes 8 seconds.
- All 70 Aion Forge issues had an `implements` link to a spec artifact.
- All 52 terminal Aion Forge issues had at least one note.
- 44 of 52 latest close events carried branch, PR URL, and commit SHA metadata.
- Fourteen releases were recorded; twelve had a same-actor HANDOFF note in the
  preceding fifteen minutes.
- Nineteen Aion Forge claims repeated an earlier claim. Fourteen followed an
  explicit release. Five same-actor reclaims had no intervening release and are
  consistent with lazy lease expiry, but the current event model cannot prove
  the expiry reason.

Created-to-closed lead time is not a direct productivity measure in this data:
many roadmap leaves were batch-created well before selection, and task sizes
vary. Attempt-level duration is the more useful execution-flow signal once its
end reasons are explicit.

## Audit-Order Finding

The 1,086-event export contained:

- 234 timestamps shared by multiple global events, covering 758 events;
- 211 same-issue/same-second groups, covering 494 events;
- 126 same-second groups containing both `note_added` and `issue_closed`.

The current export orders tied events by random UUID. In 63 of the 126 atomic
note-plus-close groups, exported order shows close before note even though the
store inserts note before close. Counts remain usable, but tied transition
order is not reliable evidence.

## Close-Contract Finding

Two issues had more than one close event. One was an explicit reopen path. The
other, `utils-16`, moved from cancelled to done through a second close without
an intervening reopen.

Source inspection explains the latter: ordinary close compares a token only
when an active lease exists and does not validate the current terminal status.
The first close removes the lease, so a later close can pass without proving
ownership. The public API nevertheless documents lease token plus expected
version as required for close.

## Reproduction Starting Points

```bash
afctl health
afctl doctor
afctl export jsonl | jq -r '.type' | sort | uniq -c
afctl issue ready --project aion --json | jq 'length'
afctl issue get utils-16 --full
```

The detailed aggregate queries used during planning should become tested report
logic under `afc-53`, not a permanent collection of shell pipelines.
