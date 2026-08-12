# Agent protocol slim-down: de-bloat the protocol doc and its supplementary material (launch gate)

`tuh-01KZSBC7K0GNYNYTTAM6DW7Q0M`

- **Status:** done
- **Priority:** 1
- **Labels:** `docs` `protocol` `launch`
- **Created:** 2026-08-11 21:24 UTC by `brandon`

## Description

Context: Brandon's verdict 2026-08-11: the agent protocol "seems very bloated and over specified and all of the supplementary information around it (especially in the docs) seems confusing and over the top." A launch gate per the 2026-08-11 announcement grill; the what/why/how was then grilled properly with Brandon later the same day — the decisions below are his, do not re-litigate.

Diagnosis (from reading the doc at the grill): `docs/agent-protocol.md` (141 lines) is three layers. (1) Provenance scaffolding: the ~350-word "Status" changelog paragraph at the top chronicling nine revision rounds; ~20 inline revision parentheticals ("*(revised 2026-07-30…)*", "*(added 2026-08-04, `001` D6)*"); the 12-line B11 field-test record; internal references (001/002, B-numbers, Cycle-4, PR numbers, task IDs) meaningless to adopters. (2) War-story-length failure lore: the shim-death paragraphs, the B12 anecdote inside the no-attempt-no-escalation rule, the Cycle-4 cautionary tale. (3) The actual protocol rules — field-tested, every rule earned.

The decisions (2026-08-11 grill):
- **Cut depth:** strip layer 1 outright; compress layer 2 into imperative rules (e.g. shim deaths become "if you see X on stderr: reconnect before doing anything else"); keep every layer-3 semantic rule identical in meaning. Roughly a halving.
- **Register:** one artifact written as a tight agent prompt — imperative, second-person, zero throat-clearing. Humans read the same file; no human/agent split.
- **History:** git-only. The changelog and revision notes are deleted, not relocated — no appendix, no internal-docs migration; design docs keep their own revision notes where the decisions were made.
- **Dedupe:** protocol restatements in other docs (`steering.md` and `recipes/trunk-based-pr-flow.md` are the main offenders) become pointers at the protocol doc; each rule lives in exactly one place.

Folded in from the 2026-08-11 escalations grill (tuh-01KZPYFK8GFXQ5T2GFPCRQ8E86, Brandon's decision): the escalation section's rewrite must sharpen the writing doctrine — an escalation's **question field carries the whole decision package**: the question, the options the agent sees, and a recommendation, all kept short; the **context field is background only**, the minimum a human needs to answer, never the lead. Give the `escalate` tool description in `internal/daemon/mcp.go` the same emphasis (this is within the keep-aligned constraint below, not a redesign). Rationale: the TUI will render the question block always-visible and collapse context by default — the sibling TUI task holds that half; no dependency edge either way.

Acceptance:
- `docs/agent-protocol.md` rewritten per the decisions above; no semantic rule lost or weakened — if unsure mid-rewrite whether a passage is semantics or lore, escalate rather than cut.
- The doc is embedded in the binary (`tuhdoo protocol` prints it verbatim), so editing the file is the whole change there; update any tests asserting on its output.
- Other docs' protocol restatements deduped to pointers; their own concerns (steering, PR flow) untouched.
- Brandon's PR review is the final bar. `make test lint` green; one PR.

Constraints: T5's twelve verbs untouched — prose only, never surface. MCP tool descriptions in `internal/daemon/mcp.go` were deliberately aligned with the notes doctrine (t-01KYVD31CNTR1EVCDHPJGSQAGH); keep them consistent with the slimmed wording but do not redesign them. `docs/` publishing rules apply (GFM, frontmatter title + description, relative links).

Sequencing: lands BEFORE the copy-tightening pass (tuh-01KZSBDXFZCRNEDY7DMD4XGP75 depends on this task) — structural cuts first so the copy pass never polishes text about to be deleted.

## History

### 2026-08-11 22:22 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · priority 0→1 · labels +docs +protocol +launch

### 2026-08-11 22:35 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-08-11 22:53 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-08-11 23:42 UTC — run by `brandon/claude-code-4` — done

- Branch: `tuh-7q0m/agent-protocol-slim-down`
- PR: <https://github.com/brandonbews/tuhdoo/pull/67>
- Merged as: `d2905f47a6f208734ce70b47407bee8880bdd613`

docs/agent-protocol.md rewritten per the 2026-08-11 grill decisions: layer 1 (Status changelog, revision parentheticals, B11 field-test record, internal references) deleted; layer 2 lore compressed to imperative rules; all semantic rules kept with identical meaning (verified via a 54-item rule inventory). Escalation section leads with the question-carries-the-decision-package / context-is-background doctrine; the escalate tool description and its question/context field schema strings in internal/daemon/mcp.go carry the same emphasis (no other tool description touched). steering.md and recipes/trunk-based-pr-flow.md protocol restatements deduped to pointers; the recipe's paste-able agent-instructions template deliberately untouched. No test changes needed (embed tests assert on stable headings/anchors, all kept; loop numbering unchanged so ops.go/api.go "step 5" comments stay accurate). Two judgment calls flagged in the PR body for Brandon: shim stdin-death lore cut (fifo example, replays-cleanly detail — one clause restores it if wanted), and the cut is ~35% by words rather than a strict halving because layers 1-2 are fully gone and further cuts would remove rule content. make test lint green; merged via PR #67, squash commit d2905f4. Unblocks the copy-tightening pass tuh-01KZSBDXFZCRNEDY7DMD4XGP75.
