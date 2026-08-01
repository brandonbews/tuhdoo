# Escalations

The steering inbox: questions raised by agents, awaiting a human answer.

## Open

### [`t-gsw5`](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) · v0 definition of done: the dogfood week holds

**Blocking** · asked by `brandon/migrator` · 2026-07-30 04:28 UTC

> v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06. The bar (docs/plan/roadmap.md, v0): one week of tuhdoo managing its own development — sessions driven through claim_next, watch running beside agents — with zero manual repair of the data branch. Answer "yes" to release this milestone (then mark it done), or describe what broke.

Raised at B12 cutover (2026-07-30, brandon/migrator): the markdown backlog was migrated into this data branch in one atomic create_task batch and tombstoned on main. This blocking escalation is the DoD clock and the agent fence in one: it keeps the v0 milestone out of the ready pool until a human verifies the week, and it puts the v0->v1 gate in the steering inbox. Development continues meanwhile through claim_next; the TUI task is the top ready item.

### [`t-cb1j`](tasks/t-01KYRMFV10W1N28TCN5WVTCB1J.md) · Two-machine dogfood: real claim races over one remote

**Blocking** · asked by `brandon/impl-2` · 2026-07-30 05:51 UTC

> This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week, and do you want a prep task filed first for sync-latency instrumentation?

Two things, one blocking and one a finding:

1. The task became ready when the TUI landed (fa8c7d3), but its substance — a week of two-machine operation against one origin, a real claim race, answering an escalation from `tuhdoo top` mid-week — is operational work only you can start and pace. This escalation fences it out of the ready pool so agents stop claim-churning it until you kick the week off. Answer when you're ready to begin (or tell me how you'd rather fence human-paced tasks — this is the same workaround B12 used for the milestone, per open-questions Cycle 3).

2. Acceptance requires "collision/latency numbers recorded onto this task as notes", and T8 says the daemon logs collision counts *and sync latencies* — but internal/syncer only counts collisions (Status.Collisions, syncer.go:37); nothing measures or logs fetch/push latency. Options: (a) I file a well-formed prep task to add sync-latency measurement/logging before the week starts (recommended — the week's evidence is half-blind without it); (b) run the week with collisions-only and eyeball latencies from timestamps; (c) you scope it differently. I deliberately did not create the prep task or wire a depends_on edge myself: making this task depend on a new child is exactly the parent/depends_on union-cycle territory open-questions Cycle 3 flags as unsettled.

### [`tuh-2sbw`](tasks/tuh-01KYWVNF91Y7H9GK0X1RAE2SBW.md) · Audit: agents via MCP can perform the main steering actions users ask for

Non-blocking · asked by `brandon/claude-code-1` · 2026-08-01 00:19 UTC

> Parity audit found real MCP visibility gaps: agents cannot list in-progress, blocked, done, or cancelled tasks, nor open escalations. Do you want a T5 design revision (grill cycle) to add an orientation path — e.g. more get_backlog arrays or a /v0/state-shaped read verb?

Audit (PR #7) verified all ten steering write-paths work through the eleven tools, but read-side parity fails against the TUI's sections. Evidence, per section: (1) Needs Input — no verb lists open escalations; a blocking escalation removes its task from ready, so an agent can't even discover the task ID to get_task it; a non-blocking escalation is invisible in the task's backlog row. (2) In Progress — a claimed task vanishes from all three get_backlog arrays, and taskJSON carries no holder field, so "what's in progress and who holds it?" has no MCP path. (3) Blocked — dep-blocked and escalation-blocked tasks appear in no array. (4) Done/Archived — done and cancelled tasks appear nowhere; "what did we finish this week?" is unanswerable. All such tasks remain readable by known ID via get_task; the gap is discovery, not access. Also worth settling in the same revision: T5's sentence "curation (cancel, reprioritize, archive) is human work via CLI/TUI on the HTTP API" no longer matches code or intent — update_task mechanically accepts cancelled/priority from agents and the full-agentic goal depends on that; PR #7 already corrected agent-protocol.md wording, but 002-technology.md T5 itself still says it. Options: (a) grow get_backlog with in_progress/blocked/done/cancelled/escalations arrays (token-heavy — rows carry full descriptions), (b) a new state-shaped orientation verb mirroring GET /v0/state (breaks eleven-verbs), (c) filter/scope inputs on get_backlog. Recommendation: run the grill cycle; my lean is (c) or (a-with-slim-rows) since a twelfth verb needs the stronger justification. This escalation is the audit's designed output — the task said gaps become a design revision, not a workaround.

## Answered

### [`t-vv29`](tasks/t-01KYRVCBE83KT62BAE1502VV29.md) · npm devDependency distribution (esbuild-pattern wrapper packages)

Asked by `brandon/claude-code-11` · 2026-07-31 08:13 UTC

> npm publishing needs credentials only you can provision. Everything else is landed and smoke-tested; the first `v*` tag will publish to npm automatically once these exist. Needed: (1) an npmjs.com account/org owning the `@tuhdoo` scope (create org "tuhdoo" — the scope and the `tuhdoo` package name were both unclaimed as of 2026-07-31); (2) a granular npm access token with read/write publish rights on the `tuhdoo` package and `@tuhdoo` scope (first publish creates them, so the token needs "create packages" rights on the scope); (3) that token added as the repo Actions secret NPM_TOKEN. Also: the repo has no LICENSE file and the npm packages currently ship without a license field — npm warns but publishes; tell me (or a successor) the intended license and we'll add it to the repo + packages. One workflow change awaits your eyes-on review per project law: commit 17ea914 adds an `npm` job to .github/workflows/release.yml (downloads release assets, assembles packages with npm/prepare.js, publishes with NPM_TOKEN via ${NODE_AUTH_TOKEN} in .npmrc; no new third-party actions). Recommendation: review 17ea914, provision the org+token, then push the first tag (e.g. v0.1.0) — that single tag exercises both the release pipeline and the npm tier end-to-end.

**Answer** (`brandon`, relayed by `brandon/claude-code-1`): Workflow commit 17ea914 reviewed and approved as-is; file a follow-up task for the two accepted warts (partial-publish re-run guard, npm provenance attestations). License: MIT — add a LICENSE file to the repo and a license field to all five npm packages. First tag: v0.1.0, to be pushed once the license lands and credentials exist. Provisioning (npm org "tuhdoo", granular token with create-packages rights on the @tuhdoo scope, repo secret NPM_TOKEN) is being done by Brandon now, in-session.
