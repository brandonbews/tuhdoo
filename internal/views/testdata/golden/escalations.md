# Escalations

The steering inbox: questions raised by agents, awaiting a human answer.

## Open

### [`t-flak`](tasks/t-flak.md) · Fix flaky TestFoo

**Blocking** · asked by `brandon/impl-2` · 2026-07-29 12:17 UTC

> TestFoo depends on wall-clock timing — rewrite or delete?

It races a 10ms sleep against the scheduler. Rewriting means faking the clock.

### [`t-view`](tasks/t-view.md) · Render markdown views

Non-blocking · asked by `brandon/impl-3` · 2026-07-29 12:16 UTC

> Views ship tables in v0, or lists until GFM support is confirmed?

## Answered

### [`t-core`](tasks/t-core.md) · Build the replay engine

Asked by `brandon/impl-1` · 2026-07-29 12:10 UTC

> Should upcasters live in core or in a separate package?

**Answer** (`brandon`): Keep them in core; they are part of honest replay.

### [`tuh-dmn4`](tasks/tuh-01KYRMFV10W1N28TCN5NDADMN4.md) · Daemon skeleton

Asked by `sarah/impl-9` · 2026-07-29 12:18 UTC

> Reuse the repo lockfile for the daemon singleton?

**Answer** (`sarah`, relayed by `sarah/impl-9`): Yes — one lockfile, one meaning.
