# Cut v0.2.0: final verification and tag handoff

`tuh-01KZ9Y3THHH5B8GT22T910R40K`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 0
- **Labels:** `release`
- **Depends on:** [`tuh-wzrg`](tuh-01KZ9Y3THHH5B8GT22SY3FWZRG.md) (done), [`tuh-wpyp`](tuh-01KZ9Y3THHH5B8GT22T1A1WPYP.md) (done), [`tuh-r3e8`](tuh-01KZ9Y3THHH5B8GT22T1TZR3E8.md) (done), [`tuh-2hvf`](tuh-01KZ9Y3THHH5B8GT22T5D72HVF.md) (done), [`tuh-nvyk`](tuh-01KZ9Y3THHH5B8GT22T650NVYK.md) (done), [`tuh-y4re`](tuh-01KZ9Y3THHH5B8GT22T7JVY4RE.md) (done), [`tuh-7a40`](tuh-01KZ4TH4HT56TE4CQPKKA37A40.md) (done)
- **Created:** 2026-08-05 21:43 UTC by `brandon/claude-code-1`

## Description

Context: release grill 2026-08-05. v0.2.0 is the first tag a second project will pin — the last tag (v0.1.1, 2026-07-31) predates the confirmation gate and must not be installed anywhere. This task is the checklist gate: it depends on every sweep task and only becomes claimable when all have landed. The tag push itself is Brandon's — agents never push tags.

The ask: (1) Verify fresh main carries all dependency PRs; `make test lint` green from the repo root; run npm/smoke.sh locally and confirm green. (2) PR any final release touches: root README install examples referencing v0.2.0. (3) Rebuild and restart the dogfood daemon per CLAUDE.md's deploy law. (4) Raise a blocking escalation handing Brandon the exact tag command (annotated tag v0.2.0 on the verified main SHA + push) — this is a legitimate blocking escalation: the work is done and the remaining step is human-only. (5) After Brandon tags: verify release.yml ran green, the smoke gate passed, npm shows 0.2.0 and the tarball assets exist; report versions; then finish_run(done).

Acceptance: all dependencies done; smoke green locally before the escalation; escalation raised with the exact commands; post-tag: release workflow green and artifacts published at 0.2.0.

Constraints: no tag push by agents; no workflow-file edits in this task; if the release workflow fails after tagging, that is fix-or-blocked, not done.

## History

### 2026-08-05 22:57 UTC — escalation from `brandon/claude-code-1` (blocking)

> v0.2.0 is ready to tag — the tag push is yours (agents never push tags). Everything upstream is verified: all seven release PRs (#37–#43) are squash-merged on main at 4dc96df1097b94167297d8985c6049f7006ece4c, make test lint is green, npm/smoke.sh passes locally (twelve verbs, named-list assertion), and the dogfood daemon is running the release candidate. ONE THING TO REVIEW FIRST: PR #41 changed .github/workflows/release.yml (adds setup-go + a smoke gate before npm publish — two steps, pinned SHAs; full diff called out in that PR's body). Tagging fires that workflow, so please eyeball the diff before pushing the tag. Then, from the repo root:
>
>   git fetch origin && git tag -a v0.2.0 4dc96df1097b94167297d8985c6049f7006ece4c -m 'v0.2.0: release-grill sweep — parents removed, dep-loop loudness, clone-join adoption, smoke-gated releases' && git push origin v0.2.0
>
> The release workflow will build, smoke-test, publish the GitHub release, and publish npm 0.2.0. After it goes green, the next claimant of this task verifies artifacts and closes it done. If the workflow goes red instead, the task description says fix-or-blocked, not done.

_Unanswered._
