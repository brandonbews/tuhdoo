# Backlog

0 in progress · 0 ready · 4 blocked · 5 on hold · 8 inbox · 82 done · 21 cancelled

**[1 open question](escalations.md) is waiting on a human.**

## In progress

_None._

## Ready

_None._

## Blocked / waiting

| ID | Task | Priority | Waiting on |
|---|---|---:|---|
| [`t-frth`](tasks/t-01KYRMFV10W1N28TCN62F6FRTH.md) | Epoch compaction (D9): snapshot event + in-commit deletion | 1 | depends on [`t-qm7a`](tasks/t-01KYRMFV10W1N28TCN5SH4QM7A.md) |
| [`tuh-n0er`](tasks/tuh-01KZEPBEE8HFDQVK96AV6RN0ER.md) | Launch tuhdoo: public-facing materials and adoption surface | 0 | depends on [`tuh-ban7`](tasks/tuh-01KZF973FY9JKJV5F38SM7BAN7.md); depends on [`tuh-yayn`](tasks/tuh-01KZPPRY7P2A6GN0AMSKPEYAYN.md); depends on [`tuh-r2ac`](tasks/tuh-01KZFGZD3XWZ8V56P5RKV3R2AC.md) |
| [`tuh-ban7`](tasks/tuh-01KZF973FY9JKJV5F38SM7BAN7.md) | Site visual identity: Brandon's logo, full pass (landing, docs chrome, favicon, og) | 1 | an [open question](escalations.md) |
| [`tuh-r2ac`](tasks/tuh-01KZFGZD3XWZ8V56P5RKV3R2AC.md) | Production-readiness sweep: the last full check before launch | 1 | depends on [`tuh-ban7`](tasks/tuh-01KZF973FY9JKJV5F38SM7BAN7.md) |

## On hold

Triaged, deliberately paused — never served to agents until reopened.

| ID | Task | Priority | Labels |
|---|---|---:|---|
| [`t-qm7a`](tasks/t-01KYRMFV10W1N28TCN5SH4QM7A.md) | v1 milestone: steering surface and a second machine | 0 | `milestone` |
| [`t-3a4d`](tasks/t-01KYRMFV10W1N28TCN62RR3A4D.md) | Daemon portability: unix-only lock and the socket-path length limit | 0 | `go` `platform` |
| [`tuh-8xe2`](tasks/tuh-01KZA0VT234XJYVZWT8YFV8XE2.md) | Monorepo grain: is one tuhdoo branch per repo right when a repo hosts many projects? | 0 | `design` |
| [`tuh-7k2y`](tasks/tuh-01KZA0VT234XJYVZWT980V7K2Y.md) | Working-set retirement: bounding what surfaces show without deleting history | 0 | `design` `ledger` |
| [`tuh-yayn`](tasks/tuh-01KZPPRY7P2A6GN0AMSKPEYAYN.md) | The announcement moment: where, what, when | 0 | `launch` `product` |

## Inbox

Untriaged captures — promoting one to open means writing it a real (prompt-quality) description first.

- [`tuh-g9dt`](tasks/tuh-01KZ4PY7QJEAZ6T8R1V046G9DT.md) Task-view history omits task.updated — field edits leave no visible trace
- [`tuh-pez8`](tasks/tuh-01KZ9Y3THHH5B8GT22T92BPEZ8.md) Epics after parents removal: is any epic UX worth having?
- [`tuh-0ssg`](tasks/tuh-01KZ9YBF1N06FQ37XV65940SSG.md) sweep for duplicate code
- [`tuh-ye8v`](tasks/tuh-01KZ9Z6647C3TBCYGGTXQJYE8V.md) create tight but meaningful design doc for my review to help me understand the codebase and have a starting point for understanding go against it. should include a mermaid diagram that visualizes all the pieces and details on all of the modules and sections of the codebase
- [`tuh-qgaw`](tasks/tuh-01KZF1DNJ3T77A01NJXHW4QGAW.md) Generated data-branch README says "the design lives in docs/" — wrong for adopters, and now wrong here
- [`tuh-yvn0`](tasks/tuh-01KZPW8CZPKF2KTWMA5B8QYVN0.md) Adoption docs and `tuhdoo init` miss hosted preview builders (Vercel, Netlify, Cloudflare Pages)
- [`tuh-hq7f`](tasks/tuh-01KZPW8CZPKF2KTWMA5EAVHQ7F.md) Grill: is the data branch's CI/build-tooling friction a deal breaker for early adopters?
- [`tuh-8e86`](tasks/tuh-01KZPYFK8GFXQ5T2GFPCRQ8E86.md) escalations are kind of hard to read and navigate in the current ux. make them cleaerer in the task view UX wise and also ask teh agent to make escalations as short and actionable as possible but still full required context

## Done

- [`t-gsw5`](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) v0 definition of done: the dogfood week holds
- [`t-c7km`](tasks/t-01KYRMFV10W1N28TCN5TDQC7KM.md) Grow watch into the interactive steering TUI (tuhdoo top)
- [`t-cb1j`](tasks/t-01KYRMFV10W1N28TCN5WVTCB1J.md) Two-machine convergence: a deliberate claim-collision harness
- [`t-z2c1`](tasks/t-01KYRMFV10W1N28TCN5ZZ9Z2C1.md) Retire full-replay-per-write and the grow-forever event overlay
- [`t-hhzm`](tasks/t-01KYRMVT1YC2929WSQ3W6YHHZM.md) Render markdown views on local daemon writes
- [`t-8x4g`](tasks/t-01KYRR78YKX9YHZE6W6B798X4G.md) Auto-derive session principals: git identity + daemon-minted agent names
- [`t-tam8`](tasks/t-01KYRVCBE83KT62BAE11W3TAM8.md) Release pipeline: tagged, cross-compiled binaries
- [`t-vv29`](tasks/t-01KYRVCBE83KT62BAE1502VV29.md) npm devDependency distribution (esbuild-pattern wrapper packages)
- [`t-d83w`](tasks/t-01KYT63MB28Z535SMJC9B0D83W.md) One TUI: bare tuhdoo is the interactive surface (Cycle 4)
- [`t-rqjm`](tasks/t-01KYT63MB28Z535SMJCA63RQJM.md) Arm the TUI detail screen: selectable escalation, enter to answer; p/c on the viewed task
- [`t-f1y3`](tasks/t-01KYT80CP4JKAM3V2C4DNGF1Y3.md) TUI readability: short display IDs, width-aware wrapping, list scrolling
- [`t-hxvv`](tasks/t-01KYTQTEQYGXVQ0QY7F95CHXVV.md) Principal identity override: stop deriving ugly actors from noreply emails
- [`t-z3g6`](tasks/t-01KYTQTEQYGXVQ0QY7FCTAZ3G6.md) Agent protocol: dangling-pointer anti-pattern; notes are garnish, transitions are the record
- [`t-r2gs`](tasks/t-01KYTRJQT44HGYXGR9C7C3R2GS.md) Escalation ergonomics: resolve answered-out-of-band, and the name itself
- [`t-z5dw`](tasks/t-01KYTSQDQJWM8YQQ8FWMHBZ5DW.md) Short IDs are the human contract: display everywhere, accept as input, annotate edges
- [`t-73kw`](tasks/t-01KYVD31CNTR1EVCDHPC5973KW.md) Needs Input: enter answers in place; blocked rows stop repeating the question
- [`t-4gmz`](tasks/t-01KYVD31CNTR1EVCDHPG0G4GMZ.md) TUI navigation: up/down arrows move the cursor; footer says so
- [`t-q5ev`](tasks/t-01KYVD31CNTR1EVCDHPGZFQ5EV.md) Rename the cancel interaction: archive as the human verb, task.cancelled stays the plumbing
- [`t-v9vk`](tasks/t-01KYVD31CNTR1EVCDHPHJEV9VK.md) TUI mouse support: click selects, click again acts as enter
- [`t-qagh`](tasks/t-01KYVD31CNTR1EVCDHPJGSQAGH.md) Align MCP tool descriptions with the revised notes doctrine
- [`t-wrkm`](tasks/t-01KYVE848CJZNG5VFWZ9J3WRKM.md) Brand the task IDs: mint tuh-, accept both prefixes, age out t-
- [`t-p213`](tasks/t-01KYVEXK2BX040KJ244S2WP213.md) CLI write verbs: a paved path when no MCP session exists
- [`t-pvg4`](tasks/t-01KYVJ2607S5S390CVYSF3PVG4.md) TUI dashboard visual redesign: section bars + fixed column grid (mock-a)
- [`t-bbmc`](tasks/t-01KYVJ5NABEKX72KNE006MBBMC.md) Inbox and held: capture without scoping pressure, pause without pretending
- [`t-769x`](tasks/t-01KYVMD4PS9NMQVP1K5HQ8769X.md) finish_run accepts a claimless finish: opFinishRun never checks holdership
- [`t-vrd8`](tasks/t-01KYVMD4PS9NMQVP1K5M1PVRD8.md) Shim died once: 'stdio session: invalid trailing data at the end of stream'
- [`tuh-1dct`](tasks/tuh-01KYWJRW0DD4CYGH29EZ151DCT.md) TUI selection highlight and visual hierarchy: full-height gutter bar, adaptive tint, bold titles
- [`tuh-83bn`](tasks/tuh-01KYWJWCK26X34J7TGSNVK83BN.md) Needs Input is the single home for escalation-blocked tasks (task-shaped 3-line rows)
- [`tuh-33h5`](tasks/tuh-01KYWKT8NQ980F0NF4MJ9W33H5.md) Release npm job: switch to OIDC trusted publishing (drop NPM_TOKEN)
- [`tuh-mt0y`](tasks/tuh-01KYWKT8NQ980F0NF4MN3VMT0Y.md) claim_task on an escalation-blocked task reports "unmet dependencies"
- [`tuh-2sbw`](tasks/tuh-01KYWVNF91Y7H9GK0X1RAE2SBW.md) Audit: agents via MCP can perform the main steering actions users ask for
- [`tuh-t14p`](tasks/tuh-01KYWWH4DZH4TR7ASVGTDBT14P.md) One-shot steering surface: two-rule contract in design docs, serialized backlog/escalations output
- [`tuh-t906`](tasks/tuh-01KYX1D49M9M0EB69HNVBZT906.md) Trunk-based PR flow: squash-only merges, enforced by rulesets, loop rewired in CLAUDE.md
- [`tuh-pmrq`](tasks/tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ.md) Workflow recipes: recommended dev-flow patterns in init/docs (product feature, not this repo's conventions)
- [`tuh-hdm2`](tasks/tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2.md) Marketing / docs site for tuhdoo (monorepo: site/ in this repo)
- [`tuh-5c5p`](tasks/tuh-01KYX6CMQV1G6XDZGNAF2M5C5P.md) Slash command /drain-backlog: the reusable drain-the-backlog prompt
- [`tuh-zb01`](tasks/tuh-01KYX7303WN3RSBXXB9CAGZB01.md) History view: h opens the done/cancelled shelf in the TUI
- [`tuh-7wbw`](tasks/tuh-01KYXAS20FAP956KBHVZ057WBW.md) Design pass on the generated markdown views: scannable, glanceable data branch
- [`tuh-a86y`](tasks/tuh-01KYXDWWM8S1GF6N9NE5FGA86Y.md) Task view is the answering home: routed from Needs Input, dash-aligned design, selectable escalations, a to archive
- [`tuh-ypww`](tasks/tuh-01KYXE40ES9YSEGW9Z0GXKYPWW.md) One real text-input widget: delineated box, fixed hint line, standard cursor editing
- [`tuh-85m6`](tasks/tuh-01KYXE5376YPXHDS98V3K985M6.md) Edit title and description from the task view
- [`tuh-11sh`](tasks/tuh-01KYXEMYC5XE928EWKYA0P11SH.md) npm provenance: trusted publishing promised attestations, the registry has none
- [`tuh-7t5y`](tasks/tuh-01KYXNTBJRKM8YDW6QG6ED7T5Y.md) shortID is duplicated between cmd/tuhdoo and internal/views — extract one shared helper
- [`tuh-s8vt`](tasks/tuh-01KYXT2KAG7QXZGF1W47E6S8VT.md) Task view field focus ring: up/down selects any editable field, enter opens its editor
- [`tuh-vtfa`](tasks/tuh-01KYXVK1TV66GR1JV8TCG8VTFA.md) Task-view history: blank line between entries, bold entry descriptors
- [`tuh-bqkm`](tasks/tuh-01KYXVSRVK2GFW439G1T0GBQKM.md) Labels editable from the task view
- [`tuh-m0qk`](tasks/tuh-01KYZ9FJH4N2XFRXJ9ANV1M0QK.md) get_backlog scope input: MCP read parity with the TUI sections (T5 revision)
- [`tuh-ntpk`](tasks/tuh-01KZ0ES83SFH6MKWP82Y2HNTPK.md) Status vocabulary: cancelled replaces archived everywhere; "on hold" stays as the one display mapping
- [`tuh-wqd6`](tasks/tuh-01KZ0ES83SFH6MKWP82YRXWQD6.md) One classifier: the daemon serves the derived situation
- [`tuh-mxbx`](tasks/tuh-01KZ0QFCE3PQMX9RFS1H8KMXBX.md) Task view: wrap-then-indent the title and description blocks so the focus gutter is continuous
- [`tuh-hs07`](tasks/tuh-01KZ2HCCBM0RY70GJKMKHFHS07.md) Task view id line shows only the short form (T7 revision: the full ULID leaves the TUI)
- [`tuh-a3v7`](tasks/tuh-01KZ33YQPXPK59NV1VBWZ9A3V7.md) TUI chrome hierarchy: unfilled frame, shelf-gray bars, bold-key footer
- [`tuh-76wt`](tasks/tuh-01KZ4TH4HT56TE4CQPKF3R76WT.md) Loser handling: verb-time stand-down, coerced superseded, expiry synthesis (D6 revision)
- [`tuh-7a40`](tasks/tuh-01KZ4TH4HT56TE4CQPKKA37A40.md) Status.Collisions undercounts push contention — a lost ref-update race is not classified as non-fast-forward
- [`tuh-7hs6`](tasks/tuh-01KZ53FJHRFXB932MH8VSS7HS6.md) TUI bar recolors: dim-red BLOCKED, bright-white INBOX (section order confirmed unchanged)
- [`tuh-3595`](tasks/tuh-01KZ53K4DF7Y0TYX3H5XP43595.md) Pinned frame off-by-one: trailing newline clips the header row
- [`tuh-n777`](tasks/tuh-01KZ5WMT4GWZTYVRGWN4PFN777.md) Confirmation gate: claim.confirmed won through the remote CAS (D6 revision 2026-08-04)
- [`tuh-ysvn`](tasks/tuh-01KZ5WMT4GWZTYVRGWN56TYSVN.md) Collision harness: drive the real D6 machinery, add a confirmation-race storm
- [`tuh-d9bk`](tasks/tuh-01KZ86YH64K9D2AKVQF57KD9BK.md) Lease tombstones: released marker, deletion retired, merge rule (grill 2026-08-04)
- [`tuh-0xjx`](tasks/tuh-01KZ9HDMYDGCM0HKMV3FZ00XJX.md) Dashboard list hides most row metadata past page width — resurface labels, dep counts, priority
- [`tuh-wzrg`](tasks/tuh-01KZ9Y3THHH5B8GT22SY3FWZRG.md) Remove the parents edge: epics are depends_on containers (edge grill 2026-08-05)
- [`tuh-wpyp`](tasks/tuh-01KZ9Y3THHH5B8GT22T1A1WPYP.md) Dependency loops and cancelled deps: reject at edit, mark loudly at replay (edge grill 2026-08-05)
- [`tuh-r3e8`](tasks/tuh-01KZ9Y3THHH5B8GT22T1TZR3E8.md) Clone-join: adopt an existing remote tuhdoo branch instead of minting a second root
- [`tuh-2hvf`](tasks/tuh-01KZ9Y3THHH5B8GT22T5D72HVF.md) Doc-sync sweep: align docs with code and the 2026-08-05 release grill; tombstone open-questions into the ledger
- [`tuh-nvyk`](tasks/tuh-01KZ9Y3THHH5B8GT22T650NVYK.md) Release plumbing: smoke.sh verb-count fix, release-workflow smoke gate, versioned make build
- [`tuh-y4re`](tasks/tuh-01KZ9Y3THHH5B8GT22T7JVY4RE.md) init hardening: loud unknown-flag errors and the MCP snippet in init output
- [`tuh-r40k`](tasks/tuh-01KZ9Y3THHH5B8GT22T910R40K.md) Cut v0.2.0: final verification and tag handoff
- [`tuh-m6qy`](tasks/tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY.md) User-facing docs: the human story of steering tuhdoo, platform-agnostic
- [`tuh-4hqt`](tasks/tuh-01KZA0VT234XJYVZWT8GVB4HQT.md) Agent protocol: document how a successor finds a predecessor's branch (salvage breadcrumbs)
- [`tuh-bsdh`](tasks/tuh-01KZA0VT234XJYVZWT8KT0BSDH.md) Run records: additive merged_as field for the commit that actually landed
- [`tuh-pk06`](tasks/tuh-01KZA0VT234XJYVZWT8S09PK06.md) Onboarding remainder: teammate joining doc and branch-protection guidance (doc + init line)
- [`tuh-ek1s`](tasks/tuh-01KZA0VT234XJYVZWT93P2EK1S.md) Uninstall doc + test: prove a team can walk away clean
- [`tuh-q5cd`](tasks/tuh-01KZANB3J4YYH09F0Z6FSZQ5CD.md) Ship the agent protocol with the binary: tuhdoo protocol command
- [`tuh-r7rg`](tasks/tuh-01KZC6XBFXGFMXEEQP9KD1R7RG.md) is the daemon running whe i type tuhdoo and closed when i exit? how does that part work?
- [`tuh-05fn`](tasks/tuh-01KZCMF7JKMXVDG0HANVVQ05FN.md) claim_next selection: document the label filter and priority order for agents; test the matching code
- [`tuh-qf4g`](tasks/tuh-01KZEPBEE8HFDQVK96AQNCQF4G.md) Grill: site stack & content strategy (human-led)
- [`tuh-kk4j`](tasks/tuh-01KZF1DNJ3T77A01NJXF1VKK4J.md) Docs swap: root docs/ becomes the published content root; working docs move to internal-docs/
- [`tuh-0q0x`](tasks/tuh-01KZF2D3MA0P24WKAWK89J0Q0X.md) Vercel project + tuhdoo.com DNS wiring (human-led)
- [`tuh-vq1p`](tasks/tuh-01KZF83BZVEXYJ2S3GGQQDVQ1P.md) Root .gitignore: ignore OS junk (.DS_Store and friends)
- [`tuh-jhqx`](tasks/tuh-01KZF97PATRZ1TFWA7CQQCJHQX.md) Docs and landing copy pass: the skeptical evaluator, against the settled writing bar
- [`tuh-wyqn`](tasks/tuh-01KZF992N0AM2TZGHMH2T5WYQN.md) Site toolchain: current create-next-app baseline — Tailwind v4, Biome, full CI gate
- [`tuh-7sqc`](tasks/tuh-01KZPPRW1EKBTVKNR76H6Z7SQC.md) Logo finals: export and deliver the brand assets (human-led)

## Cancelled

- [`t-sy1p`](tasks/t-01KYT63MB28Z535SMJCBC7SY1P.md) Tree/parent-grouped rendering in the TUI list
- [`tuh-ktq0`](tasks/tuh-01KYWE39DD1VWJVZZT3KHAKTQ0.md) just a test
- [`tuh-rg1q`](tasks/tuh-01KYWJT42G4CXYPMB6VBX0RG1Q.md) stronger visual hierarchy, maybe make tasks titles bold?
- [`tuh-z7xm`](tasks/tuh-01KYX2JXT30YBT4ZQNEZP3Z7XM.md) confirm there is a complete test suite that is running changes get merged to main (especially as we light up the PR flow)
- [`tuh-xraq`](tasks/tuh-01KYX9VTPE9EBNBMBBNXY5XRAQ.md) Init flavor picker: multiple-choice workflow setup in `tuhdoo init` that drops recipe files
- [`tuh-1nce`](tasks/tuh-01KYXDZ2Y0MX2YJBX94TVF1NCE.md) the task view needs to be brought more in line with the design of the dash. bold on the fields names up top, white bars for headings, a clear section when theres an escalation to answer with an easy way to answer.
- [`tuh-p09a`](tasks/tuh-01KYXE2S7A1WD7RKZSA0TNP09A.md) archive should be a for clarity, and escalation answers and just be enter on the question in teh task view (the task view can suport multiple escalations for the same task and the user can select fro them with same gray background click or arrow ui as the dashboard)
- [`tuh-hyed`](tasks/tuh-01KYXE4NSNBFFRTT8STNDJHYED.md) i should be able to move the cursor in all text boxes using standard commands
- [`tuh-kw85`](tasks/tuh-01KYXT6GSA20QPJ5ME1KP6KW85.md) i suspect that t-sy1p lists a dependency as why its on hold bu tthat dependency appears cleared. what the deal there
- [`tuh-qt4g`](tasks/tuh-01KYZ34E3NR1HM02P8P3G7QT4G.md) there should be a concept of codereview for workflows that work that way/require it. it's almost a normal escalation but not quite. lets explore ways to do it. my current hunch is maybe an IN CODE REVIEW section that only appears if a task actually has that status. if we go that route, i could see ON HOLD, and NEEDS INPUT only showing when relevant too. def need a grill session for this
- [`tuh-h4k6`](tasks/tuh-01KYZ9BJWWE0NZ89FJZW5KH4K6.md) status vs section in the dashoboard is a little confusing. grill and align behavior for ux. we need vocab alignment between data and surfaces this early in the project.
- [`tuh-7qxj`](tasks/tuh-01KZ4TH4HT56TE4CQPKHBM7QXJ.md) D6's machine-id tiebreak is vacuous — ULIDs never tie, so the tiebreak branch does not exist
- [`tuh-2tma`](tasks/tuh-01KZA0VT234XJYVZWT8C4X2TMA.md) Task-descriptions-are-prompts: a template or convention worth shipping?
- [`tuh-78j5`](tasks/tuh-01KZA0VT234XJYVZWT8EXV78J5.md) claim_next discovery: capability/label filters, affinity hints, priority semantics
- [`tuh-75w9`](tasks/tuh-01KZA0VT234XJYVZWT8K2D75W9.md) Escalation delivery when the TUI is closed
- [`tuh-p9qm`](tasks/tuh-01KZA0VT234XJYVZWT8Q19P9QM.md) Plan-materialization flow end-to-end; decomposition-quality prompting conventions
- [`tuh-g3nx`](tasks/tuh-01KZA0VT234XJYVZWT8VGFG3NX.md) Repo-hosting edge cases: shallow clones, --single-branch, forks, mirrors
- [`tuh-jgjr`](tasks/tuh-01KZA0VT234XJYVZWT91KSJGJR.md) Multi-repo story: does a plan ever span repos, or is that explicitly out of scope?
- [`tuh-25kw`](tasks/tuh-01KZA0VT234XJYVZWT95JM25KW.md) Epoch compaction triggers and mechanics in practice
- [`tuh-nxeh`](tasks/tuh-01KZA0VT234XJYVZWT98B7NXEH.md) v2+ parked features (pointer): intake bridge, signing, kanban, view templates, webhook fetch, supervisor, read-only sharing
- [`tuh-ehmt`](tasks/tuh-01KZFH07P6GB1X1J827PHREHMT.md) marketing aspect of site should emphasize the serverless, owned, git tracked but still distributed nature
