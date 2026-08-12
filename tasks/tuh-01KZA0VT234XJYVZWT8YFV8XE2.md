# Monorepo grain: is one tuhdoo branch per repo right when a repo hosts many projects?

`tuh-01KZA0VT234XJYVZWT8YFV8XE2`

- **Status:** on hold — deliberately paused
- **Priority:** 0
- **Labels:** `design`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Gated: unpark for a grill once site/ work has actually been steered from this backlog long enough to judge the grain — this capture unparks alongside (or shortly after) the marketing-site task (tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2), whose monorepo site/ decision is the designated first live test of one-branch-per-repo. Held at the 2026-08-06 grill (Brandon); the site is now high-priority for him, so this gate is expected to be short-lived, but the grain question is answerable only with mileage, not urgency.

Migrated from open-questions.md (onboarding thread), 2026-08-05 sweep. The question: is one tuhdoo branch per repo right when a repo hosts many projects?

For the future grill, recorded 2026-08-06:
- Friction signals to watch during the site/ dogfood: does a single priority space serve two unlike projects; do labels keep site vs core work legible (labels are agnostic decoration per D5 — no mechanics without a D5 revision); does claim_next hand a Go-focused agent a CSS task despite the label filter (claim_next's all-of labels input has existed since B9 — its docs/tests are tuh-01KZCMF7JKMXVDG0HANVVQ05FN; the adjacent claim_next-discovery capture tuh-01KZA0VT234XJYVZWT8EXV78J5 was cancelled 2026-08-06 with affinity hints deliberately dropped — if routing friction shows up here, capture it fresh with the evidence rather than reviving affinity from speculation).
- Cost asymmetry to weigh going in: multiple-fabrics-per-repo is architecture work, not config — the data branch name is a compile-time constant (store.DefaultRef), and each fabric would need its own daemon socket under .git/tuhdoo. This raises the bar for declaring the one-branch grain broken.
- The other side of the coin: the multi-repo question (many repos, one plan) was cancelled 2026-08-06 as out-of-scope-as-mechanism — see tuh-01KZA0VT234XJYVZWT91KSJGJR for that reasoning; this capture owns only the within-one-repo grain.

## History

### 2026-08-06 22:15 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→held

### 2026-08-06 22:53 UTC — edit by `brandon/claude-code-1`

description edited
