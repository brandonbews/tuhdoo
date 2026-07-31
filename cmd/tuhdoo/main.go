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
	switch args[0] {
	case "help", "--help", "-h":
		usage(os.Stdout)
		return 0
	case "version":
		fmt.Println("tuhdoo " + version)
		return 0
	case "daemon":
		return runDaemon()
	case "init":
		return runInit()
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

func usage(w io.Writer) {
	fmt.Fprint(w, `tuhdoo `+version+` — a coordination fabric for agent fleets, steered by humans

usage: tuhdoo [-w|--watch] [--as <human>]   the TUI (needs a terminal)
       tuhdoo <command>

Bare tuhdoo opens the interactive TUI: answer escalations, reprioritize,
cancel tasks, drill into any task. It acts as you (--as overrides).
-w/--watch opens the same screen read-only: steering keys are dead for
the life of the pane — the dashboard that sits beside a working agent.

  init          set up the data branch in this repository (idempotent)
  status        one-screen overview: sync state, counts, active claims
  backlog       ready / in-progress / blocked work, from live daemon state
  task <id>     one task fully hydrated, with its chronological history
  escalations   questions raised by agents, awaiting a human answer
  create <t>    add a task: --desc <text|-> --priority <n> --labels a,b
                --parents <ids> --depends-on <ids> (- reads stdin)
  update <id>   change fields: --title --desc --priority --status
                --labels --parents --depends-on (lists replace in full)
  answer <id>   answer an open escalation by its ID or its task's ID;
                the rest of the line is the answer text
  mcp           stdio MCP shim for agent harnesses. The principal is
                auto-derived: git user.email local part + a session
                name minted by the daemon; --as <p> overrides in full
                (e.g. --as brandon/impl-1)
  daemon        run the per-repo daemon in the foreground
  help          print this help
  version       print the version

The write commands act as you, like the TUI: --as wins, else the
tuhdoo.principal git config, else the user.email local part. The work
loop (claim, finish_run, release) is deliberately not here: leases
renew only while a live MCP session holds them, and a claim taken by a
one-shot command would just lapse. Agents work through: tuhdoo mcp
`)
}
