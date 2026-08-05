# Release plumbing: smoke.sh verb-count fix, release-workflow smoke gate, versioned make build

`tuh-01KZ9Y3THHH5B8GT22T650NVYK`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 2
- **Labels:** `build` `npm` `ci`
- **Created:** 2026-08-05 21:43 UTC by `brandon/claude-code-1`

## Description

Context: pre-v0.2.0 audit (2026-08-05). npm/smoke.sh hard-asserts eleven MCP tools (:93) and now fails against main (twelve since 2026-08-04); it runs nowhere automatically, so the drift went unnoticed — the exact class of rot a release gate exists to catch. Separately, `make build` doesn't stamp a version (Makefile has no ldflags; cmd/tuhdoo/main.go:18 defaults to "dev"), so the dogfood binary is indistinguishable in bug reports once project #2 runs a tagged build. Decided at the grill: smoke becomes a release-pipeline gate.

The ask: (1) Fix smoke.sh to assert the tool surface by *named list*, not a bare count — removing or adding a verb must fail with a named diff. Keep it packaging-level. (2) Add a smoke step to .github/workflows/release.yml before npm publish, so a tag can never publish a package whose own smoke test fails. (3) Makefile: stamp `-ldflags "-X main.version=$(git describe --tags --always --dirty)"` on the build target.

Acceptance: smoke.sh passes against current main; deleting a verb registration locally makes it fail naming the missing tool; `bin/tuhdoo version` prints a git-describe string after `make build`; release.yml gates publish on smoke; `make test lint` green.

Pointers: npm/smoke.sh:7,76,93; .github/workflows/release.yml:45,116-117; Makefile; cmd/tuhdoo/main.go:18.

Constraints: WORKFLOW LAW — the release.yml diff executes unattended with CI credentials and must be called out explicitly and separately in the PR body AND the session summary for Brandon's eyes-on review; never folded silently. One PR.

## History

_No activity yet._
