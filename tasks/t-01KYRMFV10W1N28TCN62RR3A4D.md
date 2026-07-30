# t-01KYRMFV10W1N28TCN62RR3A4D — Daemon portability: unix-only lock and the socket-path length limit

- Status: open — ready
- Priority: 0
- Labels: `go`, `platform`, `parked`
- Created: 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: two known limits from the build-out. (1) Single-instance enforcement uses flock, so the daemon is unix-only. (2) The B11 field test found that a repo path long enough for .git/tuhdoo/daemon.sock to exceed the ~103-byte unix-socket path limit fails daemon startup (with a clear error) — accepted for v0.

The ask: parked until real demand (a Windows user, or a dogfood path hitting the limit). Then: a portable single-instance strategy, and a socket-path fallback (e.g. a short hashed path under TMPDIR with a pointer in .git/tuhdoo/daemon.json).

Acceptance (when unparked): daemon runs single-instance on the new platform with existing daemon tests green; a long-path repo starts and serves.

Constraints: discovery stays repo-local via .git/tuhdoo/daemon.json (T4); host-agnosticism untouched (T2).

## History

_No activity yet._
