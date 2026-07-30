# t-01KYRVCBE83KT62BAE11W3TAM8 — Release pipeline: tagged, cross-compiled binaries

- Status: open — ready
- Priority: 1
- Labels: `distribution`, `ci`
- Created: 2026-07-30 06:28 UTC by `brandon/impl-2`

## Description

Context: tuhdoo has no distribution story yet — no tags, no releases, no CI; the only binary is this repo's own `make build` artifact. T1 chose Go for the single-static-binary install story (`brew install tuhdoo`), and T4's per-repo daemon rationale assumes per-repo version pinning, which needs versioned artifacts to pin. This task is the machine-level tier; the npm devDependency tier builds on it and is a separate task.

The ask: a release pipeline that turns a git tag (v0.x.y) into published binaries. Cross-compile matrix with CGO_ENABLED=0 for at least darwin/arm64, darwin/amd64, linux/arm64, linux/amd64 (windows is parked with the unix-only-daemon task t-01KYRMFV10W1N28TCN62RR3A4D — do not attempt it). Stamp the version via the existing ldflags hook (main.version in cmd/tuhdoo/main.go). Publish to GitHub Releases. goreleaser is the boring default; hand-rolled matrix is fine too if smaller. A homebrew tap is optional stretch — include only if it falls out nearly free.

Acceptance: pushing a tag produces a GitHub Release with checksummed archives for the four platforms; `tuhdoo version` on an extracted binary prints the tag; a README or docs section documents the install paths (release download, `go install ...@latest`); make test lint stays green.

Pointers: Makefile, cmd/tuhdoo/main.go (version var), docs/design/002-technology.md T1 (install story), T3 (the three version contracts — binary version moves freely).

Constraints: PROJECT LAW — anything under .github/workflows/ executes unattended with CI credentials and must be called out explicitly and separately in the session summary for Brandon's eyes-on diff review; never fold it silently into a larger commit. Host-agnosticism (T2) untouched: the pipeline publishes artifacts, the binary itself never learns about GitHub. Exclude the tuhdoo data branch from CI triggers (branches-ignore), matching what `tuhdoo init` already tells users.

## History

_No activity yet._
