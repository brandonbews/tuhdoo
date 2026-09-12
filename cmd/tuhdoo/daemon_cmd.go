package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brandonbews/tuhdoo/internal/daemon"
)

// leaseTTLEnv is a harness knob, not a user setting: a Go duration
// (e.g. "4m") that sets the claim lease TTL of the daemon this process
// runs. Unset or empty leaves daemon.DefaultLeaseTTL (T8: 15 min)
// exactly as before — nothing in production sets it. It exists so
// harness/collision can watch a lease lapse naturally inside a bounded
// run; a value that does not parse is a startup failure, never a
// silent fallback to the default.
const leaseTTLEnv = "TUHDOO_LEASE_TTL"

// leaseTTLFromEnv parses the leaseTTLEnv value: "" is the default
// (zero, which daemon.New reads as DefaultLeaseTTL); anything else must
// be a positive Go duration.
func leaseTTLFromEnv(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	ttl, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s=%q: %w", leaseTTLEnv, value, err)
	}
	if ttl <= 0 {
		return 0, fmt.Errorf("%s=%q: must be a positive duration", leaseTTLEnv, value)
	}
	return ttl, nil
}

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
	ttl, err := leaseTTLFromEnv(os.Getenv(leaseTTLEnv))
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo daemon:", err)
		return 1
	}
	d, err := daemon.New(root, daemon.Options{Version: version, LeaseTTL: ttl})
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
