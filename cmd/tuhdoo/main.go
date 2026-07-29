// Command tuhdoo is the CLI portal (002 T7): the local human's primary
// window into the daemon's replayed state, plus the daemon itself as a
// subcommand. Every read command talks HTTP to the per-repo daemon over
// its unix socket, auto-spawning it when absent (T4 lazy lifecycle).
package main

import (
	"fmt"
	"os"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}
	switch args[0] {
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
	case "watch":
		return runWatch()
	default:
		fmt.Fprintf(os.Stderr, "tuhdoo: unknown command %q\n\n", args[0])
		usage()
		return 1
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `tuhdoo `+version+` — a coordination fabric for agent fleets, steered by humans

usage: tuhdoo <command>

  init          set up the data branch in this repository (idempotent)
  status        one-screen overview: sync state, counts, active claims
  backlog       ready / in-progress / blocked work, from live daemon state
  task <id>     one task fully hydrated, with its chronological history
  escalations   questions raised by agents, awaiting a human answer
  watch         live auto-refreshing read-only dashboard (q quits)
  daemon        run the per-repo daemon in the foreground
  version       print the version
`)
}
