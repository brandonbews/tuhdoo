# Run records: additive merged_as field for the commit that actually landed

`tuh-01KZA0VT234XJYVZWT8KT0BSDH`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 0
- **Labels:** `go` `ledger` `mcp` `protocol`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Context: promoted from the migrated open-question "run-to-code linkage robustness" at the 2026-08-06 grill (Brandon). In squash and rebase flows — the majority of modern team setups, including this repo's ruleset — a done run's stored pointers all die: the branch is auto-deleted at merge, and the reported branch-commit SHAs never land on the default branch, so the host GCs them after the grace period. The one pointer that stays resolvable — the commit the merge actually created on the durable branch (the squash commit) — has no field in RunFinished. The agent holds that SHA at exactly the right moment: confirm-before-merge sequencing means finish_run(done) fires only after the merge lands. The PR-link half of the original question is settled by T2 (host links are stored strings, never dereferenced — honest disclosure, no change) and stays out of scope.

The ask: an additive optional field, merged_as (list of commit SHAs, matching Commits' shape), on the RunFinished event payload and the finish_run MCP input, flowed through runJSON to the read side and rendered in the task view's run history alongside branch/PR. Field description worded workflow-agnostically: "the commit(s) on a durable branch that carry this work, if known" — never assume squash. One sentence in docs/agent-protocol.md step 6 (finish honestly): in squash/rebase repos the branch commits die at merge; after the merge lands, report the landed commit in merged_as. Doc revision header notes the change and this grill.

Acceptance: RunFinished carries merged_as with old events replaying unchanged (additive-first, T3 — no upcaster; table-driven replay tests cover events with and without the field); finish_run accepts and persists it end-to-end (daemon test); runJSON exposes it and the task view renders it where branch/PR appear; the agent-protocol sentence exists; make test lint green.

Pointers: internal/event/catalog.go (RunFinished), internal/daemon/mcp.go (finish_run input struct ~line 364 and the handler ~542), internal/daemon/api.go (runJSON, runJSONOf), internal/core replay tests for the table-driven pattern, docs/agent-protocol.md step 6.

Constraints: no new MCP verb — a field on an existing tool, T5's twelve-verb surface untouched; stored event bytes never rewritten (T3) — no backfill of historical runs, the gap before this lands is accepted; host-agnostic (T2) — the daemon never dereferences or validates the SHAs, stored strings like branch/PR.

## History

_No activity yet._
