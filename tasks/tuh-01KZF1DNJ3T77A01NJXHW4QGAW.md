# Generated data-branch README says "the design lives in docs/" — wrong for adopters, and now wrong here

`tuh-01KZF1DNJ3T77A01NJXHW4QGAW`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `go` `ledger`
- **Created:** 2026-08-07 21:17 UTC by `brandon/claude-code-1`

## Description

Captured at the 2026-08-07 strategy-grill session. internal/views/views.go:141 (readme()) bakes the sentence "The design lives in `docs/` on the repository's main branch" into the generated data-branch README of every adopting repo. That was always an assumption foreign repos never promised, and after the docs swap (root docs/ = published content, working docs -> internal-docs/) it is wrong for this repo too. Fix is product wording — drop the sentence or generalize it; per T6 any rendering change is cosmetic-only with a highest-wins view-format bump if the version is touched.

## History

_No activity yet._
