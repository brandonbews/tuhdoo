# is the daemon running whe i type tuhdoo and closed when i exit? how does that part work?

`tuh-01KZC6XBFXGFMXEEQP9KD1R7RG`

- **Status:** done
- **Priority:** none
- **Created:** 2026-08-06 18:55 UTC by `brandon`

## Description

Question task; answered 2026-08-07 in session, closed done at Brandon's direction.

Answer: half yes, half no. Every tuhdoo command (including bare `tuhdoo`) goes through ensureDaemon (cmd/tuhdoo/client.go): it reads .git/tuhdoo/daemon.json, dials the socket to prove a daemon is actually serving (a stale file from a crash fails the dial), and if none answers it re-execs the binary as `tuhdoo daemon` — detached via Setsid into its own session, logging to .git/tuhdoo/daemon.log — then waits up to 5s for the socket (T4 lazy lifecycle). If two CLIs race, the daemon's flock makes the loser exit quietly and both find the winner's socket.

Exiting the TUI does NOT stop the daemon. It has no idle timeout and no client counting; the only exit paths are SIGINT/SIGTERM or listener failure (cmd/tuhdoo/daemon_cmd.go, daemon.Shutdown). It stays resident running the 60s sync loop until killed or reboot — which is by design: it's the sole writer and sync heartbeat for the repo, not a child of any one terminal.

## History

### 2026-08-07 19:18 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→done
