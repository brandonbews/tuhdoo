package main

// The write commands: the human portal's paved path when no MCP session
// exists — shell humans who prefer commands to the TUI, cron jobs,
// scripts, and agents wrapping up after a daemon restart killed their
// session. Each talks the same daemon HTTP API the TUI steer mode uses:
// same validation, same actor derivation (--as wins, else the
// tuhdoo.principal git config override, else the user.email local part;
// always a human root principal — agent halves are minted by MCP
// sessions only).
//
// The claim lifecycle (claim, finish_run, release) is deliberately
// absent here: leases are session-bound (T8) — the daemon renews them
// while an MCP session is live, and a claim taken by a one-shot command
// would have nothing renewing it, lapsing interrupted within the TTL.
// The MCP shim is the sanctioned work loop; the help text says so.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// writeActor resolves the acting principal for a write command exactly
// as TUI steer mode does.
func writeActor(as string) (string, int) {
	actor, err := topActor(as)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo:", err)
		return "", 1
	}
	return actor, 0
}

// readDesc returns the description flag's value, reading stdin when the
// value is "-" (task descriptions are prompts and often multi-line;
// heredocs beat quoting).
func readDesc(v string) (string, error) {
	if v != "-" {
		return v, nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read description from stdin: %w", err)
	}
	return string(b), nil
}

// splitList turns a comma-separated flag value into a list, dropping
// empty items; "" means the empty list (a full replacement that clears).
func splitList(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// resolveRefs maps each edge reference — full ID, short form, or
// fragment — through resolveTaskID (T7: short IDs are the human input
// contract everywhere).
func resolveRefs(refs []string, tasks []stateTask) ([]string, error) {
	out := make([]string, len(refs))
	for i, r := range refs {
		full, err := resolveTaskID(r, tasks)
		if err != nil {
			return nil, err
		}
		out[i] = full
	}
	return out, nil
}

// ---- create ----

func runCreate(args []string) int {
	const use = `usage: tuhdoo create <title> [--desc <text>|--desc -] [--priority <n>]
                     [--status open|inbox|held] [--labels a,b]
                     [--parents <ids>] [--depends-on <ids>] [--as <human>]
(--status inbox is capture: title-only is fine, agents are never served it;
 --status held parks a triaged task; both promote later via update --status open)`
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(os.Stderr, use)
		return 1
	}
	title := args[0]
	fs := flag.NewFlagSet("tuhdoo create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var desc, status, labels, parents, dependsOn, as string
	var priority int
	fs.StringVar(&desc, "desc", "", "")
	fs.StringVar(&status, "status", "", "")
	fs.IntVar(&priority, "priority", 0, "")
	fs.StringVar(&labels, "labels", "", "")
	fs.StringVar(&parents, "parents", "", "")
	fs.StringVar(&dependsOn, "depends-on", "", "")
	fs.StringVar(&as, "as", "", "")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, use)
		return 1
	}
	actor, code := writeActor(as)
	if code != 0 {
		return code
	}
	description, err := readDesc(desc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo create:", err)
		return 1
	}

	_, c, code := connect()
	if code != 0 {
		return code
	}
	item := map[string]any{"title": title, "priority": priority}
	if description != "" {
		item["description"] = description
	}
	if status != "" {
		item["status"] = status // daemon validates: open, inbox, or held
	}
	if labels != "" {
		item["labels"] = splitList(labels)
	}
	if parents != "" || dependsOn != "" {
		st, err := fetchState(c)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tuhdoo create:", err)
			return 1
		}
		for flagName, v := range map[string]string{"parents": parents, "depends_on": dependsOn} {
			if v == "" {
				continue
			}
			refs, err := resolveRefs(splitList(v), st.Tasks)
			if err != nil {
				fmt.Fprintln(os.Stderr, "tuhdoo create:", err)
				return 1
			}
			item[flagName] = refs
		}
	}
	var resp struct {
		IDs []string `json:"ids"`
	}
	if err := c.writeResp("POST", "/v0/tasks", actor, []map[string]any{item}, &resp); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo create:", err)
		return 1
	}
	if len(resp.IDs) != 1 {
		fmt.Fprintf(os.Stderr, "tuhdoo create: daemon returned %d ids\n", len(resp.IDs))
		return 1
	}
	fmt.Printf("created %s — %s\n", resp.IDs[0], oneLine(title))
	return 0
}

// ---- update ----

func runUpdate(args []string) int {
	const use = `usage: tuhdoo update <id> [--title <t>] [--desc <text>|--desc -]
                     [--priority <n>] [--status open|inbox|held|done|archived]
                     [--labels a,b] [--parents <ids>] [--depends-on <ids>]
                     [--as <human>]
(list flags are full replacements; an empty value clears the list;
 --status open promotes/resumes, --status held pauses — promotion from
 inbox deserves a real description in the same breath)`
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(os.Stderr, use)
		return 1
	}
	id := args[0]
	fs := flag.NewFlagSet("tuhdoo update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var title, desc, status, labels, parents, dependsOn, as string
	var priority int
	fs.StringVar(&title, "title", "", "")
	fs.StringVar(&desc, "desc", "", "")
	fs.StringVar(&status, "status", "", "")
	fs.IntVar(&priority, "priority", 0, "")
	fs.StringVar(&labels, "labels", "", "")
	fs.StringVar(&parents, "parents", "", "")
	fs.StringVar(&dependsOn, "depends-on", "", "")
	fs.StringVar(&as, "as", "", "")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, use)
		return 1
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if len(set) == 0 || (len(set) == 1 && set["as"]) {
		fmt.Fprintln(os.Stderr, use)
		return 1
	}
	actor, code := writeActor(as)
	if code != 0 {
		return code
	}

	_, c, code := connect()
	if code != 0 {
		return code
	}
	st, err := fetchState(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo update:", err)
		return 1
	}
	full, err := resolveTaskID(id, st.Tasks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo update:", err)
		return 1
	}
	body := map[string]any{}
	if set["title"] {
		body["title"] = title
	}
	if set["desc"] {
		description, err := readDesc(desc)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tuhdoo update:", err)
			return 1
		}
		body["description"] = description
	}
	if set["priority"] {
		body["priority"] = priority
	}
	if set["status"] {
		// "archive" is the human verb; the API and ledger vocabulary
		// stays "cancelled" (T7, 2026-07-31). The plumbing word still
		// passes through untouched for scripts that speak it.
		if status == "archived" {
			status = "cancelled"
		}
		body["status"] = status
	}
	if set["labels"] {
		body["labels"] = splitList(labels)
	}
	for flagName, key := range map[string]string{"parents": "parents", "depends-on": "depends_on"} {
		if !set[flagName] {
			continue
		}
		v := parents
		if flagName == "depends-on" {
			v = dependsOn
		}
		refs, err := resolveRefs(splitList(v), st.Tasks)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tuhdoo update:", err)
			return 1
		}
		body[key] = refs
	}
	if err := c.write("PATCH", "/v0/tasks/"+full, actor, body); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo update:", err)
		return 1
	}
	fmt.Printf("updated %s\n", full)
	return 0
}

// ---- answer ----

func runAnswer(args []string) int {
	const use = `usage: tuhdoo answer <id> [--as <human>] <answer text...>
(<id> is the escalation's ID or its task's ID — short forms and fragments work)`
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(os.Stderr, use)
		return 1
	}
	id := args[0]
	fs := flag.NewFlagSet("tuhdoo answer", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var as string
	fs.StringVar(&as, "as", "", "")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, use)
		return 1
	}
	answer := strings.Join(fs.Args(), " ")
	actor, code := writeActor(as)
	if code != 0 {
		return code
	}

	_, c, code := connect()
	if code != 0 {
		return code
	}
	st, err := fetchState(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo answer:", err)
		return 1
	}
	esc, err := resolveEscalation(id, st)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo answer:", err)
		return 1
	}
	if err := c.write("POST", "/v0/escalations/answer", actor, map[string]any{
		"escalation": esc.ID, "answer": answer,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo answer:", err)
		return 1
	}
	fmt.Printf("answered %s — %s\n", esc.ID, oneLine(esc.Question))
	return 0
}

// resolveEscalation maps human input to exactly one open escalation.
// The input may be the escalation's own ID (or a fragment of it) or the
// ID of the task it blocks — `tuhdoo escalations` prints task IDs, and
// humans think in tasks. Ambiguity — several open questions match — is
// an error listing the candidates, never a guess.
func resolveEscalation(frag string, st stateResp) (escalationJSON, error) {
	taskID := ""
	if full, err := resolveTaskID(frag, st.Tasks); err == nil {
		taskID = full
	}
	var cands []escalationJSON
	for _, e := range st.OpenEscalations {
		if strings.EqualFold(e.ID, frag) {
			return e, nil // exact escalation ID wins outright
		}
		if idMatches(e.ID, frag) || e.Task == taskID {
			cands = append(cands, e)
		}
	}
	switch len(cands) {
	case 1:
		return cands[0], nil
	case 0:
		return escalationJSON{}, fmt.Errorf("no open escalation matches %q — try: tuhdoo escalations", frag)
	}
	lines := make([]string, len(cands))
	for i, e := range cands {
		lines[i] = fmt.Sprintf("  %s  %s  (task %s)", e.ID, oneLine(e.Question), shortID(e.Task))
	}
	return escalationJSON{}, fmt.Errorf("%q is ambiguous — %d open escalations match:\n%s",
		frag, len(cands), strings.Join(lines, "\n"))
}
