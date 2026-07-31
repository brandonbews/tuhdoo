package main

// The read commands, plus init. Each follows the same shape: locate the
// repo, ensure the daemon (auto-spawning it if needed), fetch, render.

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// connect is the common preamble: repo discovery plus a live daemon.
func connect() (*repo, *client, int) {
	r, err := openRepo()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo:", err)
		return nil, nil, 1
	}
	c, err := ensureDaemon(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo:", err)
		return nil, nil, 1
	}
	return r, c, 0
}

// ---- init ----

// runInit is idempotent repo setup. The data branch is created by the
// daemon's startup path (daemon.New runs store.Init before it binds the
// socket), so "spawn the daemon, then confirm the branch exists" is the
// whole job — the simplest honest path, and it means the first command
// a user ever runs brings everything up.
func runInit() int {
	r, c, code := connect()
	if code != 0 {
		return code
	}
	// Prove the daemon answers before declaring success.
	st, err := fetchState(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo init: daemon not answering:", err)
		return 1
	}
	head, err := r.headShort()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo init:", err)
		return 1
	}

	sync := syncLine(st.Sync)
	if st.Sync.Mode == "local-only" {
		sync = "local-only — no git remote is configured. That is a normal state:\n" +
			"               everything works locally; add a remote later and the\n" +
			"               daemon will publish the branch automatically."
	}

	fmt.Printf(`tuhdoo is set up in this repository.

  data branch  %s (head %s) — the coordination ledger, an orphan
               branch inside this repo; never checked out, never edited
               by hand.
  sync         %s
  daemon       running (socket %s)

CI guidance: exclude the %q branch from CI triggers — github-actions:
  on: { push: { branches-ignore: ["%s"] } }

Next: tuhdoo status · tuhdoo backlog · tuhdoo (the TUI)
`, branchName(), head, sync, c.socket, branchName(), branchName())
	return 0
}

// ---- status ----

func runStatus() int {
	r, c, code := connect()
	if code != 0 {
		return code
	}
	snap, err := fetchSnapshot(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo status:", err)
		return 1
	}
	head, err := r.headShort()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo status:", err)
		return 1
	}
	printStatus(os.Stdout, newColors(os.Stdout), head, snap)
	return 0
}

func printStatus(w io.Writer, col colors, head string, s *snapshot) {
	b := s.classify()
	fmt.Fprintf(w, "%ssync%s         %s\n", col.bold, col.reset, syncLine(s.state.Sync))
	fmt.Fprintf(w, "%sdata branch%s  %s @ %s\n", col.bold, col.reset, branchName(), head)
	if s.state.Degraded != "" {
		fmt.Fprintf(w, "%sDEGRADED%s     %s\n", col.red, col.reset, s.state.Degraded)
	}
	fmt.Fprintf(w, "%stasks%s        %s%d ready%s · %s%d in progress%s · %s%d blocked%s · %d done · %d cancelled\n",
		col.bold, col.reset,
		col.green, len(b.ready), col.reset,
		col.yellow, len(b.inProgress), col.reset,
		col.red, len(b.blocked), col.reset,
		len(b.done), len(b.cancelled))
	fmt.Fprintf(w, "%sescalations%s  %s open\n", col.bold, col.reset, plural(len(s.state.OpenEscalations), "question"))
	if len(b.inProgress) == 0 {
		fmt.Fprintf(w, "%sclaims%s       none active\n", col.bold, col.reset)
		return
	}
	fmt.Fprintf(w, "%sclaims%s\n", col.bold, col.reset)
	for _, t := range b.inProgress {
		fmt.Fprintf(w, "  %s%s%s  %s  %s← %s%s\n",
			col.dim, t.ID, col.reset, oneLine(t.Title), col.yellow, t.Holder, col.reset)
	}
}

// ---- backlog ----

func runBacklog() int {
	_, c, code := connect()
	if code != 0 {
		return code
	}
	snap, err := fetchSnapshot(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo backlog:", err)
		return 1
	}
	printBacklog(os.Stdout, newColors(os.Stdout), snap)
	return 0
}

func printBacklog(w io.Writer, col colors, s *snapshot) {
	b := s.classify()
	renderReady(w, col, b.ready)
	fmt.Fprintln(w)
	renderInProgress(w, col, b.inProgress)
	fmt.Fprintln(w)
	renderBlocked(w, col, s, b.blocked)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%sDone%s %d · %sCancelled%s %d\n",
		col.dim, col.reset, len(b.done), col.dim, col.reset, len(b.cancelled))
}

// ---- task <id> ----

func runTask(id string) int {
	_, c, code := connect()
	if code != 0 {
		return code
	}
	var h hydratedTask
	if err := c.get("/v0/tasks/"+id, &h); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo task:", err)
		return 1
	}
	printTask(os.Stdout, newColors(os.Stdout), h)
	return 0
}

func printTask(w io.Writer, col colors, h hydratedTask) {
	t := h.Task
	fmt.Fprintf(w, "%s%s%s — %s\n\n", col.bold, t.ID, col.reset, oneLine(t.Title))
	status := t.Status
	if h.Claim != nil {
		status += fmt.Sprintf(" — claimed by %s", h.Claim.Actor)
		if h.Claim.Expires != nil {
			status += fmt.Sprintf(" (lease expires %s)", stamp(*h.Claim.Expires))
		}
	}
	fmt.Fprintf(w, "  status      %s\n", status)
	fmt.Fprintf(w, "  priority    %d\n", t.Priority)
	if len(t.Labels) > 0 {
		fmt.Fprintf(w, "  labels      %s\n", strings.Join(t.Labels, ", "))
	}
	if len(t.Parents) > 0 {
		fmt.Fprintf(w, "  parents     %s\n", strings.Join(t.Parents, ", "))
	}
	if len(t.DependsOn) > 0 {
		fmt.Fprintf(w, "  depends on  %s\n", strings.Join(t.DependsOn, ", "))
	}
	fmt.Fprintf(w, "  created     %s by %s\n", stamp(t.CreatedAt), t.CreatedBy)

	fmt.Fprintf(w, "\n%sDescription%s\n", col.bold, col.reset)
	if t.Description == "" {
		fmt.Fprintf(w, "  %snone%s\n", col.dim, col.reset)
	} else {
		fmt.Fprintln(w, indent(t.Description, "  "))
	}

	fmt.Fprintf(w, "\n%sHistory%s\n", col.bold, col.reset)
	entries := historyOf(col, h)
	if len(entries) == 0 {
		fmt.Fprintf(w, "  %sno activity yet%s\n", col.dim, col.reset)
	}
	for _, e := range entries {
		fmt.Fprint(w, e.text)
	}
}

// histEntry is one history item; id is its event ULID, the
// chronological sort key every machine agrees on.
type histEntry struct {
	id   string
	text string
}

// historyOf merges a task's notes, runs, and escalations into one
// chronological (ULID-ordered) sequence, mirroring internal/views.
func historyOf(col colors, h hydratedTask) []histEntry {
	var out []histEntry
	for _, n := range h.Notes {
		text := fmt.Sprintf("  %s%s%s  note by %s\n%s\n",
			col.dim, stamp(n.AddedAt), col.reset, n.Actor, indent(n.Text, "    "))
		out = append(out, histEntry{n.ID, text})
	}
	for _, r := range h.Runs {
		var b strings.Builder
		fmt.Fprintf(&b, "  %s%s%s  run by %s — %s%s%s\n",
			col.dim, idStamp(r.ID), col.reset, r.Actor, col.bold, r.Outcome, col.reset)
		var links []string
		if r.Branch != "" {
			links = append(links, "branch "+r.Branch)
		}
		if r.PR != "" {
			links = append(links, "pr "+r.PR)
		}
		if len(r.Commits) > 0 {
			links = append(links, "commits "+strings.Join(r.Commits, ", "))
		}
		if len(links) > 0 {
			fmt.Fprintf(&b, "    %s\n", strings.Join(links, " · "))
		}
		if r.Summary != "" {
			fmt.Fprintf(&b, "%s\n", indent(r.Summary, "    "))
		}
		if r.Synthesized {
			fmt.Fprintf(&b, "    %ssynthesized by replay, not recorded by the agent%s\n", col.dim, col.reset)
		}
		out = append(out, histEntry{r.ID, b.String()})
	}
	for _, e := range h.Escalations {
		var b strings.Builder
		mark := ""
		if e.Blocking {
			mark = fmt.Sprintf(" %s[blocking]%s", col.red, col.reset)
		}
		fmt.Fprintf(&b, "  %s%s%s  escalation from %s%s\n",
			col.dim, stamp(e.RaisedAt), col.reset, e.Actor, mark)
		fmt.Fprintf(&b, "    Q: %s\n", oneLine(e.Question))
		if e.Context != "" {
			fmt.Fprintf(&b, "%s\n", indent(e.Context, "       "))
		}
		if e.Answered {
			fmt.Fprintf(&b, "    A (%s): %s\n", answererLabel(e), oneLine(e.Answer))
		} else {
			fmt.Fprintf(&b, "    %sunanswered%s\n", col.dim, col.reset)
		}
		out = append(out, histEntry{e.ID, b.String()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

// idStamp derives an event's instant from its ULID (runs carry no
// timestamp field of their own).
func idStamp(id string) string {
	t, err := event.IDTime(id)
	if err != nil {
		return "(unknown time)"
	}
	return stamp(t)
}

// ---- escalations ----

func runEscalations() int {
	_, c, code := connect()
	if code != 0 {
		return code
	}
	snap, err := fetchSnapshot(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo escalations:", err)
		return 1
	}
	printEscalations(os.Stdout, newColors(os.Stdout), snap)
	return 0
}

func printEscalations(w io.Writer, col colors, s *snapshot) {
	var open, answered []escalationJSON
	for _, e := range s.allEscalations() {
		if e.Answered {
			answered = append(answered, e)
		} else {
			open = append(open, e)
		}
	}
	renderOpenEscalations(w, col, s, open)
	fmt.Fprintf(w, "\n%sAnswered%s (%d)\n", col.bold, col.reset, len(answered))
	if len(answered) == 0 {
		fmt.Fprintf(w, "  %snone%s\n", col.dim, col.reset)
		return
	}
	for _, e := range answered {
		fmt.Fprintf(w, "  %s (%s, raised %s) — %s: %s\n",
			oneLine(e.Question), e.Task, stamp(e.RaisedAt), answererLabel(e), oneLine(e.Answer))
	}
}

// answererLabel attributes an answer, marking the out-of-band path
// where an agent relayed it on the answerer's behalf.
func answererLabel(e escalationJSON) string {
	if e.RelayedBy == "" {
		return e.AnsweredBy
	}
	return fmt.Sprintf("%s, relayed by %s", e.AnsweredBy, e.RelayedBy)
}
