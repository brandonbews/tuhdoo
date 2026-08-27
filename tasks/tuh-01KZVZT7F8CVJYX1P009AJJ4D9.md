# 002 doc-drift sweep: T6 view-stamp path; T7 WAITING vocabulary, parent remnants, shelf bolding

`tuh-01KZVZT7F8CVJYX1P009AJJ4D9`

- **Status:** done
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

### 2026-08-27 08:47 UTC — run by `brandon/claude-code-2` — done

- Branch: `tuh-j4d9/002-drift-sweep`
- PR: <https://github.com/brandonbews/tuhdoo/pull/98>
- Merged as: `c830ea6`

Landed via PR #98 (squash c830ea6). Four dated revision notes in 002, each verified against shipped code first: (1) T6 view stamp corrected to root-level .views-meta.json (views/.meta was never implemented — checked git history to B8); (2) T7 WAITING enumeration gains the golden-pinned cyclic marker and dep:<id>:cancelled suffix, pointing at 001 D5 for semantics; (3) the parent remnants (· in <parent> marker, --parents resolver entry) deleted — repo-wide grep confirms no such flag; (4) shelf notes record the 2026-07-31 bold-titles decision and also the 2026-08-25 ramp-badges-everywhere reversal, because amending the PR #82 sentence while asserting held-badges-stay-dim would have written a false note (golden at top_golden_test.go:245-247 pins the ramp-colored held badge). No code changes; make test lint green. Doc-only — no daemon deploy needed.
