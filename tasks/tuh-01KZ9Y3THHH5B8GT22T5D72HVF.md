# Doc-sync sweep: align docs with code and the 2026-08-05 release grill; tombstone open-questions into the ledger

`tuh-01KZ9Y3THHH5B8GT22T5D72HVF`

- **Status:** done
- **Priority:** 2
- **Labels:** `docs`
- **Depends on:** [`tuh-wzrg`](tuh-01KZ9Y3THHH5B8GT22SY3FWZRG.md) (done)
- **Created:** 2026-08-05 21:43 UTC by `brandon/claude-code-1`

## Description

Context: the 2026-08-05 pre-v0.2.0 audit found ~15 doc-vs-doc and doc-vs-code disagreements, and the release grill made decisions that need design-doc revision notes. Brandon's standing decision from that grill: design docs are the agent-facing decision record only — anything tracking work or open questions moves into the tuhdoo ledger. Depends on the parents-removal task so docs describe shipped reality.

The ask — every numbered item below:
1. CLAUDE.md: the project law says "eleven tools (T5)" — it is twelve (confirm_claim, 2026-08-04). Also note v0's DoD was declared done 2026-08-03 and v1 is the live phase.
2. docs/README.md (worst file): rewrite post-B12 — agent-protocol.md is field-tested and revised (not "draft; field-test pending B9"), backlog.md migrated six days ago (not future tense), twelve verbs.
3. docs/plan/roadmap.md: line 7 still says "still-running dogfood week" (retired at :19 on 2026-08-03); the v0 Ships list includes tombstoned `watch` and omits create/update/answer/mcp/daemon/version; refresh or drop the "369 commits" aside at :33.
4. docs/plan/backlog.md tombstone: retire "The v0 clock" section (:17-23) — superseded DoD, and its escalation-as-fence pattern is now documented as the wrong fence (agent-protocol "no attempt, no escalation").
5. docs/design/002 T5: the two "verb count stays eleven" paragraphs (:100, :102) and "all ten write-side steering actions" (:102) — reword as dated history consistent with the twelve-tool header; fix the T3 envelope example showing v:1 for task.created (moved to v2 on 2026-07-31).
6. 002 T5 "The ledger never deletes; cancelled is the archive" (:104): the archive vocabulary was retired 2026-08-01, and D9 compaction does delete files in-commit — reword to match both ("append-only tree semantics, not tree contents").
7. 001 and 002 headers: revision lines stop at Cycle 2 while bodies carry revisions through 2026-08-05 — fix both.
8. 001 D6 + 002 T8 confirmation-rule contradiction: write the two-layer story — the writers' CAS invariant guarantees at most one confirmation per contest; replay additionally carries a defensive earliest-confirmation rule as fail-safe determinism (internal/core/replay.go:310-342 is the reference).
9. 001 D5 revision note (edge grill 2026-08-05): parents removed, epics are depends_on containers; loop posture = reject at edit + detect-and-mark at replay, never claimed as prevention; cancelled deps block dependents loudly; dangling dep counts as met (defensive). 002 T5 revision note: create_task/update_task drop the parents parameter (verb count unchanged).
10. 001 D5 / 002 T7 status-count wording: five stored statuses vs derived display words (ready/in_progress/blocked) — state the stored-vs-derived distinction once, explicitly.
11. Root README.md: name the data branch (`tuhdoo`), reference the latest release rather than a pinned stale v0.1.0 example, mention the TUI --watch mode and the create/update/answer write commands, include the MCP harness snippet.
12. docs/design/open-questions.md becomes a tombstone (backlog.md B12 pattern): each still-live thread becomes a ledger task and the tombstone names where each went. Settled, do NOT migrate — record instead: name check (tuhdoo affirmed 2026-08-05, Brandon owns tuhdoo.com), milestone semantics (2026-08-03), edge semantics (2026-08-05), agent-protocol doc (delivered). Live, migrate as inbox (or held where an unpark gate exists): task-description conventions; claim_next discovery/filters (note the label-agnosticism constraint from D5 2026-08-05); salvage flow for superseded/interrupted runs; escalation delivery when the TUI is closed; run↔code linkage robustness; plan-materialization flow; init UX remainder (teammate joining flow, branch-protection guidance) minus what the clone-join and init-hardening tasks already cover; repo-hosting edge cases (shallow/single-branch/forks/mirrors); monorepo grain; multi-repo story; uninstall story; compaction triggers in practice (note the existing blocked D9 task); working-set retirement. The v2+ parked set (intake bridge, signing, kanban, view templates, webhook fetch, supervisor, read-only sharing) is already gated in roadmap v2+ — one held pointer task is enough.

Acceptance: every numbered item addressed in one docs PR; open-questions.md is a tombstone with task IDs; migrated tasks follow the tasks-are-prompts convention (inbox captures may be leaner — the bar applies at promotion); no decision changed silently — only dated revision notes recording this grill; `make test lint` green.

Constraints: edit design docs in place with dated revision notes (the Cycle-2 pattern); do not renumber or rewrite history beyond the listed fixes; one PR.

## History

### 2026-08-05 22:41 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-2hvf/doc-sync-sweep`
- PR: <https://github.com/brandonbews/tuhdoo/pull/40>

Landed via PR #40 (squash-merged, checks green). All twelve sweep items done: CLAUDE.md law now says twelve tools and v1 is the live phase; docs/README.md rewritten post-B12; roadmap and backlog.md staleness fixed; 002 verb-count history reworded, envelope example at v2, no-delete wording reconciled with D9; both headers carry the revisions-through-2026-08-05 line; 001 D6 has the two-layer confirmation note (writers CAS invariant = the guarantee, replay earliest-confirmation = defensive backstop) with T8 aligned; 001 D5 has the edge-grill revision (parents removed, loop posture detection-never-prevention, cancelled deps block loudly, dangling deps met); stored-vs-derived status vocabulary stated in D5 and T7; root README refreshed (data branch named, un-stale-able release link, MCP snippet). open-questions.md is a tombstone: 5 settled threads recorded, 14 migrated to ledger tasks (IDs in the file; created this session as inbox + one held pointer). Design docs are now the decision record only; open questions live on the ledger.
