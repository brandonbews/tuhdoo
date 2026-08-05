# Cut v0.2.0: final verification and tag handoff

`tuh-01KZ9Y3THHH5B8GT22T910R40K`

- **Status:** open — ready
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

_No activity yet._
