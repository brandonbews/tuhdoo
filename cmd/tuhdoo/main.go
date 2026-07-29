package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/brandonbews/tuhdoo/internal/daemon"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

// main stays minimal on purpose: B10 restructures the CLI properly.
func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println("tuhdoo " + version)
			return
		case "daemon":
			os.Exit(runDaemon())
		}
	}
	fmt.Fprintln(os.Stderr, "tuhdoo "+version+" — commands: daemon, version; see docs/plan/backlog.md")
	os.Exit(1)
}

// runDaemon runs the per-repo daemon in the foreground against the
// current directory's repository.
func runDaemon() int {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo daemon:", err)
		return 1
	}
	d, err := daemon.New(root, daemon.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo daemon:", err)
		return 1
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-sig
		d.Shutdown("received signal " + s.String())
	}()

	if err := d.Run(); err != nil {
		return 1
	}
	return 0
}
