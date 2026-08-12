# Socket-path length limit: stable TMPDIR fallback for deep repo paths

`t-01KYRMFV10W1N28TCN62RR3A4D`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 0
- **Labels:** `go` `platform`
- **Created:** 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: the B11 field test found that a repo path deep enough for .git/tuhdoo/daemon.sock to exceed the ~103-byte unix sun_path limit fails daemon startup (clear error — accepted for v0). This task originally also carried Windows portability (the flock single-instance lock is unix-only). On 2026-08-12 Brandon decided tuhdoo does not support Windows: macOS and Linux only; Windows users run it under WSL, where tuhdoo is Linux and everything works. The Windows half is dropped, not parked — do not reopen it when a Windows user materializes; the public docs state the stance instead.

The ask: when the default socket path would exceed the limit, bind a short per-repo-stable hashed path under the OS temp dir instead, and record whichever path was bound in .git/tuhdoo/daemon.json. Clients already discover the socket solely through daemon.json (liveSocket in cmd/tuhdoo/client.go), so no client changes are needed.

Acceptance: the path choice is a pure function with table-driven tests (in-dir when it fits; stable temp-dir fallback when it does not; error only when the fallback also exceeds the limit); an integration test proves a repo deep enough to bust the limit starts and serves over the fallback socket; stale-socket removal works at the fallback path; existing daemon tests stay green; docs (README install section, docs/joining.md) state the macOS/Linux-plus-WSL platform stance.

Pointers: internal/daemon/daemon.go (maxSocketPath, New's listen block), cmd/tuhdoo/client.go (liveSocket), internal/daemon/daemon_test.go (shortTempDir documents the pain this fixes).

Constraints: discovery stays repo-local via .git/tuhdoo/daemon.json (T4); host-agnosticism untouched (T2); the fallback path must be stable per repo so a crashed daemon's stale socket is found and removed by the same startup path that handles the in-dir case.

## History

### 2026-07-31 15:49 UTC — edit by `brandon`

description edited · status open→held · labels −parked

### 2026-08-12 19:15 UTC — edit by `brandon`

retitled · description edited · status held→open
