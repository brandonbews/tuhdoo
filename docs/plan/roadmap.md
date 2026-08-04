# Roadmap

Phases map to the surface/build-order decisions in `001` D8 and `002` T7. Each phase has a definition of done; a phase isn't done because its code exists — it's done when its *proof* holds.

## v0 — The loop, dogfoodable

*(2026-07-30, Cycle 4: `watch` was folded into the verb-less TUI — its pane is now `tuhdoo --watch`. The scope and DoD below describe v0 as shipped; read `tuhdoo watch` as `tuhdoo --watch` for the still-running dogfood week.)*

**Scope:** daemon + MCP + CLI portal. No interactivity beyond read-only `watch`.

**Definition of done:** tuhdoo has managed its own development, proven by all five:

1. the backlog lives on the data branch and `docs/plan/backlog.md` is a tombstone;
2. every commit on `refs/heads/tuhdoo` is daemon-authored — the branch has never been repaired by hand;
3. at least one event-schema version bump has landed on the live branch and replayed correctly;
4. agents have driven the full loop — `claim_next` → work → `finish_run` / `escalate` — with no direct git writes to the data branch;
5. the daemon has been restarted mid-session repeatedly with no lost or corrupted events.

*(Rewritten 2026-08-03 by the milestone grill — supersedes "a full week of real use produces no manual repair of the data branch." The week was a proxy for "enough varied use to expose bugs," and it did not survive contact with the dogfood: the binary changed every few minutes, so a strict reading reset the clock on every deploy and the criterion was unsatisfiable for as long as development continued. The clause that was actually load-bearing — no manual repair — survives as (2), and is mechanically checkable. Points (3) and (5) record what the rapid iteration bought that a quiet week would not have: a live schema bump and dozens of mid-claim restarts are harsher than the week ever intended. Every point is checkable by any actor, which is the standard the milestone's blocking escalation failed — see the fence rule in `docs/agent-protocol.md`.)*

**Ships:** the Go binary (`tuhdoo`) containing: daemon (serialized writer, sync loop, replay engine, view builder), MCP over streamable HTTP + `tuhdoo mcp` stdio shim, JSON HTTP API, CLI (`init`, `status`, `backlog`, `task`, `escalations`, `watch`), the four markdown views, the agent protocol doc.

## v1 — Steering, and a second machine

**Scope:** the TUI (grown from `watch` by adding input handling: answer escalations, reprioritize, cancel) plus the first real multi-machine usage — two machines, one remote, real claim races resolved by the D6 machinery. *(2026-07-30, Cycle 4: the TUI shipped verb-less — bare `tuhdoo`, `--watch` for the disarmed mode; `top` was a transient name. Details in `002` T7.)*

**Definition of done:**

1. a blocking escalation is raised by an agent, answered from the TUI, and picked up by a successor agent without the human touching git;
2. two daemons against one remote produce observed claim collisions with one winner each, a `superseded` run recorded for every loser, and byte-identical replayed state and views on both sides afterward — with the `superseded` runs written by the real D6 machinery (coercion and expiry synthesis), not by the harness playing the loser, and a confirmation-race storm in which every task lands exactly one `claim.confirmed` (a duplicate confirmation is a hard failure) *(clause extended 2026-08-04, confirmation-gate grill — `001` D6)*;
3. Brandon's 5-person work team could be onboarded with `tuhdoo init` + docs alone (whether or not they are).

*(Clause 2 rewritten 2026-08-03 by the milestone grill — supersedes "two machines run fleets against the same remote for a week with collision counts logged and zero divergent state." Same disease as v0's week, plus a measurement problem: `syncer.Status.Collisions` counts non-fast-forward pushes, not claims voided by the D6 winner rule, so "collision counts logged" would have recorded the wrong quantity. The rewritten clause names the three facts that actually prove convergence, and is satisfiable by a deliberate collision harness in an afternoon rather than by waiting for incidental races across a week — which matters, because as of this date the data branch carries 369 commits and **zero merge commits**: the D3 set-union merge path has never executed outside unit tests, and single-machine dogfooding structurally cannot exercise it. Clause 3 stays a judgment call on purpose; mechanizing it would swap in a proxy, which is the failure this rewrite exists to undo.)*

## v2+ — Earned, not scheduled

Gated features, each with its unpark condition (`docs/design/open-questions.md`, Cycle 5):

- **Kanban/browser UI** — unparks only after the steering loop is proven in real team use (D8).
- **Public intake bridge** (host issues → events) — unparks on real OSS-maintainer demand; must remain an optional add-on (T2).
- **Event signing** — unparks if trust boundaries beyond repo-write appear (D7).
- **Per-machine supervisor / cross-project dashboard** — unparks when per-repo daemons demonstrably annoy (T4).
- **Custom view templates, webhook-driven fetch** — quality-of-life; unpark on demand.

Epoch compaction (D9) sits between v1 and v2: designed, but built only when a real data branch is big enough to need it — building it against synthetic data would be guessing.
