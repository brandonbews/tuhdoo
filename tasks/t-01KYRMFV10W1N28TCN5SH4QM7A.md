# v1 milestone: steering surface and a second machine

`t-01KYRMFV10W1N28TCN5SH4QM7A`

- **Status:** done
- **Priority:** none
- **Labels:** `milestone`
- **Depends on:** [`t-gsw5`](t-01KYRMFV10W1N28TCN5NWAGSW5.md) (done), [`tuh-pk06`](tuh-01KZA0VT234XJYVZWT8S09PK06.md) (done)
- **Created:** 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: internal-docs/plan/roadmap.md v1. A milestone is a label, not a mechanism (001 D5, revised by the milestone grill 2026-08-03): its done-ness is declared by a human, never computed. This task is a container and a gate, not a unit of work — the v1 children carry parent edges into it (the steering TUI, done; the two-machine convergence harness, ready), and Epoch compaction (D9) depends on it, so declaring this done is what unparks compaction. It stays `held` until the evidence below exists; parked work is held, never fenced with a blocking escalation.

Definition of done (human-verified — mirrors internal-docs/plan/roadmap.md v1, which is authoritative if the two ever disagree):

1. a blocking escalation is raised by an agent, answered from the TUI, and picked up by a successor agent without the human touching git;
2. two daemons against one remote produce observed claim collisions with one winner each, a `superseded` run recorded for every loser, and byte-identical replayed state and views on both sides afterward;
3. Brandon's 5-person work team could be onboarded with `tuhdoo init` + docs alone (whether or not they are).

Clause 3 is a judgment call on purpose; mechanizing it would swap in a proxy.

Constraints: agents must not work this task. If you are an agent holding this claim, release it with reason "human-declared milestone".

## History

### 2026-08-03 20:06 UTC — edit by `brandon`

status open→held

### 2026-08-03 20:56 UTC — edit by `brandon`

description edited

### 2026-08-06 21:27 UTC — edit by `brandon/claude-code-1`

depends_on +tuh-pk06

### 2026-08-07 21:53 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-08-12 21:31 UTC — edit by `brandon/claude-code-1`

status held→done

### 2026-08-12 21:32 UTC — note from `brandon/claude-code-1`

Closed done 2026-08-12 at Brandon's direction, with his framing recorded verbatim in spirit: this is NOT a declaration that v1 is done as a phase — "i just dont need a v1 road sign at the moment." Evidence state at closure: clause 2 proven (collision harness 2026-08-03, extended 2026-08-05 to real D6 machinery + confirmation-race storm); clause 3 judged met by the same adoption surface that closed the launch epic (tuh-01KZEPBEE8HFDQVK96AV6RN0ER, declared done earlier today); clause 1 (escalation answered from the TUI, picked up by a successor, no human git) never formally round-tripped end-to-end. Roadmap.md carries the matching closure note. Epoch compaction (t-01KYRMFV10W1N28TCN62F6FRTH), formerly unparked by this milestone, was deliberately parked held the same day — it unparks on real repo-size pressure, not on this label.
