# 002 doc-drift sweep: T6 view-stamp path; T7 WAITING vocabulary, parent remnants, shelf bolding

`tuh-01KZVZT7F8CVJYX1P009AJJ4D9`

- **Status:** open — in progress, claimed by `brandon/claude-code-2`
- **Priority:** none
- **Labels:** `docs` `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit findings, re-verified 2026-08-27 at the audit-findings triage; the drift got WORSE since the audit (PR #82 added text reinforcing two of the stale claims). Folds in tuh-01KZVZT7F8CVJYX1P000JQJH5J (T6 stamp drift), cancelled as folded-in.

The ask: revise internal-docs/design/002-technology.md in place with dated revision notes, per the repo convention (the D5/D6/D8 pattern in 001). Four fixes:
1. T6 view stamp (002:127): documented at views/.meta; actually root-level .views-meta.json because views render at the branch root — internal/views/views.go:51-54 comment records the divergence, T6 never caught up. One-line fix + note.
2. T7 one-shot WAITING vocabulary (002:142): the contract enumerates dep:<id> / esc:<id> only; the shipped golden-pinned vocabulary also has the leading `cyclic,` marker and the `dep:<id>:cancelled` suffix (cmd/tuhdoo/snapshot.go waitingOn ~110-138; oneshot_golden_test.go:94-96,116). The SEMANTICS are already recorded at 001:70 (D5 edge grill) — what T7 is missing is the token enumeration; fold it in.
3. T7 parent remnants (002:146, 002:148): the `· in <parent>` edge marker and `--parents` in the fragment-resolver list survive the 2026-08-05 edge grill that deleted parents — edgeText (cmd/tuhdoo/top.go ~1767) renders dep counts only, no --parents flag exists anywhere, and 002:96 itself records the removal, so the doc self-contradicts. Delete the remnants with a note.
4. T7 shelf rows (002:154): "both sections' rows stay dim" — shipped rows are dim id/badge + BOLD title (2026-07-31 visual-hierarchy decision, recorded only in the top.go gridRow comment ~1697-1704; pinned by top_golden_test.go:242). Record the 2026-07-31 decision with its date, and reconcile 002:156 (added by #82: "shelf rows are dim by design"), which reinforces the stale framing.

Acceptance: 002 revised in place, each fix carrying a dated revision note; no code changes; make test lint green. One PR.

Constraints: decision-record posture — append revision notes, never rewrite the original text's history. Do NOT fold in the selection-bar-ladder recording question (tuh-01KZVZT7F8CVJYX1P00B2NE8GX): that one is gated on the internal-docs tiering grill (tuh-01M10ZA2VCJ59WWYZG58RXHV8A). This task fixes statements that are WRONG; where decisions live is the grill's question.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +docs
