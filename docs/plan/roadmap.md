# Roadmap

Phases map to the surface/build-order decisions in `001` D8 and `002` T7. Each phase has a definition of done; a phase isn't done because its code exists — it's done when its *proof* holds.

## v0 — The loop, dogfoodable

*(2026-07-30, Cycle 4: `watch` was folded into the verb-less TUI — its pane is now `tuhdoo --watch`. The scope and DoD below describe v0 as shipped; read `tuhdoo watch` as `tuhdoo --watch` for the still-running dogfood week.)*

**Scope:** daemon + MCP + CLI portal. No interactivity beyond read-only `watch`.

**Definition of done:** tuhdoo manages its own remaining development. Concretely: this repo's `docs/plan/backlog.md` has been migrated into a live tuhdoo data branch; Brandon runs `tuhdoo watch` beside an agent session; the agent works exclusively through `claim_next` → work → `add_note` → `finish_run` / `escalate`; a full week of real use produces no manual repair of the data branch.

**Ships:** the Go binary (`tuhdoo`) containing: daemon (serialized writer, sync loop, replay engine, view builder), MCP over streamable HTTP + `tuhdoo mcp` stdio shim, JSON HTTP API, CLI (`init`, `status`, `backlog`, `task`, `escalations`, `watch`), the four markdown views, the agent protocol doc.

## v1 — Steering, and a second machine

**Scope:** the TUI (grown from `watch` by adding input handling: answer escalations, reprioritize, cancel/archive) plus the first real multi-machine usage — two machines, one remote, real claim races resolved by the D6 machinery. *(2026-07-30, Cycle 4: the TUI shipped verb-less — bare `tuhdoo`, `--watch` for the disarmed mode; `top` was a transient name. Details in `002` T7.)*

**Definition of done:** a blocking escalation is raised by an agent, answered from the TUI, and picked up by a successor agent without the human touching git; two machines run fleets against the same remote for a week with collision counts logged and zero divergent state; Brandon's 5-person work team could be onboarded with `tuhdoo init` + docs alone (whether or not they are).

## v2+ — Earned, not scheduled

Gated features, each with its unpark condition (`docs/design/open-questions.md`, Cycle 5):

- **Kanban/browser UI** — unparks only after the steering loop is proven in real team use (D8).
- **Public intake bridge** (host issues → events) — unparks on real OSS-maintainer demand; must remain an optional add-on (T2).
- **Event signing** — unparks if trust boundaries beyond repo-write appear (D7).
- **Per-machine supervisor / cross-project dashboard** — unparks when per-repo daemons demonstrably annoy (T4).
- **Custom view templates, webhook-driven fetch** — quality-of-life; unpark on demand.

Epoch compaction (D9) sits between v1 and v2: designed, but built only when a real data branch is big enough to need it — building it against synthetic data would be guessing.
