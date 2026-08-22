# get_task and /v0/state serve lease verdicts at last-refresh time, not read time

`tuh-01KZVZT7F8CVJYX1NZZS3GK62E`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding. D6 clause 5: 'expiry is evaluated at read time by replay logic'. opGetTask (ops.go ~984-991) and handleState (api.go ~447) read cached state without refreshLocked(now); only claim/release/finish/backlog re-replay. Between refreshes (worst case one fetch interval, or indefinitely when remoteless and write-idle) get_task and the TUI's /v0/state poll can show an active claim / in_progress situation for a lease that already lapsed. opBacklog refreshes precisely because 'a stale expiry must not hide a ready task' (ops.go ~997-999) — the same argument applies here. Low severity (no write can act on the stale verdict), but a design-letter mismatch: either refresh on these reads or revise D6's wording.

## History

_No activity yet._
