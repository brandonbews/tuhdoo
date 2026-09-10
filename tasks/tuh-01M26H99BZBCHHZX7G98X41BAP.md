# Design revision: the daemon is the live replica; git is touched only when the ledger changes (D2, D3/T6, D6, D9, T2, T4)

`tuh-01M26H99BZBCHHZX7G98X41BAP`

- **Status:** done
- **Priority:** 1
- **Labels:** `design` `docs` `design-revision`
- **Created:** 2026-09-10 20:48 UTC by `brandon`

## Description

Context: grilled 2026-09-10 with Brandon, from the "TUI stuck at loading" diagnosis. Measured on this repo's ledger (753 events, 123 leases, 162 tasks): cold start ran one `git cat-file` per blob (876 processes, 7.2s) against a fixed 5s client wait, so any launch after a daemon death failed with "daemon did not come up within 5s" and the MCP shim reported "Connection closed"; every read op (14 `refreshLocked` call sites) ran rev-parse + ls-tree + full replay (~36ms) under the daemon mutex; the TUI hydrated every task individually every 2s with no in-flight guard, so one snapshot cost ~6s of serialized daemon time while a new one started every 2s (first paint ~20s, unbounded pile-up, ~80 process spawns/s per open pane); every commit ran ~195 subprocesses (166 `hash-object` on mostly-unchanged rendered views, 25 `mktree`); merges and the confirmation gate re-read every event of the other side's tree with no cache (~6s each). The architectural miscalculation: the daemon was built as a stateless front-end to git instead of the live replica D2 already describes. Three multiplying costs (spawns per object x objects per operation x operations per client tick) fall out of that one posture; fixing one alone lets the others grow back into the same wall.

The ask: revise the design record in place, dated notes, the D5/D6/D8 pattern. No code in this PR. The settled decisions (all Brandon's, on recommendation, 2026-09-10):

1. D2 gains an implementation-posture note: the daemon's memory is the live replica; git is touched only when the ledger changes (commit, merge, cold start); reads never spawn git. Accepted consequence: an external move of refs/heads/tuhdoo (manual fetch into it, branch -f) is noticed at the next sync cycle (<=60s) and reloaded, not on the next read.
2. T2: the store is the single mover of the local ref; the syncer computes the union tree and commits through the store (syncer gains a dependency on store). Accepted consequence: syncer merge tests run against a store, not a bare git fake.
3. D6 clause 5 unchanged in substance; note: read-time evaluation is memoized between lease transitions via a pure `core.NextTransition(leases, now)`; there is still no reaper. State carries a monotonic version bumped on staged commit, merge, and lease transition. Accepted consequence: a lease lapsing wakes every waiter at that instant, so an open pane redraws the moment a task returns to the pool.
4. T4: the JSON API's read shape is one versioned snapshot, `GET /v0/snapshot?since=N`, long-polled (parks until the version moves or ~30s). `/v0/state` and `GET /v0/tasks/{id}` are deleted. Accepted consequence: each open pane holds one parked request; shutdown must wake parked requests before the HTTP server drains.
5. The snapshot carries every task fully hydrated (~690KB today, ~4KB/task; six months at August pace ~3.7MB). Working-set retirement (tuh-01KZA0VT234XJYVZWT980V7K2Y) is the long-term bound, not this design.
6. T2: tuhdoo may compute object IDs itself (blob OID = hash of "blob <len>\0<bytes>", SHA-1 or SHA-256 per `rev-parse --show-object-format`, detected once at startup) but never writes an object except through git. Used to diff rendered views and events against the in-memory tree so only genuinely new blobs are written.
7. T2: the commit tree is built through a private index at `.git/tuhdoo/index` (`GIT_INDEX_FILE`), reseeded from the head tree with `read-tree` at every daemon start, fed only changed paths via `update-index --index-info` (mode 0 lines remove), then `write-tree`. Two processes per commit at any depth. Note that a private index is not a worktree: the data branch is still never checked out; the user's own worktrees are unaffected and unrestricted.
8. New blobs are written one `hash-object -w --stdin` per blob (typical commit: one event plus three to five views). Accepted consequence: a merge re-rendering many task pages costs a process per changed page (~0.5s for fifty); batching is a local gitx change if measurement ever demands it.
9. D3 and T6: "after every write/merge" becomes "after every state change": views are a projection of the versioned state, rendered on every version bump, staged only when bytes change (OID diff), riding the batcher's 2s quiet and the next sync cycle. This deletes `stageViewsLocked` as a thing a write path can forget (the bug behind tuh-01M0XBC1P2NYTPZQ4BAFSWHGY1 and tuh-01KZWNMJH264W3B3TGNP7FP51R). D9 gains a commit trigger: lease expiry produces a view-only commit ("0 events, N files"), bounded by how often leases lapse. Claim commits poke the syncer (T8 eager wire time, as always intended).
10. T2: `gitx.Git` gains `CatFiles(oids)` over one `cat-file --batch`; `CatFile` becomes a wrapper so the batch format is parsed in one place; syncer replays read decoded events from the store's cache by OID.
11. T4: the daemon takes the lock, binds the socket, writes daemon.json, then loads; every endpoint answers `starting` until the first replay lands; the client's fixed `spawnWait` is deleted and replaced by a generous ceiling (~30s) on the socket appearing, pointing at daemon.log. Accepted consequence: writes arriving during the load window are rejected with a retryable `starting`, never queued.
12. T5 untouched: the twelve tools read memoized state; no new tools. T7 untouched: same screens, same keys, one-shot output byte-identical. T8: no knob changes; optional note that merges and the gate are now cheap enough that lowering the fetch interval (the only lever on the provisional collision window) is a knob turn, not a project.

Cross-machine posture, verified 2026-09-10 and worth one sentence in the D2 note: nothing here touches the syncer's write path semantics (fetch/union-merge/CAS push, the confirmation gate, lease merge rules), so conflict count, conflict handling, the provisional winner, the CAS-anchored final verdict, and cross-machine lease verdicts are unchanged; they get ~200x cheaper locally.

Acceptance:
- Every clause above carries a dated revision note in 001 (D2, D3, D6 clause 5, D9) or 002 (T2, T4, T6, T8) naming the decision and its accepted consequence, in the existing in-place style. No clause is rewritten wholesale.
- The three subsumed captures are cited by ID where their evidence is used (the D2 note cites tuh-01M11XDA6ST74WD1JWCWEAW1V2; the T6 note cites tuh-01M0XBC1P2NYTPZQ4BAFSWHGY1 and tuh-01KZWNMJH264W3B3TGNP7FP51R).
- `make test lint` green; PR title = this task's title; PR body opens with this task ID.

Pointers: internal-docs/design/001-core-design.md (D2, D3, D6, D9); internal-docs/design/002-technology.md (T2, T4, T6, T8); CLAUDE.md "Working conventions" for the revision pattern.

Constraints: docs only. Do not un-accept any previously accepted consequence; add to them. Nothing under .github/workflows.

## History

### 2026-09-10 23:15 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-01M26H99BZ/design-live-replica`
- PR: <https://github.com/brandonbews/tuhdoo/pull/102>
- Merged as: `2919067dc5d670369d518b5ec4ee0bbae4cd1225`

Landed as PR #102 (squash 2919067). Dated in-place revision notes on 001 (D2, D3, D6 clause 5, D9) and 002 (T2, T4, T5, T6, T7, T8) record every clause of the 2026-09-10 live-replica grill with its accepted consequence; captures tuh-01M11XDA6ST74WD1JWCWEAW1V2, tuh-01M0XBC1P2NYTPZQ4BAFSWHGY1, tuh-01KZWNMJH264W3B3TGNP7FP51R cited where their evidence is used. A code-review pass surfaced factual gaps that the second commit closes (remoteless ref check, index reseed on every head reload, snapshot answers when version differs from since, starting-window per client, fleet cost of lapse commits). Four review findings dispute settled decisions and are listed on the PR for Brandon rather than acted on. Side capture: tuh-01M26JSHKSEQ0MJYBK1NK18X1S (shim stdin test fails under Go 1.27 locally; CI on 1.26 green).
