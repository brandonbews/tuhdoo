# tuhdoo answer swallows ambiguous task fragments instead of listing candidates

`tuh-01KZVZT7F8CVJYX1P009PAE9QM`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding. write_cmds.go ~305-309 (resolveEscalation): resolveTaskID's ambiguity error is discarded (taskID = ""), so `tuhdoo answer <frag>` where the fragment ambiguously matches several tasks reports 'no open escalation matches' (or matches on escalation-ID only) instead of listing the task candidates. Doesn't guess — no letter-of-T7 violation — but hides ambiguity the contract elsewhere insists on surfacing. Decide and fix; behavior change, out of the sweep's scope.

## History

_No activity yet._
