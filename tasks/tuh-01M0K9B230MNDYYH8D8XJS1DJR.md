# Configurable daemon commit identity (Options.Ident is unwired) — held until a non-Vercel author check bites

`tuh-01M0K9B230MNDYYH8D8XJS1DJR`

- **Status:** on hold — deliberately paused
- **Priority:** none
- **Labels:** `design` `daemon`
- **Created:** 2026-08-21 23:08 UTC by `brandon/claude-code-1`

## Description

Split from the Vercel adopter report (tuh-01M0HF5SS536W9JAS2CB2ZQCT8) at the 2026-08-21 triage grill (Brandon). Held with an explicit unpark trigger — do not build from this capture without a /grill-me cycle first.

Context: the daemon commits to the data branch as daemon@tuhdoo.invalid (hardcoded default, internal/daemon/daemon.go:69). daemon.Options.Ident exists (daemon.go:66-74) but no CLI or config path wires it. Host commit-author authorization checks (Vercel's confirmed live, 2026-08-20/21) can therefore never pass for ledger pushes.

Why this is NOT the Vercel fix: for Vercel the docs/recipe path (vercel.json with git.deploymentEnabled false for the data branch, committed on the data branch) silences deploys entirely. Making the author check PASS would instead authorize every ledger push to start a build that the Ignored Build Step then cancels — deploy attempts multiply rather than disappear. This mechanism's value is elsewhere: other hosts' author checks, or attribution honesty (per-machine/per-user commit authorship).

Design territory (D7 / identity): what does commit authorship mean when events already carry principals? Candidate default: repo git config user.email. The grill must also decide whether daemon@tuhdoo.invalid's honesty ('a daemon wrote this commit') is worth keeping as the default.

Unpark when: a non-Vercel host author-check bites a real adopter (captured fresh with evidence, per the D1 re-affirmation pattern — 001-core-design.md:54), or genuine attribution pressure appears.

## History

_No activity yet._
