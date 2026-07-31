# t-01KYVE848CJZNG5VFWZ9J3WRKM — Brand the task IDs: mint tuh-, accept both prefixes, age out t-

- Status: done
- Priority: 1
- Labels: `cli`, `tui`, `ux`, `design`
- Depends on: [t-01KYVD31CNTR1EVCDHPGZFQ5EV](t-01KYVD31CNTR1EVCDHPGZFQ5EV.md) (done)
- Created: 2026-07-31 06:37 UTC by `brandon/claude-code-3`

## Description

Context: steering decision (Brandon + session, 2026-07-30), reached through the to-brand-or-not-to-brand lane: a prefix must exist for shape (a bare 4-char short form like d83w does not read as a reference), so the letters should carry brand, and the brand metric is distinctiveness — tuh- is uniquely tuhdoo (td- reads as generic todo and collides with HTML <td>; no-prefix loses ID shape). Decision: new tasks mint with the tuh- prefix; existing t- IDs are never rewritten (T3) and simply age out of view at dogfood pace. Short form follows the ID's own prefix: tuh-d83w for new tasks, t-d83w for old ones.

The ask:
1. Minting: the task-ID prefix constant (internal/daemon/ops.go, currently "t-" + ULID) becomes "tuh-".
2. Prefix-agnostic surfaces: audit every place that assumes the literal t- prefix — shortID() (cmd/tuhdoo/top.go) already reuses the ID's own prefix via the first hyphen, and fragment matching (idMatches in cmd/tuhdoo/commands.go) already strips the ID's own prefix generically; verify both against tuh- IDs with tests, and grep for any validation, routing, or parsing that pins ^t- (API task routes, ValidateTaskID-style checks, MCP arg validation) and make it accept both prefixes.
3. Input: tuhdoo task accepts fragments with either prefix or none, per the existing resolution rules; a fragment's prefix must not exclude matches from the other prefix era when the tail matches (decide and test the cross-prefix rule: recommended — prefix in the fragment is matched literally against the ID, so t-d83w and tuh-d83w are distinct, bare d83w matches both eras).
4. Docs: dated note in 002 T7 extending the short-ID contract with the branding decision and the mixed-prefix window (mint-forward, no migration, old IDs never rewritten).

Acceptance: a task created after this lands has a tuh-01… ID end to end (MCP create through TUI display through data branch); short forms render tuh-xxxx for new and t-xxxx for old tasks side by side; resolution tests cover both-era fragments and the ambiguity rule; no code path rejects either prefix; make test lint green; stored bytes untouched.

Constraints: boring Go; no event rewrites, no migration of existing IDs; the eleven-verb MCP surface unchanged.

## History

### 2026-07-31 15:21 UTC — run by `brandon/claude-code-1` — done

- Branch: `main`
- Commits: `27bca13`

Landed in commit 27bca13. Audit found exactly one non-test code path pinning the prefix: the mint site in internal/daemon/ops.go, now a taskIDPrefix constant set to "tuh-" with a T7/T3 comment. API routes resolve IDs by pure map lookup (no shape validation), MCP arg validation has no task-ID shape check, shortID() and idMatches() were already prefix-agnostic via the ID's own first hyphen — verified with new tests rather than changed. Cross-prefix rule implemented as recommended and it fell out of the existing generic code with zero code changes: a prefixed fragment matches its era literally (t-d83w never matches a tuh- task — a leftover hyphen can't occur in a ULID tail), bare d83w matches both eras with ambiguity errors on tail collisions (TestResolveTaskIDCrossPrefix seeds identical tails across eras). End-to-end verified in a scratch repo with a scratch-built binary and its own daemon (created tuh-01… via API, backlog/task/short-form resolution all correct, event bytes and view filenames carry tuh-; scratch daemon killed and dir removed — the live daemon was never touched and is still the original pid). Dated T7 note extends the short-ID contract with the branding decision and the mint-forward mixed-prefix window. cli_test.go's ambiguous-fragment probe deliberately split: tuh-0 ambiguous, t-0 now asserts unknown against an all-tuh- repo. make test lint green. NOTE: the live daemon still mints t- until the wrap-up deploy (rebuild + restart) happens at the end of this drain session. Possible follow-up if dogfood friction appears: a "did you mean tuh-…?" hint when a t- fragment misses.
