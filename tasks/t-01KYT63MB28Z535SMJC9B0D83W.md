# t-01KYT63MB28Z535SMJC9B0D83W — One TUI: bare tuhdoo is the interactive surface (Cycle 4)

- Status: done
- Priority: 2
- Labels: `cli`, `tui`, `design-revision`
- Created: 2026-07-30 18:55 UTC by `4099114+brandonbews`

## Description

Context: Cycle 4 (grill, 2026-07-30) settled that the human interactive surface is a single verb-less TUI. watch and top are two Bubble Tea models duplicating rendering; top has no detail view; bare `tuhdoo` wastes the front door.

The ask (5 commits, docs first per project law; full plan in the session that created this task):
1. Docs revision: T7/D8/roadmap/backlog-tombstone/open-questions(:13)/batcher comment — dated 2026-07-30, cite Cycle 4.
2. Fold watch into the TUI model: move tick/snap/fetch machinery into top.go, delete watch.go+watch_test.go, add `armed bool` (watch mode ignores a/p/c), full header (mode badge: `watch mode` vs `acting as <actor>` + syncLine + counts line), tombstone `case "watch"`.
3. Verb-less dispatch: bare `tuhdoo` launches TUI guarded (stdout TTY required, else usage/exit 1; uninitialized repo → `run tuhdoo init` hint); flags -w/--watch and --as via stdlib flag.FlagSet (--as+--watch rejected; watch mode skips actor derivation); `tuhdoo help|-h|--help` → usage to stdout exit 0; tombstone `case "top"`; usage rewritten.
4. In-place task detail screen: enter on task row (escalation row → its parent task) → modeDetail rendering via printTask/historyOf from the live snapshot (no new fetch); j/k scroll by manual line-windowing (no bubbles dep); esc back, q quit, one meaning per key everywhere; read-only this round.
5. Edge markers: flat list rows get dim suffixes `· N deps` / `· in <parent>`; no tree rendering (gated on edge-semantics question).

Acceptance:
- make test lint green after each commit; one commit per piece, named for it.
- One-shot commands (status/backlog/task/escalations) byte-identical.
- Tests: disarmed-mode key-deadness (successor of TestWatchQuitKeysOnly), Blocked section visible in watch mode (old watch never showed it), detail open/esc/q/scroll/live-refresh, bare-invocation non-TTY guard, tombstones, help.

Constraints: boring Go; no new dependencies; stored event bytes untouched (this is all CLI-side); no force-push.

## History

### 2026-07-30 19:02 UTC — note from `4099114+brandonbews`

Commits 1-3 landed green (606d0f0 docs revision, f85134f watch fold, aaad5b5 verb-less dispatch). Deviation from plan: the "uninitialized repo → run tuhdoo init hint" scenario doesn't exist — the daemon auto-creates the data branch on spawn (runInit is spawn+confirm), so only "not a git repo" errors, and openRepo's existing message covers it. Remaining: commit 4 (modeDetail screen: enter/esc, j/k scroll via line windowing, render via printTask from live snapshot) and commit 5 (edge-marker suffixes · N deps / · in <parent> on rows). Then push, update memory/docs? No — docs done in commit 1.

### 2026-07-30 19:07 UTC — run by `4099114+brandonbews` — done

- Branch: `main`
- Commits: `606d0f0`, `f85134f`, `aaad5b5`, `224eeb5`, `50ae2b4`

All five commits landed on main and pushed; make test lint green after each. The TUI is now bare `tuhdoo` (TTY-guarded; non-TTY gets usage exit 1), `-w/--watch` disarms it with a header badge, watch/top verbs are tombstones, header carries badge+sync+counts, enter opens an in-place read-only detail screen (esc back, q quit, j/k line-window scroll, live off the 2s snapshot), and rows carry edge-marker suffixes (· in <parent> · N deps). One planned scenario didn't exist: "uninitialized repo → init hint" — the daemon auto-creates the data branch, so only non-git-dir errors (openRepo's message covers it). One-shot commands untouched. Follow-ups already filed: t-01KYT63MB28Z535SMJCA63RQJM (armed detail), t-01KYT63MB28Z535SMJCBC7SY1P (tree rendering, gated on edge-semantics grill). Not manually smoke-tested in a real terminal — Brandon should run bare `tuhdoo` and `tuhdoo -w` in tmux once.
