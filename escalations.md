# Escalations

The steering inbox: questions raised by agents, awaiting a human answer.

## Open

### [`tuh-d9bk`](tasks/tuh-01KZ86YH64K9D2AKVQF57KD9BK.md) · Lease tombstones: released marker, deletion retired, merge rule (grill 2026-08-04)

**Blocking** · asked by `brandon/claude-code-1` · 2026-08-05 06:12 UTC

> Finding 3 blocks the 17/17 harness bar: the MCP renewal loop deliberately evicts provisionally-voided claims from session tracking, so a race loser's confirm_claim answers "this session holds no claim" instead of D6's promised "lost". Which semantics should the fix implement, and does PR #32 (the decided tombstone work, complete and green) merge now or ride along with the finding-3 fix?

The decided tombstone work is fully implemented on branch tuh-d9bk/lease-tombstones (PR #32, now draft, unmerged): released tombstones, DeleteLease retired, the three-arm merge rule, replay untouched, doc revisions — make test lint green, plus new store/syncer/core tests including the finding-1 shape at five replay instants. Run against the reworked harness (tuh-ysvn harness/ overlaid on the fix), seeding, the full confirmation-race storm (40 contests, exactly one confirmation each, losers coerced/synthesized correctly — the finding-1/2 machinery works), and two-machine convergence all pass. The run then exits 1 at the settle phase, first confirm-first attempt: "confirm_claim on alpha: this session holds no claim on task ...: claim_next or claim_task first".

Cause (code-certain, and pre-existing — my branch does not touch mcp.go): confirm_claim gates on the session's own tracking (heldClaim, internal/daemon/mcp.go ~line 488). renewOnce (~line 253) untracks any tracked claim whose Status != ClaimActive on its 5-minute tick (leaseTTL/3), and its comment names "lost to a cross-machine race" as a deliberate eviction case — written in the #28/#29 gate build, and now contradicting the D6 clause 3 rewrite ("any verb touched by a provisionally-voided claimant says plainly that it lost"). The storm never hits it because contests confirm within seconds of claiming; the settle phase sits minutes after the raced claims, so whether a tick lands in the window is run-timing luck — this bar is a coin flip until fixed, and it will never be deterministic evidence for roadmap v1 DoD clause 2.

Two candidate semantics (both keep voided claims tracked so the verbs answer honestly; they differ on lease renewal):
(i) Tracked but never renewed: the loser hears "lost" for up to TTL after the last renewal; after the lease lapses, replay synthesis closes the attempt and the verbs answer "attempt closed, salvage via add_note" — also a D6-sanctioned answer. Minimal change, preserves the just-grilled expiry-synthesis timing, and keeps the tombstone merge rule's safety rationale trivially intact (a voided lease is never renewed at all, so no renewal can postdate a tombstone).
(ii) Tracked and renewed while the session lives: "lease expires unclosed" then effectively means session death, so a connected loser stays confirmable/reportable indefinitely and synthesis only ever closes disconnected losers. More faithful to T5/T8 session-bound leases, but it re-opens the expiry-synthesis timing that D6 clause 3 just settled.

My recommendation: (i), folded into this same task (the acceptance bar already names 17/17 as the definition of done), then re-run the harness for a real 17/17 and merge PR #32 with the renewal fix included — one PR, since the bar is only meetable with both. If you'd rather treat finding 3 as its own grilled decision first, the alternative is: merge PR #32 as-is now (it is necessary under either finding-3 outcome and independently tested) and spawn a finding-3 task; the dependent harness-merge task tuh-01KZ5WMT4GWZTYVRGWN56TYSVN stays gated either way until the bar is green.

## Answered

### [`t-gsw5`](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) · v0 definition of done: the dogfood week holds

Asked by `brandon/migrator` · 2026-07-30 04:28 UTC

> v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06. The bar (docs/plan/roadmap.md, v0): one week of tuhdoo managing its own development — sessions driven through claim_next, watch running beside agents — with zero manual repair of the data branch. Answer "yes" to release this milestone (then mark it done), or describe what broke.

**Answer** (`brandon`): Yes — the week held, and the milestone is closed as of today (2026-08-03) rather than on 2026-08-06, because the evidence is already conclusive and the calendar was never the bar.

The DoD has been rewritten (docs/plan/roadmap.md, v0) from elapsed time to five checkable facts, all true now:

1. the backlog lives on the data branch; docs/plan/backlog.md is a tombstone — done at the B12 cutover, 2026-07-30;
2. every commit on refs/heads/tuhdoo is daemon-authored — 369 of 369, zero human commits, no hand repair ever;
3. an event-schema version bump landed on the live branch and replayed correctly — task.created/task.updated v1 to v2, 2026-07-31, with identity upcasters registered in core.NewReplayer;
4. agents drove the full loop through claim_next to finish_run/escalate, with no direct git writes to the data branch;
5. the daemon was restarted mid-session on every deploy since the cutover, with no lost or corrupted events.

Why the week was retired rather than waited out: the binary changed every few minutes throughout, so a strict reading reset the clock on every deploy and the criterion was unsatisfiable for as long as development continued. The load-bearing clause was never the week — it was 'no manual repair', which is point 2 and is mechanically checkable. The rapid iteration made this a harsher test than the week intended, not a weaker one: a live schema bump on a running ledger and dozens of mid-claim restarts are exactly the failure modes the criterion existed to catch.

Filing note: this escalation was the wrong fence. Nothing had stalled — the milestone was simply not to be worked yet, which is 'held'. It was chosen on 2026-07-30, one day before the held status existed. The rule is now written down: docs/agent-protocol.md, 'no attempt, no escalation'.

### [`t-cb1j`](tasks/t-01KYRMFV10W1N28TCN5WVTCB1J.md) · Two-machine convergence: a deliberate claim-collision harness

Asked by `brandon/impl-2` · 2026-07-30 05:51 UTC

> This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week, and do you want a prep task filed first for sync-latency instrumentation?

**Answer** (`brandon`): Neither — the premise is retired. The milestone grill of 2026-08-03 rewrote this task's acceptance from 'a week of two-machine operation' to a deliberate claim-collision harness (see the rewritten description). It is no longer human-paced: two clones on one box give two genuinely independent daemons, because machineID is minted per repo directory and ULID ordering never trusts wall clocks. An agent can execute it start to finish, so it returns to the ready pool with this answer rather than waiting on me.

On the sync-latency prep task: not filed, and the latency requirement is dropped from acceptance. The grill found the deeper measurement problem — syncer.Status.Collisions counts non-fast-forward pushes, not claims voided by the D6 winner rule, so the original 'collision/latency numbers' bar would have recorded the wrong quantity no matter how much latency instrumentation landed first. The rewritten acceptance names the facts that actually prove convergence (one winner per race, a superseded run per loser, byte-identical state and views, at least one real merge commit) and folds the push-contention count in as one reported figure among several. If latency proves to matter, it gets filed on evidence from this run rather than ahead of it.

Filing note for the record: fencing this task with a blocking escalation was the wrong tool. Nothing had stalled — the task simply wasn't to be started yet, which is 'held'. The rule is now written down: docs/agent-protocol.md, 'no attempt, no escalation'.

### [`t-vv29`](tasks/t-01KYRVCBE83KT62BAE1502VV29.md) · npm devDependency distribution (esbuild-pattern wrapper packages)

Asked by `brandon/claude-code-11` · 2026-07-31 08:13 UTC

> npm publishing needs credentials only you can provision. Everything else is landed and smoke-tested; the first `v*` tag will publish to npm automatically once these exist. Needed: (1) an npmjs.com account/org owning the `@tuhdoo` scope (create org "tuhdoo" — the scope and the `tuhdoo` package name were both unclaimed as of 2026-07-31); (2) a granular npm access token with read/write publish rights on the `tuhdoo` package and `@tuhdoo` scope (first publish creates them, so the token needs "create packages" rights on the scope); (3) that token added as the repo Actions secret NPM_TOKEN. Also: the repo has no LICENSE file and the npm packages currently ship without a license field — npm warns but publishes; tell me (or a successor) the intended license and we'll add it to the repo + packages. One workflow change awaits your eyes-on review per project law: commit 17ea914 adds an `npm` job to .github/workflows/release.yml (downloads release assets, assembles packages with npm/prepare.js, publishes with NPM_TOKEN via ${NODE_AUTH_TOKEN} in .npmrc; no new third-party actions). Recommendation: review 17ea914, provision the org+token, then push the first tag (e.g. v0.1.0) — that single tag exercises both the release pipeline and the npm tier end-to-end.

**Answer** (`brandon`, relayed by `brandon/claude-code-1`): Workflow commit 17ea914 reviewed and approved as-is; file a follow-up task for the two accepted warts (partial-publish re-run guard, npm provenance attestations). License: MIT — add a LICENSE file to the repo and a license field to all five npm packages. First tag: v0.1.0, to be pushed once the license lands and credentials exist. Provisioning (npm org "tuhdoo", granular token with create-packages rights on the @tuhdoo scope, repo secret NPM_TOKEN) is being done by Brandon now, in-session.

### [`tuh-2sbw`](tasks/tuh-01KYWVNF91Y7H9GK0X1RAE2SBW.md) · Audit: agents via MCP can perform the main steering actions users ask for

Asked by `brandon/claude-code-1` · 2026-08-01 00:19 UTC

> Parity audit found real MCP visibility gaps: agents cannot list in-progress, blocked, done, or cancelled tasks, nor open escalations. Do you want a T5 design revision (grill cycle) to add an orientation path — e.g. more get_backlog arrays or a /v0/state-shaped read verb?

**Answer** (`brandon`, relayed by `brandon/claude-code-1`): Yes — T5 revision grilled and settled 2026-08-01. Decisions: (1) MCP must be read-self-sufficient — the CLI one-shot contract stays the scripting surface, not the excuse; the blocking-escalation discovery hole and relay_answer's need for listable escalation IDs made the hybrid doctrine untenable. (2) Mechanism is the audit's lean: an optional scope input on get_backlog (values in_progress, blocked, done, cancelled, escalations — plumbing vocabulary); omitted scope stays byte-identical to today so the worker hot path pays nothing; verb count stays eleven. (3) Slim rows as a design constraint: id/title/status/priority/labels plus per-scope payoff fields (holder+lease, dep:/esc: reasons, closed_at/closed_by newest-first), no descriptions — get_task hydrates; escalations scope returns full open records in raise order. (4) The rider is in: T5's "curation is human work" sentence gets rewritten in the same revision — curation is mechanically open to agents via update_task, the human-direction norm lives in agent-protocol.md. Implementation task with the full settled design: tuh-01KYZ9FJH4N2XFRXJ9ANV1M0QK (docs-revision-first, priority 1).

### [`tuh-11sh`](tasks/tuh-01KYXEMYC5XE928EWKYA0P11SH.md) · npm provenance: trusted publishing promised attestations, the registry has none

Asked by `brandon/claude-code-1` · 2026-08-01 03:50 UTC

> PR #13 (https://github.com/brandonbews/tuhdoo/pull/13) changes .github/workflows/release.yml and per the workflow-file law needs your eyes-on diff review before merge — auto-merge is deliberately NOT enabled. The diff is two hunks: (1) comment correction, (2) `npm publish --access public --provenance` (one added flag). Root cause in the PR body: npm auto-enable of provenance under trusted publishing fails silently (verbose-only logging, npm/cli oidc.js); explicit flag makes future failures loud. Options: (a) review and merge PR #13 yourself, or (b) reply approving it and the next claimant merges and finishes. Recommendation: (a) — one-glance diff. Registry verification (dist.attestations on all five packages + npmjs badge) is deferred to the next v* tag by nature.

**Answer** (`brandon`): i don't see any open PRs so i assume this is done

### [`tuh-ysvn`](tasks/tuh-01KZ5WMT4GWZTYVRGWN56TYSVN.md) · Collision harness: drive the real D6 machinery, add a confirmation-race storm

Asked by `brandon/claude-code-1` · 2026-08-05 02:29 UTC

> The reworked collision harness (branch tuh-ysvn/collision-harness-real-machinery, commit 5c4d547) found two production gaps in PR #30's silent-loser path; fixing them changes lease semantics, which is design territory — how should leases end when a voided claimant stands down?
>
> Finding 1 — lease deletion rewrites replay history. releaseVoidedLocked (internal/daemon/ops.go) deletes the loser's lease, but leaseExpiredBy (internal/core/replay.go) counts a MISSING lease as lapsed at EVERY instant, including past ones. In exactly the contests where a confirmation out-ranked an earlier-ULID claim, replay thereafter sees the earlier claim as lease-less at claim-apply time and records it expired with a synthesized interrupted run — not the promised superseded run. Deterministic on both machines, but the verb's 'recorded as superseded' acknowledgment is false. Hit 13/40 storm contests.
>
> Finding 2 — merge resurrects deleted leases. The union merge (internal/syncer/merge.go) brings deleted lease files back; its comment ('a resurrected lease only matters to an ACTIVE claim') predates PR #30 making voided-lease deletion load-bearing. Whether a stand-down closes immediately or waits out the 15-min TTL is a merge-timing coin flip (5/10 silent losers un-closed at verify).
>
> Options I see: (a) releaseVoidedLocked overwrites the lease with an already-lapsed expiry (a tombstone) instead of deleting — fixes finding 1 since the lease exists with a real expiry; but the merge's 'later expiry wins' rule then lets a racing renewal beat the tombstone, so finding 2 needs the rule to let releases beat renewals (e.g. tombstones win, or lease files carry a released marker). (b) Keep deletion but make leaseExpiredBy time-aware (missing lease = lapsed only from some instant) — needs a durable record of WHEN it vanished, which deletion doesn't leave; seems structurally worse. (c) Don't touch leases on stand-down: the attempt closes by natural TTL expiry (up to 15 min later); simplest, costs only latency of the synthesized close, but the ack text must stop promising immediate closure.
>
> My recommendation: (a), decided through a grill cycle since it amends the T8/merge lease rules and D6 clause 3's mechanism. The harness rewrite itself is complete and is the evidence instrument for roadmap v1 DoD clause 2: its marquee checks pass (zero duplicate confirmations across 40 raced contests, one claim.confirmed per contest, byte-identical state/views both sides); the 3 failing checks are precisely these findings, written up in harness/README.md on the branch. The harness cannot go green (its acceptance bar) until the lease design is settled and fixed in a follow-up task.

**Answer** (`brandon`, relayed by `brandon/claude-code-1`): Decision (Brandon, 2026-08-04 short grill in-session): option (a) with a correction found during the grill — a plain lapsed-expiry overwrite is insufficient because the merge's later-expiry-wins rule would let a peer's stale lease copy beat it. Decided: (1) lease tombstones carry an explicit marker — overwrite with {expires: <stand-down instant>, released: true}; (2) lease deletion is retired everywhere (both DeleteLease call sites; the invariant becomes 'lease files are never deleted, only overwritten'), which also defuses union resurrection; (3) merge rule for leases: released beats plain regardless of expiry, two released -> earlier expiry wins, two plain -> later expiry wins (unchanged); (4) replay unchanged — the tombstone's expiry makes leaseExpiredBy correct at every instant. Rationale recorded: a claim's lease has one writer (its own daemon, one mutex) and a daemon never renews after standing down, so released-beats-plain can never undo a live renewal. Implementation is task tuh-01KZ86YH64K9D2AKVQF57KD9BK (full spec + doc-revision list in its description); the harness task now depends on it and its acceptance bar stays harness 17/17.
