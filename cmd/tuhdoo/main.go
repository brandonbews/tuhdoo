// Command tuhdoo is the human surface (002 T7, Cycle 4): bare
// invocation opens the interactive TUI; the one-shot subcommands are
// scriptable renderings of the same daemon state. Every path talks
// HTTP to the per-repo daemon over its unix socket, auto-spawning it
// when absent (T4 lazy lifecycle).
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	tuhdoo "github.com/brandonbews/tuhdoo"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return runTUI(nil)
	}
	// Every subcommand answers -h/--help with its own usage on stdout,
	// exit 0 (adopter report, 2026-08-26: `tuhdoo init --help` used to
	// be rejected as an unexpected argument) — one behavior everywhere,
	// so muscle memory transfers between commands. First position after
	// the command only: later positions can be data (answer text, --as
	// values). A pure print — no repo, no daemon.
	if len(args) >= 2 && (args[1] == "-h" || args[1] == "--help") && helpFor(os.Stdout, args[0]) {
		return 0
	}
	switch args[0] {
	case "help", "--help", "-h":
		usage(os.Stdout)
		return 0
	case "version":
		fmt.Println("tuhdoo " + version)
		return 0
	case "protocol":
		// A pure print of the embedded doc: no flags, no daemon, no
		// repo — it must work anywhere an agent harness might run.
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "usage: tuhdoo protocol")
			return 1
		}
		os.Stdout.WriteString(tuhdoo.AgentProtocol)
		return 0
	case "daemon":
		return runDaemon()
	case "init":
		return runInit(args[1:])
	case "status":
		return runStatus()
	case "backlog":
		return runBacklog()
	case "task":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: tuhdoo task <id>")
			return 1
		}
		return runTask(args[1])
	case "escalations":
		return runEscalations()
	case "create":
		return runCreate(args[1:])
	case "update":
		return runUpdate(args[1:])
	case "answer":
		return runAnswer(args[1:])
	case "watch":
		fmt.Fprintln(os.Stderr, "tuhdoo: watch is now a mode of the TUI; run: tuhdoo --watch")
		return 1
	case "top":
		fmt.Fprintln(os.Stderr, "tuhdoo: top is gone; the TUI is bare tuhdoo (watch mode: tuhdoo --watch)")
		return 1
	case "mcp":
		return runMCP(args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return runTUI(args)
		}
		fmt.Fprintf(os.Stderr, "tuhdoo: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 1
	}
}

// commandDoc is one subcommand's row in the usage text: the dispatch
// name run() switches on, the invocation column (name plus argument
// shape), and its description lines (newline-separated, unindented).
type commandDoc struct {
	name   string
	invoke string
	desc   string
}

// commandDocs lists every live subcommand, in the order the global
// usage prints them. Both help renderings draw from these rows —
// usage() for the two-column command list, helpFor() for one command's
// -h/--help — so per-command help can never drift from the global text.
var commandDocs = []commandDoc{
	{"init", "init", "set up the data branch in this repository (idempotent)"},
	{"status", "status", "one-screen overview: sync state, counts, active claims"},
	{"backlog", "backlog", `every task, one line each: ID, state, priority, holder,
labels, waiting-on IDs, title. Plain aligned columns,
no styling — grep a state (ready, in-progress, blocked,
on-hold, inbox, done, cancelled) to select its rows`},
	{"task", "task <id>", "one task fully hydrated, with its chronological history"},
	{"escalations", "escalations", `every escalation, one line each, open before answered —
same plain-column form; grep open or blocking`},
	{"create", "create <t>", `add a task: --desc <text|-> --priority <n> --labels a,b
--depends-on <ids> (- reads stdin)
priority is P0-highest: 0 is most urgent, bigger is
later; omitted means unprioritized, served last
--status inbox|held captures without opening the task
(inbox: title-only is fine; agents never get served it)`},
	{"update", "update <id>", `change fields: --title --desc --priority --status
(open|inbox|held|done|cancelled) --labels
--depends-on (lists replace in full)`},
	{"answer", "answer <id>", `answer an open escalation by its ID or its task's ID;
the rest of the line is the answer text`},
	{"mcp", "mcp", `stdio MCP shim for agent harnesses. The principal is
auto-derived: git user.email local part + a session
name minted by the daemon; --as <p> overrides in full
(e.g. --as brandon/impl-1)`},
	{"protocol", "protocol", `print the agent protocol — the instruction text a
harness loads for agents, embedded in this binary.
A pure print: works without a repo or a daemon`},
	{"daemon", "daemon", "run the per-repo daemon in the foreground"},
	{"help", "help", "print this help"},
	{"version", "version", "print the version"},
}

func usage(w io.Writer) {
	fmt.Fprint(w, `tuhdoo `+version+` — a coordination fabric for agent fleets, steered by humans

usage: tuhdoo [-w|--watch] [--as <human>]   the TUI (needs a terminal)
       tuhdoo <command>

Bare tuhdoo opens the interactive TUI: answer escalations, reprioritize,
cancel tasks, drill into any task. It acts as you (--as overrides).
-w/--watch opens the same screen read-only: steering keys are dead for
the life of the pane — the dashboard that sits beside a working agent.

`)
	for _, c := range commandDocs {
		for i, line := range strings.Split(c.desc, "\n") {
			if i == 0 {
				fmt.Fprintf(w, "  %-12s  %s\n", c.invoke, line)
			} else {
				fmt.Fprintf(w, "                %s\n", line)
			}
		}
	}
	fmt.Fprint(w, `
The write commands act as you, like the TUI: --as wins, else the
tuhdoo.principal git config, else the user.email local part. The work
loop (claim, finish_run, release) is deliberately not here: leases
renew only while a live MCP session holds them, and a claim taken by a
one-shot command would just lapse. Agents work through: tuhdoo mcp
`)
}

// helpFor prints one subcommand's usage to w — the same row the global
// usage() renders, reflowed for a single command — and reports whether
// name is a known command.
func helpFor(w io.Writer, name string) bool {
	for _, c := range commandDocs {
		if c.name != name {
			continue
		}
		fmt.Fprintf(w, "usage: tuhdoo %s\n", c.invoke)
		for _, line := range strings.Split(c.desc, "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
		return true
	}
	return false
}
