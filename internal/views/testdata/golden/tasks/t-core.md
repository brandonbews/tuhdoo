# Build the replay engine

`t-core`

- **Status:** done
- **Priority:** 5
- **Labels:** `core` `go`
- **Created:** 2026-07-29 12:02 UTC by `brandon`

## Description

Pure function: event set -> state.

Acceptance:
- order-insensitive replay
- lease expiry returns tasks to the pool

## History

### 2026-07-29 12:09 UTC — note from `brandon/impl-1`

Found the ordering bug: replay sorted by insertion, not ULID.
Fix in progress.

### 2026-07-29 12:10 UTC — escalation from `brandon/impl-1`

> Should upcasters live in core or in a separate package?

T3 says in-memory only; either placement satisfies that.

**Answer** (`brandon`): Keep them in core; they are part of honest replay.

### 2026-07-29 12:11 UTC — run by `brandon/impl-1` — done

- Branch: `feat/replay-engine`
- PR: <https://example.com/pr/12>
- Commits: `a1b2c3d`, `e4f5a6b`

Replay engine with winner rule and lease expiry; 24-permutation order-insensitivity test green.
