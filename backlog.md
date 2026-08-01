# Backlog

## Ready

| ID | Task | Priority | Labels |
|---|---|---:|---|
| [tuh-01KYXDWWM8S1GF6N9NE5FGA86Y](tasks/tuh-01KYXDWWM8S1GF6N9NE5FGA86Y.md) | Task view is the answering home: routed from Needs Input, dash-aligned design, selectable escalations, a to archive | 2 | cli, tui, ux, design |
| [tuh-01KYXE40ES9YSEGW9Z0GXKYPWW](tasks/tuh-01KYXE40ES9YSEGW9Z0GXKYPWW.md) | One real text-input widget: delineated box, fixed hint line, standard cursor editing | 1 | cli, tui, ux |

## In progress

_None._

## Blocked / waiting

| ID | Task | Priority | Waiting on |
|---|---|---:|---|
| [t-01KYRMFV10W1N28TCN5NWAGSW5](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) | v0 definition of done: the dogfood week holds | 0 | escalation: v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06. The bar (docs/plan/roadmap.md, v0): one week of tuhdoo managing its own development — sessions driven through claim_next, watch running beside agents — with zero manual repair of the data branch. Answer "yes" to release this milestone (then mark it done), or describe what broke. |
| [t-01KYRMFV10W1N28TCN5SH4QM7A](tasks/t-01KYRMFV10W1N28TCN5SH4QM7A.md) | v1 milestone: steering surface and a second machine | 0 | depends on [t-01KYRMFV10W1N28TCN5NWAGSW5](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) |
| [t-01KYRMFV10W1N28TCN5WVTCB1J](tasks/t-01KYRMFV10W1N28TCN5WVTCB1J.md) | Two-machine dogfood: real claim races over one remote | 2 | escalation: This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week, and do you want a prep task filed first for sync-latency instrumentation? |
| [t-01KYRMFV10W1N28TCN62F6FRTH](tasks/t-01KYRMFV10W1N28TCN62F6FRTH.md) | Epoch compaction (D9): snapshot event + in-commit deletion | 1 | depends on [t-01KYRMFV10W1N28TCN5SH4QM7A](tasks/t-01KYRMFV10W1N28TCN5SH4QM7A.md) |

## On hold

Triaged, deliberately paused — never served to agents until reopened.

| ID | Task | Priority | Labels |
|---|---|---:|---|
| [t-01KYRMFV10W1N28TCN62RR3A4D](tasks/t-01KYRMFV10W1N28TCN62RR3A4D.md) | Daemon portability: unix-only lock and the socket-path length limit | 0 | go, platform |
| [t-01KYT63MB28Z535SMJCBC7SY1P](tasks/t-01KYT63MB28Z535SMJCBC7SY1P.md) | Tree/parent-grouped rendering in the TUI list | 0 | cli, tui, design |
| [tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ](tasks/tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ.md) | Workflow recipes: recommended dev-flow patterns in init/docs (product feature, not this repo's conventions) | 0 | docs, product |
| [tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2](tasks/tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2.md) | Marketing / docs site for tuhdoo (monorepo: site/ in this repo) | 0 | docs, product, web |
| [tuh-01KYX7303WN3RSBXXB9CAGZB01](tasks/tuh-01KYX7303WN3RSBXXB9CAGZB01.md) | History view: surface the activity ledger (TUI pane and/or command) | 0 | design, tui, cli, product |

## Inbox

Untriaged captures — promoting one to open means writing it a real (prompt-quality) description first.

- [tuh-01KYXAS20FAP956KBHVZ057WBW](tasks/tuh-01KYXAS20FAP956KBHVZ057WBW.md) — do a design pass to make the generated markdown files in the tuhdoo branch pretty, or least easy to read and scannable/glanceable
- [tuh-01KYXE4NSNBFFRTT8STNDJHYED](tasks/tuh-01KYXE4NSNBFFRTT8STNDJHYED.md) — i should be able to move the cursor in all text boxes using standard commands
- [tuh-01KYXE5376YPXHDS98V3K985M6](tasks/tuh-01KYXE5376YPXHDS98V3K985M6.md) — you should be able to edit the desription or title in the task view

## Done

- [t-01KYRMFV10W1N28TCN5TDQC7KM](tasks/t-01KYRMFV10W1N28TCN5TDQC7KM.md) — Grow watch into the interactive steering TUI (tuhdoo top)
- [t-01KYRMFV10W1N28TCN5ZZ9Z2C1](tasks/t-01KYRMFV10W1N28TCN5ZZ9Z2C1.md) — Retire full-replay-per-write and the grow-forever event overlay
- [t-01KYRMVT1YC2929WSQ3W6YHHZM](tasks/t-01KYRMVT1YC2929WSQ3W6YHHZM.md) — Render markdown views on local daemon writes
- [t-01KYRR78YKX9YHZE6W6B798X4G](tasks/t-01KYRR78YKX9YHZE6W6B798X4G.md) — Auto-derive session principals: git identity + daemon-minted agent names
- [t-01KYRVCBE83KT62BAE11W3TAM8](tasks/t-01KYRVCBE83KT62BAE11W3TAM8.md) — Release pipeline: tagged, cross-compiled binaries
- [t-01KYRVCBE83KT62BAE1502VV29](tasks/t-01KYRVCBE83KT62BAE1502VV29.md) — npm devDependency distribution (esbuild-pattern wrapper packages)
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
- [tuh-01KYWJRW0DD4CYGH29EZ151DCT](tasks/tuh-01KYWJRW0DD4CYGH29EZ151DCT.md) — TUI selection highlight and visual hierarchy: full-height gutter bar, adaptive tint, bold titles
- [tuh-01KYWJWCK26X34J7TGSNVK83BN](tasks/tuh-01KYWJWCK26X34J7TGSNVK83BN.md) — Needs Input is the single home for escalation-blocked tasks (task-shaped 3-line rows)
- [tuh-01KYWKT8NQ980F0NF4MJ9W33H5](tasks/tuh-01KYWKT8NQ980F0NF4MJ9W33H5.md) — Release npm job: switch to OIDC trusted publishing (drop NPM_TOKEN)
- [tuh-01KYWKT8NQ980F0NF4MN3VMT0Y](tasks/tuh-01KYWKT8NQ980F0NF4MN3VMT0Y.md) — claim_task on an escalation-blocked task reports "unmet dependencies"
- [tuh-01KYWVNF91Y7H9GK0X1RAE2SBW](tasks/tuh-01KYWVNF91Y7H9GK0X1RAE2SBW.md) — Audit: agents via MCP can perform the main steering actions users ask for
- [tuh-01KYWWH4DZH4TR7ASVGTDBT14P](tasks/tuh-01KYWWH4DZH4TR7ASVGTDBT14P.md) — One-shot steering surface: two-rule contract in design docs, serialized backlog/escalations output
- [tuh-01KYX1D49M9M0EB69HNVBZT906](tasks/tuh-01KYX1D49M9M0EB69HNVBZT906.md) — Trunk-based PR flow: squash-only merges, enforced by rulesets, loop rewired in CLAUDE.md
- [tuh-01KYX6CMQV1G6XDZGNAF2M5C5P](tasks/tuh-01KYX6CMQV1G6XDZGNAF2M5C5P.md) — Slash command /drain-backlog: the reusable drain-the-backlog prompt

## Cancelled

- [tuh-01KYWE39DD1VWJVZZT3KHAKTQ0](tasks/tuh-01KYWE39DD1VWJVZZT3KHAKTQ0.md) — just a test
- [tuh-01KYWJT42G4CXYPMB6VBX0RG1Q](tasks/tuh-01KYWJT42G4CXYPMB6VBX0RG1Q.md) — stronger visual hierarchy, maybe make tasks titles bold?
- [tuh-01KYX2JXT30YBT4ZQNEZP3Z7XM](tasks/tuh-01KYX2JXT30YBT4ZQNEZP3Z7XM.md) — confirm there is a complete test suite that is running changes get merged to main (especially as we light up the PR flow)
- [tuh-01KYX9VTPE9EBNBMBBNXY5XRAQ](tasks/tuh-01KYX9VTPE9EBNBMBBNXY5XRAQ.md) — Init flavor picker: multiple-choice workflow setup in `tuhdoo init` that drops recipe files
- [tuh-01KYXDZ2Y0MX2YJBX94TVF1NCE](tasks/tuh-01KYXDZ2Y0MX2YJBX94TVF1NCE.md) — the task view needs to be brought more in line with the design of the dash. bold on the fields names up top, white bars for headings, a clear section when theres an escalation to answer with an easy way to answer.
- [tuh-01KYXE2S7A1WD7RKZSA0TNP09A](tasks/tuh-01KYXE2S7A1WD7RKZSA0TNP09A.md) — archive should be a for clarity, and escalation answers and just be enter on the question in teh task view (the task view can suport multiple escalations for the same task and the user can select fro them with same gray background click or arrow ui as the dashboard)
