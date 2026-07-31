# Backlog

## Ready

| ID | Task | Priority | Labels |
|---|---|---:|---|
| [t-01KYRMFV10W1N28TCN62RR3A4D](tasks/t-01KYRMFV10W1N28TCN62RR3A4D.md) | Daemon portability: unix-only lock and the socket-path length limit | 0 | go, platform, parked |
| [t-01KYT63MB28Z535SMJCBC7SY1P](tasks/t-01KYT63MB28Z535SMJCBC7SY1P.md) | Tree/parent-grouped rendering in the TUI list | 0 | cli, tui, design |

## In progress

_None._

## Blocked / waiting

| ID | Task | Priority | Waiting on |
|---|---|---:|---|
| [t-01KYRMFV10W1N28TCN5NWAGSW5](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) | v0 definition of done: the dogfood week holds | 0 | escalation: v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06. The bar (docs/plan/roadmap.md, v0): one week of tuhdoo managing its own development — sessions driven through claim_next, watch running beside agents — with zero manual repair of the data branch. Answer "yes" to release this milestone (then mark it done), or describe what broke. |
| [t-01KYRMFV10W1N28TCN5SH4QM7A](tasks/t-01KYRMFV10W1N28TCN5SH4QM7A.md) | v1 milestone: steering surface and a second machine | 0 | depends on [t-01KYRMFV10W1N28TCN5NWAGSW5](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) |
| [t-01KYRMFV10W1N28TCN5WVTCB1J](tasks/t-01KYRMFV10W1N28TCN5WVTCB1J.md) | Two-machine dogfood: real claim races over one remote | 2 | escalation: This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week, and do you want a prep task filed first for sync-latency instrumentation? |
| [t-01KYRMFV10W1N28TCN62F6FRTH](tasks/t-01KYRMFV10W1N28TCN62F6FRTH.md) | Epoch compaction (D9): snapshot event + in-commit deletion | 1 | depends on [t-01KYRMFV10W1N28TCN5SH4QM7A](tasks/t-01KYRMFV10W1N28TCN5SH4QM7A.md) |
| [t-01KYRVCBE83KT62BAE1502VV29](tasks/t-01KYRVCBE83KT62BAE1502VV29.md) | npm devDependency distribution (esbuild-pattern wrapper packages) | 1 | escalation: npm publishing needs credentials only you can provision. Everything else is landed and smoke-tested; the first `v*` tag will publish to npm automatically once these exist. Needed: (1) an npmjs.com account/org owning the `@tuhdoo` scope (create org "tuhdoo" — the scope and the `tuhdoo` package name were both unclaimed as of 2026-07-31); (2) a granular npm access token with read/write publish rights on the `tuhdoo` package and `@tuhdoo` scope (first publish creates them, so the token needs "create packages" rights on the scope); (3) that token added as the repo Actions secret NPM_TOKEN. Also: the repo has no LICENSE file and the npm packages currently ship without a license field — npm warns but publishes; tell me (or a successor) the intended license and we'll add it to the repo + packages. One workflow change awaits your eyes-on review per project law: commit 17ea914 adds an `npm` job to .github/workflows/release.yml (downloads release assets, assembles packages with npm/prepare.js, publishes with NPM_TOKEN via ${NODE_AUTH_TOKEN} in .npmrc; no new third-party actions). Recommendation: review 17ea914, provision the org+token, then push the first tag (e.g. v0.1.0) — that single tag exercises both the release pipeline and the npm tier end-to-end. |

## Done

- [t-01KYRMFV10W1N28TCN5TDQC7KM](tasks/t-01KYRMFV10W1N28TCN5TDQC7KM.md) — Grow watch into the interactive steering TUI (tuhdoo top)
- [t-01KYRMFV10W1N28TCN5ZZ9Z2C1](tasks/t-01KYRMFV10W1N28TCN5ZZ9Z2C1.md) — Retire full-replay-per-write and the grow-forever event overlay
- [t-01KYRMVT1YC2929WSQ3W6YHHZM](tasks/t-01KYRMVT1YC2929WSQ3W6YHHZM.md) — Render markdown views on local daemon writes
- [t-01KYRR78YKX9YHZE6W6B798X4G](tasks/t-01KYRR78YKX9YHZE6W6B798X4G.md) — Auto-derive session principals: git identity + daemon-minted agent names
- [t-01KYRVCBE83KT62BAE11W3TAM8](tasks/t-01KYRVCBE83KT62BAE11W3TAM8.md) — Release pipeline: tagged, cross-compiled binaries
- [t-01KYT63MB28Z535SMJC9B0D83W](tasks/t-01KYT63MB28Z535SMJC9B0D83W.md) — One TUI: bare tuhdoo is the interactive surface (Cycle 4)
- [t-01KYT63MB28Z535SMJCA63RQJM](tasks/t-01KYT63MB28Z535SMJCA63RQJM.md) — Arm the TUI detail screen: selectable escalation, enter to answer; p/c on the viewed task
- [t-01KYT80CP4JKAM3V2C4DNGF1Y3](tasks/t-01KYT80CP4JKAM3V2C4DNGF1Y3.md) — TUI readability: short display IDs, width-aware wrapping, list scrolling
- [t-01KYTQTEQYGXVQ0QY7F95CHXVV](tasks/t-01KYTQTEQYGXVQ0QY7F95CHXVV.md) — Principal identity override: stop deriving ugly actors from noreply emails
- [t-01KYTQTEQYGXVQ0QY7FCTAZ3G6](tasks/t-01KYTQTEQYGXVQ0QY7FCTAZ3G6.md) — Agent protocol: dangling-pointer anti-pattern; notes are garnish, transitions are the record
- [t-01KYTRJQT44HGYXGR9C7C3R2GS](tasks/t-01KYTRJQT44HGYXGR9C7C3R2GS.md) — Escalation ergonomics: resolve answered-out-of-band, and the name itself
- [t-01KYTSQDQJWM8YQQ8FWMHBZ5DW](tasks/t-01KYTSQDQJWM8YQQ8FWMHBZ5DW.md) — Short IDs are the human contract: display everywhere, accept as input, annotate edges
- [t-01KYVD31CNTR1EVCDHPC5973KW](tasks/t-01KYVD31CNTR1EVCDHPC5973KW.md) — Needs Input: enter answers in place; blocked rows stop repeating the question
- [t-01KYVD31CNTR1EVCDHPG0G4GMZ](tasks/t-01KYVD31CNTR1EVCDHPG0G4GMZ.md) — TUI navigation: up/down arrows move the cursor; footer says so
- [t-01KYVD31CNTR1EVCDHPGZFQ5EV](tasks/t-01KYVD31CNTR1EVCDHPGZFQ5EV.md) — Rename the cancel interaction: archive as the human verb, task.cancelled stays the plumbing
- [t-01KYVD31CNTR1EVCDHPHJEV9VK](tasks/t-01KYVD31CNTR1EVCDHPHJEV9VK.md) — TUI mouse support: click selects, click again acts as enter
- [t-01KYVD31CNTR1EVCDHPJGSQAGH](tasks/t-01KYVD31CNTR1EVCDHPJGSQAGH.md) — Align MCP tool descriptions with the revised notes doctrine
- [t-01KYVE848CJZNG5VFWZ9J3WRKM](tasks/t-01KYVE848CJZNG5VFWZ9J3WRKM.md) — Brand the task IDs: mint tuh-, accept both prefixes, age out t-
- [t-01KYVEXK2BX040KJ244S2WP213](tasks/t-01KYVEXK2BX040KJ244S2WP213.md) — CLI write verbs: a paved path when no MCP session exists
- [t-01KYVJ2607S5S390CVYSF3PVG4](tasks/t-01KYVJ2607S5S390CVYSF3PVG4.md) — TUI dashboard visual redesign: section bars + fixed column grid (mock-a)
- [t-01KYVJ5NABEKX72KNE006MBBMC](tasks/t-01KYVJ5NABEKX72KNE006MBBMC.md) — Inbox and held: capture without scoping pressure, pause without pretending
- [t-01KYVMD4PS9NMQVP1K5HQ8769X](tasks/t-01KYVMD4PS9NMQVP1K5HQ8769X.md) — finish_run accepts a claimless finish: opFinishRun never checks holdership
- [t-01KYVMD4PS9NMQVP1K5M1PVRD8](tasks/t-01KYVMD4PS9NMQVP1K5M1PVRD8.md) — Shim died once: 'stdio session: invalid trailing data at the end of stream'

## Cancelled

_None._
