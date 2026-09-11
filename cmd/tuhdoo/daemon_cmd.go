package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/brandonbews/tuhdoo/internal/daemon"
)

// exitAlreadyRunning is `tuhdoo daemon`'s exit status when another
// daemon already holds the repository's lock. Two CLIs racing to spawn
// each start a daemon; the flock loser exits with this code so the CLI
// that spawned it keeps waiting for the winner's socket (awaitDaemon)
// instead of reporting a death. Any other non-zero exit is a death.
const exitAlreadyRunning = 3

// runDaemon runs the per-repo daemon in the foreground against the
// surrounding repository (resolved to the worktree root, so it works
// from a subdirectory too).
func runDaemon() int {
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo daemon:", err)
		return 1
	}
	d, err := daemon.New(root, daemon.Options{Version: version})
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo daemon:", err)
		if errors.Is(err, daemon.ErrAlreadyRunning) {
			return exitAlreadyRunning
		}
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
