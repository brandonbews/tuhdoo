# Escalations

The steering inbox: questions raised by agents, awaiting a human answer.

## Open

### [`t-gsw5`](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) · v0 definition of done: the dogfood week holds

**Blocking** · asked by `brandon/migrator` · 2026-07-30 04:28 UTC

> v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06. The bar (docs/plan/roadmap.md, v0): one week of tuhdoo managing its own development — sessions driven through claim_next, watch running beside agents — with zero manual repair of the data branch. Answer "yes" to release this milestone (then mark it done), or describe what broke.

Raised at B12 cutover (2026-07-30, brandon/migrator): the markdown backlog was migrated into this data branch in one atomic create_task batch and tombstoned on main. This blocking escalation is the DoD clock and the agent fence in one: it keeps the v0 milestone out of the ready pool until a human verifies the week, and it puts the v0->v1 gate in the steering inbox. Development continues meanwhile through claim_next; the TUI task is the top ready item.

### [`t-cb1j`](tasks/t-01KYRMFV10W1N28TCN5WVTCB1J.md) · Two-machine convergence: a deliberate claim-collision harness

**Blocking** · asked by `brandon/impl-2` · 2026-07-30 05:51 UTC

> This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week, and do you want a prep task filed first for sync-latency instrumentation?

Two things, one blocking and one a finding:

1. The task became ready when the TUI landed (fa8c7d3), but its substance — a week of two-machine operation against one origin, a real claim race, answering an escalation from `tuhdoo top` mid-week — is operational work only you can start and pace. This escalation fences it out of the ready pool so agents stop claim-churning it until you kick the week off. Answer when you're ready to begin (or tell me how you'd rather fence human-paced tasks — this is the same workaround B12 used for the milestone, per open-questions Cycle 3).

2. Acceptance requires "collision/latency numbers recorded onto this task as notes", and T8 says the daemon logs collision counts *and sync latencies* — but internal/syncer only counts collisions (Status.Collisions, syncer.go:37); nothing measures or logs fetch/push latency. Options: (a) I file a well-formed prep task to add sync-latency measurement/logging before the week starts (recommended — the week's evidence is half-blind without it); (b) run the week with collisions-only and eyeball latencies from timestamps; (c) you scope it differently. I deliberately did not create the prep task or wire a depends_on edge myself: making this task depend on a new child is exactly the parent/depends_on union-cycle territory open-questions Cycle 3 flags as unsettled.

## Answered

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
