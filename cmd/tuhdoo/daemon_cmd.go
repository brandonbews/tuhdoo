package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/brandonbews/tuhdoo/internal/daemon"
)

// runDaemon runs the per-repo daemon in the foreground against the
// surrounding repository (resolved to the worktree root, so it works
// from a subdirectory too).
func runDaemon() int {
	root, err := repoRoot()
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
