# 011 Operator Ergonomics Requirements

This retrospective file records the accepted and rejected outcomes of the
original packet; it does not create new implementation scope.

| Requirement | Disposition |
| --- | --- |
| One-command operator close with structured metadata and note | delivered by `afc-67` |
| Optional/latest expected-version resolution for standalone operator/update commands | delivered by `afc-68` |
| Status-flap cooldown or operator lock | rejected/cancelled as an over-broad solution (`afc-69`) |
| Bulk operator mutations | carried forward as rewritten `afc-70`; excluded from packet completion |

All delivered behavior must retain optimistic concurrency and explicit operator
authorization. Later bulk behavior must build on packet 015 idempotency rather
than bypass it.
