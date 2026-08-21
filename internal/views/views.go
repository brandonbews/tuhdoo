// Package views renders core.State into the generated markdown views on
// the data branch (002 T6). Every function here is pure: same State in,
// byte-identical views out, on every machine, forever. That is what the
// package trades everything else for — no clocks (so no relative times),
// no map iteration (all walks go through the State's order slices or an
// explicit sort), no template engine, no I/O.
//
// The design target (2026-07-31) is rendered markdown on a git host:
// someone glancing at the branch on GitHub. README.md is the landing
// page (a needs-input callout plus one-row counts), escalations.md is
// the steering inbox (questions a human can answer, blocking first),
// backlog.md orders actionable states before the closed ones. Raw-terminal
// legibility is best-effort; terminal readers have the CLI and TUI.
package views

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
)

// FormatVersion is the view-format contract of T3's three-contract
// taxonomy: a single integer, bumped whenever rendered output changes.
// Peers compare it via CanWrite before regenerating (highest wins).
// 2: relayed escalation answers carry "(relayed by ...)" attribution.
// 3: held and inbox statuses render as their own backlog sections.
// 4: the held status renders as "on hold" everywhere humans read it.
// 5: rendered-first redesign — short IDs, at-a-glance counts, escalations
// as answerable questions, cancelled reads "archived".
// 6: status-vocabulary revision (2026-08-01) — displayed words are the
// stored words: cancelled renders as "cancelled" (archive retired);
// "on hold" for held stays as the one sanctioned display mapping.
// 7: blocked rows mark dependency-loop members and cancelled
// dependencies (2026-08-05 edge grill).
// 8: task edits render in history (2026-08-11 grill) — every
// task.updated is an entry: actor, stamp, compact per-field summary.
// 9: README de-assumes the host repo (2026-08-11 grill) — the "design
// lives in `docs/`" sentence becomes a https://tuhdoo.com pointer, and
// the fixed prose is tightened to the utilitarian bar.
// 10: P0-highest priority flip (2026-08-21) — ready sorts most-urgent
// first (0 on top), and unprioritized tasks (no priority set) sort
// last and render "—" in priority cells, "none" on task pages.
const FormatVersion = 10

// MetaPath is where the view-format stamp lives. T6 named "views/.meta",
// but the views render at the branch root (README.md and friends), so
// the stamp lives at the root too.
const MetaPath = ".views-meta.json"

// Render produces every view file as path → bytes. Paths are relative to
// the data-branch root: README.md, backlog.md, escalations.md, one
// tasks/<id>.md per task, and the MetaPath stamp.
func Render(s *core.State) map[string][]byte {
	b := classify(s)
	out := make(map[string][]byte, len(s.TaskOrder)+4)
	out["README.md"] = readme(s, b)
	out["backlog.md"] = backlog(s, b)
	out["escalations.md"] = escalations(s)
	for _, id := range s.TaskOrder {
		out["tasks/"+id+".md"] = taskPage(s, s.Tasks[id])
	}
	out[MetaPath] = []byte(fmt.Sprintf("{\"format\":%d}\n", FormatVersion))
	return out
}

// CanWrite reports whether this generator may overwrite the existing
// views, given the current meta stamp's bytes (nil/empty when absent).
// Highest version wins (T6): a stamp declaring a HIGHER format means a
// newer peer owns the views — the daemon must write events only and let
// that peer regenerate. An absent or unreadable stamp never blocks:
// refusing to write over garbage would wedge view generation forever,
// and the guard exists to prevent ping-pong between versions, not to
// authenticate the stamp.
func CanWrite(existingMeta []byte) bool {
	return Format(existingMeta) <= FormatVersion
}

// Format parses a stamp's declared view-format version. Absent or
// unparseable metadata is 0: any writer may replace it. The sync layer
// compares two peers' stamps with this to decide whose views win a
// merge.
func Format(meta []byte) int {
	var m struct {
		Format int `json:"format"`
	}
	if err := json.Unmarshal(meta, &m); err != nil {
		return 0
	}
	return m.Format
}

// buckets is the backlog partition of tasks. Every open task lands in
// exactly one of ready / inProgress / blocked; held and inbox tasks
// (2026-07-31) shelve separately — parked and captured work, never
// claimable.
type buckets struct {
	ready      []*core.Task // claimable now, highest priority first
	inProgress []*core.Task // actively claimed, creation order
	blocked    []*core.Task // open but not claimable, creation order
	held       []*core.Task // triaged, deliberately paused; creation order
	inbox      []*core.Task // untriaged captures; creation order
	done       []*core.Task
	cancelled  []*core.Task
}

func classify(s *core.State) buckets {
	var b buckets
	for _, id := range s.TaskOrder {
		t := s.Tasks[id]
		switch s.Situation(id) {
		case core.SituationReady:
			b.ready = append(b.ready, t)
		case core.SituationInProgress:
			b.inProgress = append(b.inProgress, t)
		case core.SituationBlocked:
			b.blocked = append(b.blocked, t)
		case core.StatusDone:
			b.done = append(b.done, t)
		case core.StatusCancelled:
			b.cancelled = append(b.cancelled, t)
		case core.StatusHeld:
			b.held = append(b.held, t)
		case core.StatusInbox:
			b.inbox = append(b.inbox, t)
		}
	}
	// Most urgent first (P0-highest, 2026-08-21), creation (ULID) order
	// within a rank — the same ordering core.ReadyTasks serves
	// claim_next from.
	sort.SliceStable(b.ready, func(i, j int) bool {
		return core.MoreUrgent(b.ready[i].Priority, b.ready[j].Priority)
	})
	return b
}

func readme(s *core.State, b buckets) []byte {
	var w strings.Builder
	w.WriteString("# tuhdoo data branch\n\n")
	w.WriteString("This branch is the coordination ledger for this repository. It is kept by\n")
	w.WriteString("**tuhdoo**, a shared backlog, work queue, and activity ledger for a fleet of\n")
	w.WriteString("agents steered by humans, synced over the repository's existing git remote.\n")
	w.WriteString("The branch holds the append-only event log under `events/` and these\n")
	w.WriteString("generated markdown views. The views are derived from the log; never edit\n")
	w.WriteString("them by hand. Docs: https://tuhdoo.com\n\n")

	w.WriteString("## At a glance\n\n")
	w.WriteString(needsHuman(len(s.OpenEscalations())))
	w.WriteString("| In progress | Ready | Blocked | On hold | Inbox | Done | Cancelled |\n")
	w.WriteString("|---:|---:|---:|---:|---:|---:|---:|\n")
	fmt.Fprintf(&w, "| %d | %d | %d | %d | %d | %d | %d |\n\n",
		len(b.inProgress), len(b.ready), len(b.blocked),
		len(b.held), len(b.inbox), len(b.done), len(b.cancelled))

	w.WriteString("- [backlog.md](backlog.md) lists every task, grouped by state.\n")
	w.WriteString("- [escalations.md](escalations.md) lists the questions agents have raised for a human to answer.\n")
	w.WriteString("- `tasks/` holds one page per task, with its description, status, and full history.\n\n")
	fmt.Fprintf(&w, "Generated by tuhdoo (view format %d).\n", FormatVersion)
	return []byte(w.String())
}

// needsHuman is the steering callout: bold when there is something to
// answer, a quiet all-clear when there is not. Always one line plus a
// blank separator.
func needsHuman(n int) string {
	switch n {
	case 0:
		return "No open questions are waiting; the fleet is unblocked.\n\n"
	case 1:
		return "**[1 open question](escalations.md) is waiting on a human.**\n\n"
	default:
		return fmt.Sprintf("**[%d open questions](escalations.md) are waiting on a human.**\n\n", n)
	}
}

func backlog(s *core.State, b buckets) []byte {
	var w strings.Builder
	w.WriteString("# Backlog\n\n")
	fmt.Fprintf(&w, "%d in progress · %d ready · %d blocked · %d on hold · %d inbox · %d done · %d cancelled\n\n",
		len(b.inProgress), len(b.ready), len(b.blocked),
		len(b.held), len(b.inbox), len(b.done), len(b.cancelled))
	if n := len(s.OpenEscalations()); n > 0 {
		w.WriteString(needsHuman(n))
	}

	w.WriteString("## In progress\n\n")
	if len(b.inProgress) == 0 {
		w.WriteString("_None._\n\n")
	} else {
		w.WriteString("| ID | Task | Priority | Claimed by |\n|---|---|---:|---|\n")
		for _, t := range b.inProgress {
			fmt.Fprintf(&w, "| %s | %s | %s | `%s` |\n",
				rootLink(t.ID), inline(t.Title), priorityCell(t.Priority), s.ActiveClaim(t.ID).Actor)
		}
		w.WriteString("\n")
	}

	w.WriteString("## Ready\n\n")
	if len(b.ready) == 0 {
		w.WriteString("_None._\n\n")
	} else {
		w.WriteString("| ID | Task | Priority | Labels |\n|---|---|---:|---|\n")
		for _, t := range b.ready {
			fmt.Fprintf(&w, "| %s | %s | %s | %s |\n",
				rootLink(t.ID), inline(t.Title), priorityCell(t.Priority), labelSpans(t.Labels))
		}
		w.WriteString("\n")
	}

	w.WriteString("## Blocked / waiting\n\n")
	if len(b.blocked) == 0 {
		w.WriteString("_None._\n\n")
	} else {
		w.WriteString("| ID | Task | Priority | Waiting on |\n|---|---|---:|---|\n")
		for _, t := range b.blocked {
			fmt.Fprintf(&w, "| %s | %s | %s | %s |\n",
				rootLink(t.ID), inline(t.Title), priorityCell(t.Priority), waitingOn(s, t))
		}
		w.WriteString("\n")
	}

	// Held above Inbox (2026-07-31): held items passed triage and sit
	// closer to workable than raw captures do.
	w.WriteString("## On hold\n\n")
	if len(b.held) == 0 {
		w.WriteString("_None._\n\n")
	} else {
		w.WriteString("Triaged, deliberately paused — never served to agents until reopened.\n\n")
		w.WriteString("| ID | Task | Priority | Labels |\n|---|---|---:|---|\n")
		for _, t := range b.held {
			fmt.Fprintf(&w, "| %s | %s | %s | %s |\n",
				rootLink(t.ID), inline(t.Title), priorityCell(t.Priority), labelSpans(t.Labels))
		}
		w.WriteString("\n")
	}

	w.WriteString("## Inbox\n\n")
	if len(b.inbox) == 0 {
		w.WriteString("_None._\n\n")
	} else {
		w.WriteString("Untriaged captures — promoting one to open means writing it a real (prompt-quality) description first.\n\n")
		compactList(&w, b.inbox)
		w.WriteString("\n")
	}

	w.WriteString("## Done\n\n")
	compactList(&w, b.done)
	w.WriteString("\n## Cancelled\n\n")
	compactList(&w, b.cancelled)
	return []byte(w.String())
}

func compactList(w *strings.Builder, tasks []*core.Task) {
	if len(tasks) == 0 {
		w.WriteString("_None._\n")
		return
	}
	for _, t := range tasks {
		fmt.Fprintf(w, "- %s %s\n", rootLink(t.ID), inline(t.Title))
	}
}

// waitingOn names why a blocked task cannot be claimed: each unmet
// dependency, and/or an open blocking escalation. The escalation's
// question is not repeated here — blocked rows link to the steering
// inbox instead (the same doctrine the TUI settled: rows stop
// repeating the question). The loud annotations (2026-08-05 edge
// grill) render distinctly from ordinary waiting: loop membership
// leads the cell — a cyclic task can never become ready on its own —
// and a cancelled dependency reads "waiting on cancelled", because
// re-pointing that edge is a human decision.
func waitingOn(s *core.State, t *core.Task) string {
	b := s.Blockage(t.ID)
	var parts []string
	if b.Cyclic {
		parts = append(parts, "**cyclic** — a human must cut an edge")
	}
	for _, dep := range b.UnmetDeps {
		if slices.Contains(b.CancelledDeps, dep) {
			parts = append(parts, "waiting on cancelled "+rootLink(dep))
		} else {
			parts = append(parts, "depends on "+rootLink(dep))
		}
	}
	if len(b.BlockingEscalations) > 0 {
		parts = append(parts, "an [open question](escalations.md)")
	}
	return strings.Join(parts, "; ")
}

func escalations(s *core.State) []byte {
	var w strings.Builder
	w.WriteString("# Escalations\n\n")
	w.WriteString("The steering inbox: questions raised by agents, awaiting a human answer.\n\n")

	w.WriteString("## Open\n\n")
	open := s.OpenEscalations()
	if len(open) == 0 {
		w.WriteString("_None — the fleet is unblocked._\n\n")
	}
	// Blocking questions gate work out of the ready pool; they outrank
	// non-blocking ones. Raise (ULID) order within each group.
	for _, e := range open {
		if e.Blocking {
			openEscalation(&w, s, e)
		}
	}
	for _, e := range open {
		if !e.Blocking {
			openEscalation(&w, s, e)
		}
	}

	w.WriteString("## Answered\n\n")
	answered := 0
	for _, id := range s.EscOrder {
		e := s.Escalations[id]
		if !e.Answered {
			continue
		}
		if answered > 0 {
			w.WriteString("\n")
		}
		answered++
		fmt.Fprintf(&w, "### %s · %s\n\n", rootLink(e.Task), inline(s.Tasks[e.Task].Title))
		fmt.Fprintf(&w, "Asked by `%s` · %s\n\n", e.Actor, stamp(e.RaisedAt))
		quoteBlock(&w, e.Question)
		w.WriteString("\n")
		writeBlock(&w, fmt.Sprintf("**Answer** (`%s`%s): %s", e.AnsweredBy, relaySuffix(e), e.Answer))
	}
	if answered == 0 {
		w.WriteString("_None._\n")
	}
	return []byte(w.String())
}

// openEscalation renders one open question: which task it fences (the
// heading), how urgent it is and who asks (the meta line), the question
// itself (a blockquote — the thing a human answers), then its context.
func openEscalation(w *strings.Builder, s *core.State, e *core.Escalation) {
	fmt.Fprintf(w, "### %s · %s\n\n", rootLink(e.Task), inline(s.Tasks[e.Task].Title))
	urgency := "Non-blocking"
	if e.Blocking {
		urgency = "**Blocking**"
	}
	fmt.Fprintf(w, "%s · asked by `%s` · %s\n\n", urgency, e.Actor, stamp(e.RaisedAt))
	quoteBlock(w, e.Question)
	w.WriteString("\n")
	if e.Context != "" {
		writeBlock(w, e.Context)
		w.WriteString("\n")
	}
}

// priorityCell renders a nullable priority for backlog table cells:
// the number, or an em dash for unprioritized (P0-highest flip,
// 2026-08-21 — absent means "no priority", never 0).
func priorityCell(p *int) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%d", *p)
}

// priorityWord is priorityCell for prose lines: "none" reads better
// than a dash after a bold field name.
func priorityWord(p *int) string {
	if p == nil {
		return "none"
	}
	return fmt.Sprintf("%d", *p)
}

func taskPage(s *core.State, t *core.Task) []byte {
	var w strings.Builder
	fmt.Fprintf(&w, "# %s\n\n", inline(t.Title))
	fmt.Fprintf(&w, "`%s`\n\n", t.ID)
	fmt.Fprintf(&w, "- **Status:** %s\n", statusLine(s, t))
	fmt.Fprintf(&w, "- **Priority:** %s\n", priorityWord(t.Priority))
	if len(t.Labels) > 0 {
		fmt.Fprintf(&w, "- **Labels:** %s\n", labelSpans(t.Labels))
	}
	if len(t.DependsOn) > 0 {
		fmt.Fprintf(&w, "- **Depends on:** %s\n", depLinks(s, t.DependsOn))
	}
	fmt.Fprintf(&w, "- **Created:** %s by `%s`\n\n", stamp(t.CreatedAt), t.CreatedBy)

	w.WriteString("## Description\n\n")
	if t.Description == "" {
		w.WriteString("_No description._\n")
	} else {
		writeBlock(&w, t.Description) // verbatim: descriptions are prompts (T5)
	}

	w.WriteString("\n## History\n\n")
	entries := history(s, t.ID)
	if len(entries) == 0 {
		w.WriteString("_No activity yet._\n")
	}
	for i, en := range entries {
		if i > 0 {
			w.WriteString("\n")
		}
		w.WriteString(en.text)
	}
	return []byte(w.String())
}

func statusLine(s *core.State, t *core.Task) string {
	switch s.Situation(t.ID) {
	case core.StatusDone:
		return "done"
	case core.StatusCancelled:
		return "cancelled"
	case core.StatusHeld:
		return HumanStatus(core.StatusHeld) + " — deliberately paused"
	case core.StatusInbox:
		return "inbox — untriaged capture"
	case core.SituationInProgress:
		return fmt.Sprintf("open — in progress, claimed by `%s`", s.ActiveClaim(t.ID).Actor)
	case core.SituationReady:
		return "open — ready"
	}
	// Blocked: an open escalation outranks dependencies in the telling —
	// it names the human who can unblock.
	if _, escs := s.ClaimBlockers(t.ID); len(escs) > 0 {
		return "open — waiting on an escalation answer"
	}
	return "open — blocked on dependencies"
}

// entry is one history item; id is its event ULID, the chronological
// sort key every machine agrees on.
type entry struct {
	id   string
	text string
}

// history merges a task's notes, runs, escalations, and edits into one
// chronological (ULID-ordered) sequence of rendered blocks.
func history(s *core.State, taskID string) []entry {
	var out []entry
	for i := range s.Notes {
		if n := &s.Notes[i]; n.Task == taskID {
			out = append(out, entry{n.ID, noteEntry(n)})
		}
	}
	for i := range s.Updates {
		if u := &s.Updates[i]; u.Task == taskID {
			out = append(out, entry{u.ID, updateEntry(u)})
		}
	}
	for i := range s.Runs {
		if r := &s.Runs[i]; r.Task == taskID {
			out = append(out, entry{r.ID, runEntry(r)})
		}
	}
	for _, id := range s.EscOrder {
		if e := s.Escalations[id]; e.Task == taskID {
			out = append(out, entry{e.ID, escEntry(e)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

func noteEntry(n *core.Note) string {
	var w strings.Builder
	fmt.Fprintf(&w, "### %s — note from `%s`\n\n", stamp(n.AddedAt), n.Actor)
	writeBlock(&w, n.Text)
	return w.String()
}

// updateEntry renders one task edit: actor, stamp, and the compact
// per-field summaries core recorded at replay (2026-08-11 grill —
// every edit gets an entry). A summary-less entry (an event that wrote
// no fields) keeps its heading: the edit still happened.
func updateEntry(u *core.Update) string {
	var w strings.Builder
	fmt.Fprintf(&w, "### %s — edit by `%s`\n", idStamp(u.ID), u.Actor)
	if len(u.Fields) > 0 {
		w.WriteString("\n")
		writeBlock(&w, strings.Join(u.Fields, " · "))
	}
	return w.String()
}

func runEntry(r *core.Run) string {
	var w strings.Builder
	fmt.Fprintf(&w, "### %s — run by `%s` — %s\n\n", idStamp(r.ID), r.Actor, r.Outcome)
	links := false
	if r.Branch != "" {
		fmt.Fprintf(&w, "- Branch: `%s`\n", r.Branch)
		links = true
	}
	if r.PR != "" {
		fmt.Fprintf(&w, "- PR: <%s>\n", r.PR)
		links = true
	}
	if len(r.Commits) > 0 {
		fmt.Fprintf(&w, "- Commits: `%s`\n", strings.Join(r.Commits, "`, `"))
		links = true
	}
	if len(r.MergedAs) > 0 {
		fmt.Fprintf(&w, "- Merged as: `%s`\n", strings.Join(r.MergedAs, "`, `"))
		links = true
	}
	if links {
		w.WriteString("\n")
	}
	if r.Summary != "" {
		writeBlock(&w, r.Summary)
	}
	if r.Synthesized {
		w.WriteString("\n_Synthesized by replay, not recorded by the agent._\n")
	}
	return w.String()
}

func escEntry(e *core.Escalation) string {
	var w strings.Builder
	blocking := ""
	if e.Blocking {
		blocking = " (blocking)"
	}
	fmt.Fprintf(&w, "### %s — escalation from `%s`%s\n\n", stamp(e.RaisedAt), e.Actor, blocking)
	quoteBlock(&w, e.Question)
	if e.Context != "" {
		w.WriteString("\n")
		writeBlock(&w, e.Context)
	}
	if e.Answered {
		w.WriteString("\n")
		writeBlock(&w, fmt.Sprintf("**Answer** (`%s`%s): %s", e.AnsweredBy, relaySuffix(e), e.Answer))
	} else {
		w.WriteString("\n_Unanswered._\n")
	}
	return w.String()
}

// relaySuffix marks an answer recorded on the answerer's behalf — the
// out-of-band path, where an agent is the scribe (T5 relay_answer).
func relaySuffix(e *core.Escalation) string {
	if e.RelayedBy == "" {
		return ""
	}
	return fmt.Sprintf(", relayed by `%s`", e.RelayedBy)
}

// rootLink links a task from a branch-root view: short ID in a code
// span, full ID in the target path.
func rootLink(id string) string {
	return fmt.Sprintf("[`%s`](tasks/%s.md)", event.ShortID(id), id)
}

// depLinks links tasks from within tasks/ (same directory), plus each
// dependency's status, so a reader sees at a glance which prerequisites
// still gate this task.
func depLinks(s *core.State, ids []string) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("[`%s`](%s.md)", event.ShortID(id), id)
		if d, ok := s.Tasks[id]; ok {
			parts[i] += " (" + HumanStatus(d.Status) + ")"
		}
	}
	return strings.Join(parts, ", ")
}

// HumanStatus maps a stored status to the word humans read. Since the
// status-vocabulary revision (2026-08-01) the displayed words are the
// stored words with exactly one exception: held reads "on hold". This
// is the mapping's only definition — every display surface (markdown
// views, CLI, TUI) calls it here.
func HumanStatus(status string) string {
	if status == core.StatusHeld {
		return "on hold"
	}
	return status
}

// labelSpans renders labels as individual code spans — the closest a
// git host renders to badges without HTML.
func labelSpans(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return "`" + strings.Join(labels, "` `") + "`"
}

// stamp renders an absolute UTC instant. Relative times ("3 hours ago")
// are banned: they read a clock and rot the moment they are written.
func stamp(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

// idStamp derives an event's instant from its ULID. Replay has already
// validated every ID it stores, so the error arm is vestigial.
func idStamp(id string) string {
	t, err := event.IDTime(id)
	if err != nil {
		return "(unknown time)"
	}
	return stamp(t)
}

// inline flattens text for a table cell, heading, or one-line list item:
// newlines collapse to spaces and pipes are escaped so GitHub's table
// parser keeps rows intact.
func inline(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", "\\|")
}

// writeBlock writes multi-line body text verbatim, guaranteeing exactly
// one trailing newline.
func writeBlock(w *strings.Builder, text string) {
	w.WriteString(strings.TrimRight(text, "\n"))
	w.WriteString("\n")
}

// quoteBlock writes text as a markdown blockquote, line by line — the
// rendered-first shape for a question a human is being asked to answer.
func quoteBlock(w *strings.Builder, text string) {
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if line == "" {
			w.WriteString(">\n")
		} else {
			w.WriteString("> " + line + "\n")
		}
	}
}
