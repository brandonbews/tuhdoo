package main

// The read commands, plus init. Each follows the same shape: locate the
// repo, ensure the daemon (auto-spawning it if needed), fetch, render.

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/views"
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
	fmt.Fprintf(w, "%stasks%s        %s%d ready%s · %s%d in progress%s · %s%d blocked%s · %d on hold · %d inbox · %d done · %d cancelled\n",
		col.bold, col.reset,
		col.green, len(b.ready), col.reset,
		col.yellow, len(b.inProgress), col.reset,
		col.red, len(b.blocked), col.reset,
		len(b.held), len(b.inbox),
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
	printBacklog(os.Stdout, snap)
	return 0
}

// printBacklog emits one row per task under a header: serialization,
// not design (T7, 2026-07-31). Plain aligned columns via tabwriter, no
// ANSI ever — the bytes are identical to a terminal and to a pipe — and
// a STATE column instead of section headers, so `tuhdoo backlog | grep
// ready` selects exactly the ready rows. Row order mirrors the old
// sections (ready by priority, then in-progress, blocked, on-hold,
// inbox, done, cancelled; creation order within each); full IDs
// throughout — scriptable plumbing.
func printBacklog(w io.Writer, s *snapshot) {
	b := s.classify()
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "ID\tSTATE\tPRI\tHOLDER\tLABELS\tWAITING\tTITLE")
	row := func(t stateTask, state, waiting string) {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			t.ID, state, t.Priority, cell(t.Holder),
			cell(strings.Join(t.Labels, ",")), waiting, oneLine(t.Title))
	}
	for _, t := range b.ready {
		row(t, "ready", "-")
	}
	for _, t := range b.inProgress {
		row(t, "in-progress", "-")
	}
	for _, t := range b.blocked {
		row(t, "blocked", s.waitingOn(t.ID))
	}
	for _, t := range b.held {
		row(t, "on-hold", "-")
	}
	for _, t := range b.inbox {
		row(t, "inbox", "-")
	}
	for _, t := range b.done {
		row(t, "done", "-")
	}
	for _, t := range b.cancelled {
		row(t, "cancelled", "-")
	}
	tw.Flush()
}

// newTabWriter is the one aligned-column configuration both serialized
// commands share: two-space gutters, no minimum width, spaces only.
func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
}

// cell renders one column value, "-" standing in for empty so every
// column is present on every row.
func cell(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

// ---- task <id> ----

func runTask(id string) int {
	_, c, code := connect()
	if code != 0 {
		return code
	}
	// Resolve short forms and fragments against the live task list
	// before fetching — input sugar only (T7): a full ID passes through
	// as itself and the rendered output is untouched.
	st, err := fetchState(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo task:", err)
		return 1
	}
	full, err := resolveTaskID(id, st.Tasks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo task:", err)
		return 1
	}
	var h hydratedTask
	if err := c.get("/v0/tasks/"+full, &h); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo task:", err)
		return 1
	}
	printTask(os.Stdout, newColors(os.Stdout), h)
	return 0
}

// resolveTaskID maps human input — a full ID, the short form, or any
// unambiguous ID fragment — to one known task ID, the git model (T7):
// the long ULID is plumbing, the short form is the human contract.
// Ambiguity is an error that lists the candidates; nothing is guessed.
func resolveTaskID(frag string, tasks []stateTask) (string, error) {
	if frag == "" {
		return "", fmt.Errorf("empty task id")
	}
	var cands []stateTask
	for _, t := range tasks {
		if strings.EqualFold(t.ID, frag) {
			return t.ID, nil // exact full ID wins outright
		}
		if idMatches(t.ID, frag) {
			cands = append(cands, t)
		}
	}
	switch len(cands) {
	case 1:
		return cands[0].ID, nil
	case 0:
		return "", fmt.Errorf("unknown task %q — no task ID matches", frag)
	}
	lines := make([]string, len(cands))
	for i, t := range cands {
		lines[i] = fmt.Sprintf("  %s  %s  (%s)", event.ShortID(t.ID), oneLine(t.Title), t.ID)
	}
	return "", fmt.Errorf("%q is ambiguous — %d tasks match:\n%s",
		frag, len(cands), strings.Join(lines, "\n"))
}

// idMatches reports whether frag — case-insensitive, with or without
// the type prefix — is a substring of id's tail. The short form is one
// such substring (the tail's last four), so it needs no special case.
// Only the ID's own prefix is stripped from the fragment, which is the
// cross-prefix rule for the tuh-/t- era split (T7, 2026-07-31): a
// prefixed fragment matches its own era literally (`t-d83w` never
// matches a `tuh-` task and vice versa — a leftover hyphen can't occur
// in a ULID tail), while a bare fragment (`d83w`) matches both eras.
func idMatches(id, frag string) bool {
	f, l := strings.ToLower(frag), strings.ToLower(id)
	i := strings.Index(l, "-") + 1
	return strings.Contains(l[i:], strings.TrimPrefix(f, l[:i]))
}

// printTask renders one task's full biography the way the one-shot
// `tuhdoo task <id>` prints it: full IDs throughout, the scriptable
// plumbing form.
func printTask(w io.Writer, col colors, h hydratedTask) {
	printTaskRef(w, col, h, nil)
}

// printTaskRef is printTask with the task references — parents and
// depends_on — passed through ref: the TUI shortens and annotates them
// for display, and gets the full ULID exactly once, dimmed on its own
// line as the copyable canonical form. A nil ref keeps the one-shot
// rendering byte-identical.
func printTaskRef(w io.Writer, col colors, h hydratedTask, ref func(string) string) {
	t := h.Task
	if ref == nil {
		ref = func(id string) string { return id }
		fmt.Fprintf(w, "%s%s%s — %s\n\n", col.bold, t.ID, col.reset, oneLine(t.Title))
	} else {
		fmt.Fprintf(w, "%s%s%s — %s\n\n", col.bold, event.ShortID(t.ID), col.reset, oneLine(t.Title))
		fmt.Fprintf(w, "  %sid          %s%s\n", col.dim, t.ID, col.reset)
	}
	status := views.HumanStatus(t.Status)
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
		fmt.Fprintf(w, "  parents     %s\n", joinRefs(t.Parents, ref))
	}
	if len(t.DependsOn) > 0 {
		fmt.Fprintf(w, "  depends on  %s\n", joinRefs(t.DependsOn, ref))
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

// joinRefs renders a list of task references through ref.
func joinRefs(ids []string, ref func(string) string) string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = ref(id)
	}
	return strings.Join(out, ", ")
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
	printEscalations(os.Stdout, snap)
	return 0
}

// printEscalations emits one row per escalation under a header — the
// same serialization register as printBacklog (T7, 2026-07-31): plain
// aligned columns, no ANSI, full IDs, "-" for empty cells. Open
// questions first, then answered, raise (ULID) order within each; the
// STATE column says which, so grep selects either set.
func printEscalations(w io.Writer, s *snapshot) {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "ID\tSTATE\tBLOCKING\tTASK\tASKED-BY\tRAISED\tANSWERED-BY\tRELAYED-BY\tQUESTION\tANSWER")
	row := func(e escalationJSON) {
		state, blocking := "open", "-"
		if e.Answered {
			state = "answered"
		}
		if e.Blocking {
			blocking = "blocking"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			e.ID, state, blocking, e.Task, e.Actor, stampCompact(e.RaisedAt),
			cell(e.AnsweredBy), cell(e.RelayedBy),
			oneLine(e.Question), cell(oneLine(e.Answer)))
	}
	all := s.allEscalations()
	for _, e := range all {
		if !e.Answered {
			row(e)
		}
	}
	for _, e := range all {
		if e.Answered {
			row(e)
		}
	}
	tw.Flush()
}

// answererLabel attributes an answer, marking the out-of-band path
// where an agent relayed it on the answerer's behalf.
func answererLabel(e escalationJSON) string {
	if e.RelayedBy == "" {
		return e.AnsweredBy
	}
	return fmt.Sprintf("%s, relayed by %s", e.AnsweredBy, e.RelayedBy)
}
