# Escalations

The steering inbox: questions raised by agents, awaiting a human answer.

## Open

### v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06. The bar (docs/plan/roadmap.md, v0): one week of tuhdoo managing its own development — sessions driven through claim_next, watch running beside agents — with zero manual repair of the data branch. Answer "yes" to release this milestone (then mark it done), or describe what broke.

- Task: [t-01KYRMFV10W1N28TCN5NWAGSW5](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) — v0 definition of done: the dogfood week holds
- Asked by: `brandon/migrator`
- Raised: 2026-07-30 04:28 UTC
- Blocking: yes

Raised at B12 cutover (2026-07-30, brandon/migrator): the markdown backlog was migrated into this data branch in one atomic create_task batch and tombstoned on main. This blocking escalation is the DoD clock and the agent fence in one: it keeps the v0 milestone out of the ready pool until a human verifies the week, and it puts the v0->v1 gate in the steering inbox. Development continues meanwhile through claim_next; the TUI task is the top ready item.

### This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week, and do you want a prep task filed first for sync-latency instrumentation?

- Task: [t-01KYRMFV10W1N28TCN5WVTCB1J](tasks/t-01KYRMFV10W1N28TCN5WVTCB1J.md) — Two-machine dogfood: real claim races over one remote
- Asked by: `brandon/impl-2`
- Raised: 2026-07-30 05:51 UTC
- Blocking: yes

Two things, one blocking and one a finding:

1. The task became ready when the TUI landed (fa8c7d3), but its substance — a week of two-machine operation against one origin, a real claim race, answering an escalation from `tuhdoo top` mid-week — is operational work only you can start and pace. This escalation fences it out of the ready pool so agents stop claim-churning it until you kick the week off. Answer when you're ready to begin (or tell me how you'd rather fence human-paced tasks — this is the same workaround B12 used for the milestone, per open-questions Cycle 3).

2. Acceptance requires "collision/latency numbers recorded onto this task as notes", and T8 says the daemon logs collision counts *and sync latencies* — but internal/syncer only counts collisions (Status.Collisions, syncer.go:37); nothing measures or logs fetch/push latency. Options: (a) I file a well-formed prep task to add sync-latency measurement/logging before the week starts (recommended — the week's evidence is half-blind without it); (b) run the week with collisions-only and eyeball latencies from timestamps; (c) you scope it differently. I deliberately did not create the prep task or wire a depends_on edge myself: making this task depend on a new child is exactly the parent/depends_on union-cycle territory open-questions Cycle 3 flags as unsettled.

### npm publishing needs credentials only you can provision. Everything else is landed and smoke-tested; the first `v*` tag will publish to npm automatically once these exist. Needed: (1) an npmjs.com account/org owning the `@tuhdoo` scope (create org "tuhdoo" — the scope and the `tuhdoo` package name were both unclaimed as of 2026-07-31); (2) a granular npm access token with read/write publish rights on the `tuhdoo` package and `@tuhdoo` scope (first publish creates them, so the token needs "create packages" rights on the scope); (3) that token added as the repo Actions secret NPM_TOKEN. Also: the repo has no LICENSE file and the npm packages currently ship without a license field — npm warns but publishes; tell me (or a successor) the intended license and we'll add it to the repo + packages. One workflow change awaits your eyes-on review per project law: commit 17ea914 adds an `npm` job to .github/workflows/release.yml (downloads release assets, assembles packages with npm/prepare.js, publishes with NPM_TOKEN via ${NODE_AUTH_TOKEN} in .npmrc; no new third-party actions). Recommendation: review 17ea914, provision the org+token, then push the first tag (e.g. v0.1.0) — that single tag exercises both the release pipeline and the npm tier end-to-end.

- Task: [t-01KYRVCBE83KT62BAE1502VV29](tasks/t-01KYRVCBE83KT62BAE1502VV29.md) — npm devDependency distribution (esbuild-pattern wrapper packages)
- Asked by: `brandon/claude-code-11`
- Raised: 2026-07-31 08:13 UTC
- Blocking: yes

## Answered

_None._
