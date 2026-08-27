# tuhdoo answer: surface ambiguous task fragments as candidate lists

`tuh-01KZVZT7F8CVJYX1P009PAE9QM`

- **Status:** open — in progress, claimed by `brandon/claude-code-2`
- **Priority:** none
- **Labels:** `go` `cli` `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit finding, re-verified 2026-08-27. resolveEscalation (cmd/tuhdoo/write_cmds.go:316-320) discards resolveTaskID's ambiguity error (taskID = ""), so `tuhdoo answer <frag>` where the fragment ambiguously matches several tasks falls through to "no open escalation matches %q" (~line 334) instead of listing the task candidates. The function's own doc comment (311-315: "Ambiguity — several open questions match — is an error listing the candidates, never a guess") and 002 T7:148 ("the resolver already handles collisions loudly") both promise loud ambiguity; it is honored for escalation-ID ambiguity (336-341) but not task-fragment ambiguity. No design decision needed — the written contract already decided; this is a fix.

The ask: propagate the ambiguity error. When resolveTaskID reports the fragment is ambiguous, return that error (listing the candidate tasks) instead of falling through. Not-found must keep falling through to escalation-ID matching exactly as today. If resolveTaskID's error shapes don't distinguish ambiguous from not-found, make them distinguishable first (typed error or sentinel) — only ambiguity becomes loud.

Acceptance: CLI test: a fragment ambiguously matching two or more tasks errors listing the candidates (existing resolver ambiguity phrasing); a fragment matching nothing as a task but matching an escalation ID still resolves (regression pin); unambiguous behavior unchanged. make test lint green.

Constraints: error wording follows the existing resolver ambiguity messages; no other resolution semantics change.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +go +cli
