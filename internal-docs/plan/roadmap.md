# Roadmap

Phases map to the surface/build-order decisions in `001` D8 and `002` T7. Each phase has a definition of done; a phase isn't done because its code exists — it's done when its *proof* holds. The file carries the live phase and the done-declarations of finished ones — it does not pre-schedule phases beyond the live one; consciously deferred features live beside the decisions that deferred them in `001`/`002`. The Ideas icebox at the bottom is the one non-phase section: might-be-cool notions kept off the ledger on purpose. *(Trimmed 2026-08-07, roadmap grill: done-phase history compressed to its proof record, the v2+ section removed as duplication of `001` D4/D7/D8 and `002` T2/T4/T7 — full text in git history.)*

## v0 — The loop, dogfoodable *(declared done 2026-08-03)*

**Definition of done:** tuhdoo has managed its own development, proven by all five:

1. the backlog lives on the data branch and `internal-docs/plan/backlog.md` (at the time, `docs/plan/backlog.md`) is a tombstone;
2. every commit on `refs/heads/tuhdoo` is daemon-authored — the branch has never been repaired by hand;
3. at least one event-schema version bump has landed on the live branch and replayed correctly;
4. agents have driven the full loop — `claim_next` → work → `finish_run` / `escalate` — with no direct git writes to the data branch;
5. the daemon has been restarted mid-session repeatedly with no lost or corrupted events.

## v1 — Steering, and a second machine *(live)*

**Scope:** the TUI (grown from `watch` by adding input handling: answer escalations, reprioritize, cancel) plus the first real multi-machine usage — two machines, one remote, real claim races resolved by the D6 machinery.

**Definition of done:**

1. a blocking escalation is raised by an agent, answered from the TUI, and picked up by a successor agent without the human touching git;
2. two daemons against one remote produce observed claim collisions with one winner each, a `superseded` run recorded for every loser, and byte-identical replayed state and views on both sides afterward — with the `superseded` runs written by the real D6 machinery (coercion and expiry synthesis), not by the harness playing the loser, and a confirmation-race storm in which every task lands exactly one `claim.confirmed` (a duplicate confirmation is a hard failure) *(clause extended 2026-08-04, confirmation-gate grill — `001` D6)*;
3. Brandon's 5-person work team could be onboarded with `tuhdoo init` + docs alone (whether or not they are).

*(Clause 2 rewritten 2026-08-03 by the milestone grill — supersedes "two machines run fleets against the same remote for a week with collision counts logged and zero divergent state." Same disease as v0's week, plus a measurement problem: `syncer.Status.Collisions` counts non-fast-forward pushes, not claims voided by the D6 winner rule, so "collision counts logged" would have recorded the wrong quantity. The rewritten clause names the three facts that actually prove convergence, and is satisfiable by a deliberate collision harness in an afternoon rather than by waiting for incidental races across a week — which matters, because as of the rewrite date the data branch carried 369 commits and **zero merge commits** (it has since passed 500, still merge-free): single-machine dogfooding structurally cannot exercise the D3 set-union merge path. *(Update 2026-08-05: the two-root union merge now runs against real daemons in harness tests — the clone-join work, PR #38, proved the simultaneous-init race convergent — but the live branch itself has still never carried a merge commit.)* Clause 3 stays a judgment call on purpose; mechanizing it would swap in a proxy, which is the failure this rewrite exists to undo.)*

## Ideas

An icebox (added 2026-08-07, Brandon): things that might be cool someday, deliberately kept off the ledger so they cost zero attention — no gates, no owners, no commitments, and agents never work from this list. Picking one up later means a fresh capture with real evidence, not a revival of its line here.

- **Escalation delivery when the TUI is closed** (notifications). If ever wanted, the shape is already settled — a 2026-08-06 grill decided on a generic on-escalation exec hook, rejected baked-in OS notifications outright, and left one named open question; the full record lives on cancelled task `tuh-01KZA0VT234XJYVZWT8K2D75W9`.
