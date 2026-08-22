# Batcher.LastError is a reporting channel nobody reads

`tuh-01KZVZT7F8CVJYX1P0090K9FDB`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding. store/batcher.go ~87-94: LastError is the designed surface for timer-driven background flush failures, and no production code reads it (only store_test.go). Failed events stay pending and the next synchronous Flush surfaces the error, so nothing is lost — but a repo where writes stop after a timer-flush failure holds an error nobody will see or log. Wire it into daemon status/logging, or delete it. Behavior decision, out of the sweep's scope.

## History

_No activity yet._
