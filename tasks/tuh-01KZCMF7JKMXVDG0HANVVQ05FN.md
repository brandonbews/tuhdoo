# claim_next selection: document the label filter and priority order for agents; test the matching code

`tuh-01KZCMF7JKMXVDG0HANVVQ05FN`

- **Status:** done
- **Priority:** 0
- **Labels:** `docs` `mcp` `go` `protocol`
- **Created:** 2026-08-06 22:52 UTC by `brandon/claude-code-1`

## Description

Context: spun out of the claim_next-discovery capture (tuh-01KZA0VT234XJYVZWT8EXV78J5, cancelled at the 2026-08-06 triage grill). The label filter shipped in B9 (2026-07-29) but is invisible and unverified: docs/agent-protocol.md never tells agents the input exists (its only gesture is line ~39's "nothing matches your labels"), and no test anywhere passes labels to claim_next on either the MCP or HTTP path — hasAllLabels has zero coverage. Priority order is settled in code but stated only in a JSON-schema string (ops.go ~64 "higher claims first"). This is product documentation for any repo that installs tuhdoo, not this-repo convention.

The ask, one PR: (1) A short paragraph in docs/agent-protocol.md (in or near the loop's step 1, where claim_next is introduced) documenting selection as the product defines it: highest priority first (higher number wins; 0 default), ULID/creation-order tie-break, the same order get_backlog's ready section shows; the optional labels input on claim_next, matched all-of — every requested label must be on the task, a task carrying extra labels still matches, a task with no labels never matches a labelled claim, omitting labels takes the best ready task; and that claimed:false does not distinguish an empty pool from the filter excluding everything — re-call without labels to tell. Revision header notes the change and this grill. (2) Tests: a table-driven unit test for hasAllLabels (internal/daemon/ops.go ~1373) covering all-of matching, extra-labels-still-match, unlabelled-task-excluded, and empty-request-matches-all; plus one end-to-end test passing labels through claim_next (mirror daemon_test.go TestClaimLifecycle's shape) asserting a labelled claim skips a higher-priority non-matching task and serves the matching one, and returns claimed:false/409 when nothing carries the labels though ready work exists.

Acceptance: the protocol-doc paragraph exists with exactly the semantics above, written product-generically; both tests exist and pass; make test lint green.

Pointers: internal/daemon/ops.go hasAllLabels (~1373) and opClaimNext (~309); internal/core/state.go ReadyTasks (~307, the single ordering source); internal/daemon/mcp.go claimNextInput (~333) and the claim_next registration (~446); internal/daemon/daemon_test.go TestClaimLifecycle (priority-order end-to-end pattern); docs/agent-protocol.md ~39.

Constraints: docs and tests only — no behavior change. The claimed:false reason string stays as-is: distinguishing empty-pool from filter-miss was considered and rejected at the 2026-08-06 grill (agents can re-call without labels; not worth new surface). Product/dogfood separation (Brandon, 2026-08-06): the protocol doc describes mechanism only, value-agnostically — D5's labels-grill revision (001 ~68) makes label taxonomies the installing repo's to invent, so any example labels must read as clearly illustrative, never this repo's taxonomy as norm. No new MCP surface (T5).

## History

### 2026-08-06 23:57 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-05fn/claim-next-selection-docs`
- PR: <https://github.com/brandonbews/tuhdoo/pull/49>
- Commits: `2e5ab33`
- Merged as: `c7582f56235a83d5fea2c583b3714946426af49a`

Selection paragraph added under agent-protocol loop step 1 (priority order with ULID tie-break matching get_backlog ready order; all-of label filter semantics; claimed:false ambiguity + re-call-without-labels disambiguation), semantics verified against ReadyTasks/hasAllLabels/opClaimNext before writing; example labels are generic placeholders per the product/dogfood separation rule. New table-driven TestHasAllLabels (internal/daemon/ops_test.go, new file — package had no ops unit-test file) and end-to-end TestClaimNextLabelFilter (daemon_test.go, HTTP path mirroring TestClaimLifecycle). No behavior change; reason string untouched per the grill decision. Squash-merged via PR #49. Docs+tests only — no deploy needed.
