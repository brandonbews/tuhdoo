# Lease tombstones: released marker, deletion retired, merge rule (grill 2026-08-04)

`tuh-01KZ86YH64K9D2AKVQF57KD9BK`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 1
- **Labels:** `daemon` `core` `syncer` `store` `d6`
- **Created:** 2026-08-05 05:39 UTC by `brandon/claude-code-1`

## Description

Context: the reworked collision harness (branch tuh-ysvn/collision-harness-real-machinery, commit 5c4d547, pushed unmerged) exposed two production gaps in the PR #30 stand-down path, escalated as 01KZ7W28PB9GPHM0CSQQ2QFABM and design-decided by Brandon in a 2026-08-04 short grill: (1) deleting a voided claim's lease rewrites replay history — leaseExpiredBy (internal/core/replay.go) counts a missing lease as lapsed at every instant, so past claim contests re-adjudicate and the promised superseded run becomes interrupted/expired; (2) the union merge resurrects deleted lease files, making immediate closure a merge-timing coin flip.

The decision (implement exactly this):
- Lease tombstones with an explicit marker: stand-down and release overwrite the lease file with {"expires": "<the instant>", "released": true} instead of deleting. Add the released field to store.leaseFile; Encode/DecodeLease grow tombstone support (old binaries ignore the unknown field and read an ordinary lapsed lease — acceptable degradation; leases are mutable files, no T3 concern).
- Retire lease deletion everywhere: both DeleteLease call sites in internal/daemon/ops.go (normal holder release ~line 454 and releaseVoidedLocked ~line 484) switch to tombstone writes; delete store.DeleteLease itself so no future caller re-imports the bug. New invariant: lease files are never deleted, only overwritten.
- Merge rule for leases/ same-path conflicts (internal/syncer/merge.go): released beats plain regardless of expiry; two released -> earlier expiry wins (fail-safe determinism, same posture as replay's earliest-confirmation rule); two plain -> later expiry wins (unchanged). Union resurrection of one-sided lease files stays and is now harmless. Rewrite the stale merge.go lease comment to state the new invariant — its staleness is what hid finding 2.
- Replay changes: NONE. The tombstone's expiry makes leaseExpiredBy answer correctly at every instant (live before stand-down, lapsed after). Do not touch leaseExpiredBy semantics for genuinely-missing leases.
- Doc revisions with dated notes (2026-08-04 grill), the D5/D6/D8 in-place pattern: 002 T8 lease wording (never deleted, tombstone rule), D6 clause 3 mechanism note in 001, and the merge comment above.

Why released-beats-plain is safe (recorded so it isn't re-litigated): a claim's lease is written only by the claiming machine's own daemon under one mutex, and a daemon never renews after standing down — so a renewal later than the tombstone structurally cannot exist; any plain copy losing to a tombstone is by construction stale.

Acceptance criteria:
- Store tests: tombstone round-trip; old-format lease still decodes.
- Syncer literal-tree table tests: released-vs-plain both directions -> identical trees, tombstone kept; released-vs-released -> earlier expiry; plain-vs-plain unchanged.
- Core tests: a voided claim with a tombstoned lease synthesizes the branch-less superseded run from the stand-down instant onward and never re-adjudicates past contests (the finding-1 shape: confirmation out-ranked an earlier-ULID claim, loser stood down — the loser stays superseded, the winner's contest history is stable at every replay instant).
- go run ./harness/collision passes 17/17 including the confirmation-race storm — this is the bar; then merge branch tuh-ysvn/collision-harness-real-machinery (its own PR, body citing roadmap v1 DoD clause 2 evidence) after this fix lands.
- make test lint green from the repo root.

Pointers: internal/store/lease.go, internal/syncer/merge.go (lease arm + comment block ~lines 25-40), internal/daemon/ops.go (two DeleteLease sites), internal/core/replay.go leaseExpiredBy (read, do not change), harness/README.md findings section (mark resolved with date when green).

Constraints: boring Go (T1); replay stays pure; stored event bytes untouched (leases are not events); daemon sole writer (D2); do not weaken any harness check to pass. All fleet binaries must be rebuilt after landing (merge-rule change) — same deploy posture as the confirmation gate.

## History

### 2026-08-05 06:12 UTC — escalation from `brandon/claude-code-1` (blocking)

> Finding 3 blocks the 17/17 harness bar: the MCP renewal loop deliberately evicts provisionally-voided claims from session tracking, so a race loser's confirm_claim answers "this session holds no claim" instead of D6's promised "lost". Which semantics should the fix implement, and does PR #32 (the decided tombstone work, complete and green) merge now or ride along with the finding-3 fix?

The decided tombstone work is fully implemented on branch tuh-d9bk/lease-tombstones (PR #32, now draft, unmerged): released tombstones, DeleteLease retired, the three-arm merge rule, replay untouched, doc revisions — make test lint green, plus new store/syncer/core tests including the finding-1 shape at five replay instants. Run against the reworked harness (tuh-ysvn harness/ overlaid on the fix), seeding, the full confirmation-race storm (40 contests, exactly one confirmation each, losers coerced/synthesized correctly — the finding-1/2 machinery works), and two-machine convergence all pass. The run then exits 1 at the settle phase, first confirm-first attempt: "confirm_claim on alpha: this session holds no claim on task ...: claim_next or claim_task first".

Cause (code-certain, and pre-existing — my branch does not touch mcp.go): confirm_claim gates on the session's own tracking (heldClaim, internal/daemon/mcp.go ~line 488). renewOnce (~line 253) untracks any tracked claim whose Status != ClaimActive on its 5-minute tick (leaseTTL/3), and its comment names "lost to a cross-machine race" as a deliberate eviction case — written in the #28/#29 gate build, and now contradicting the D6 clause 3 rewrite ("any verb touched by a provisionally-voided claimant says plainly that it lost"). The storm never hits it because contests confirm within seconds of claiming; the settle phase sits minutes after the raced claims, so whether a tick lands in the window is run-timing luck — this bar is a coin flip until fixed, and it will never be deterministic evidence for roadmap v1 DoD clause 2.

Two candidate semantics (both keep voided claims tracked so the verbs answer honestly; they differ on lease renewal):
(i) Tracked but never renewed: the loser hears "lost" for up to TTL after the last renewal; after the lease lapses, replay synthesis closes the attempt and the verbs answer "attempt closed, salvage via add_note" — also a D6-sanctioned answer. Minimal change, preserves the just-grilled expiry-synthesis timing, and keeps the tombstone merge rule's safety rationale trivially intact (a voided lease is never renewed at all, so no renewal can postdate a tombstone).
(ii) Tracked and renewed while the session lives: "lease expires unclosed" then effectively means session death, so a connected loser stays confirmable/reportable indefinitely and synthesis only ever closes disconnected losers. More faithful to T5/T8 session-bound leases, but it re-opens the expiry-synthesis timing that D6 clause 3 just settled.

My recommendation: (i), folded into this same task (the acceptance bar already names 17/17 as the definition of done), then re-run the harness for a real 17/17 and merge PR #32 with the renewal fix included — one PR, since the bar is only meetable with both. If you'd rather treat finding 3 as its own grilled decision first, the alternative is: merge PR #32 as-is now (it is necessary under either finding-3 outcome and independently tested) and spawn a finding-3 task; the dependent harness-merge task tuh-01KZ5WMT4GWZTYVRGWN56TYSVN stays gated either way until the bar is green.

**Answer** (`brandon`, relayed by `brandon/claude-code-1`): Take the agent's recommendation: option (i) — voided claims stay tracked in the MCP session but are never renewed — folded into this same task, then re-run the harness for 17/17 and merge PR #32 with the renewal fix included, as one PR. (Brandon, live in session: "i really dont understand any of this problem. i'll take your recommendation on the escalation")

### 2026-08-05 06:13 UTC — note from `brandon/claude-code-1`

Resume state: all decided tombstone work is done on branch tuh-d9bk/lease-tombstones (commit 2204873, pushed; PR #32, held as draft pending escalation 01KZ88VCEP4AZ8CXY5DW1R72C6). make test lint green. To reproduce the acceptance bar: git worktree at the branch commit, then `git checkout origin/tuh-ysvn/collision-harness-real-machinery -- harness/`, then `go run ./harness/collision` (~10 min); this run passed seeding + confirmation-race storm + convergence and exited 1 at the settle phase on the finding-3 eviction (fix site if answer is option (i): internal/daemon/mcp.go renewOnce — stop untracking ClaimVoided claims; keep the renewal skip; heldClaim gate then answers correctly). After the answer lands: implement, re-run harness for 17/17, mark PR #32 ready, confirm_claim before arming auto-merge.

### 2026-08-05 06:13 UTC — run by `brandon/claude-code-1` — blocked

- Branch: `tuh-d9bk/lease-tombstones`
- PR: <https://github.com/brandonbews/tuhdoo/pull/32>
- Commits: `2204873`

Tombstone work complete and green on PR #32 (draft); blocked on escalation 01KZ88VCEP4AZ8CXY5DW1R72C6 (finding 3: renewOnce evicts voided claims, harness settle fails before the 17/17 bar).
