package main

// The stdio MCP shim (002 T4): `tuhdoo mcp --as <agent>` bridges a
// harness's stdio to the daemon's /mcp endpoint, so any harness config
// is just command + args. The shim adds no behavior of its own — it
// mirrors the daemon's tools verbatim and forwards calls — because the
// daemon session is what T5's session-bound leases hang off: as long as
// the shim process is alive and connected, the agent's leases renew.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/brandonbews/tuhdoo/internal/daemon"
)

// actorTransport injects the X-Tuhdoo-Actor header on every request to
// the daemon — including the first POST of the session, where the
// daemon binds the principal.
type actorTransport struct {
	base  http.RoundTripper
	actor string
}

func (t actorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("X-Tuhdoo-Actor", t.actor)
	return t.base.RoundTrip(req)
}

func runMCP(args []string) int {
	principal := ""
	switch {
	case len(args) == 2 && args[0] == "--as":
		principal = args[1]
	case len(args) == 1 && strings.HasPrefix(args[0], "--as="):
		principal = strings.TrimPrefix(args[0], "--as=")
	default:
		fmt.Fprintln(os.Stderr, "usage: tuhdoo mcp --as <principal>   (e.g. --as brandon/impl-1)")
		return 1
	}
	if err := daemon.ValidateActor(principal); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo mcp:", err)
		return 1
	}

	r, err := openRepo()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo mcp:", err)
		return 1
	}
	c, err := ensureDaemon(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo mcp:", err)
		return 1
	}

	if err := bridgeMCP(c.socket, principal); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo mcp:", err)
		return 1
	}
	return 0
}

// bridgeMCP connects to the daemon's MCP endpoint as principal and
// serves a mirror of its tools over stdio until the harness hangs up.
// A non-nil error means the bridge ended abnormally (daemon side died).
func bridgeMCP(socket, principal string) error {
	ctx := context.Background()

	hc := &http.Client{Transport: actorTransport{base: unixTransport(socket), actor: principal}}
	client := mcp.NewClient(&mcp.Implementation{Name: "tuhdoo-shim", Version: version}, nil)
	sess, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   "http://tuhdoo/mcp",
		HTTPClient: hc,
	}, nil)
	if err != nil {
		return fmt.Errorf("connect to daemon MCP endpoint: %w", err)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "tuhdoo", Version: version}, &mcp.ServerOptions{
		Instructions: sess.InitializeResult().Instructions,
	})
	// Mirror the daemon's tools name-for-name, schema-for-schema. The
	// handler forwards raw arguments and returns the daemon's result
	// unchanged (content, structured content, isError).
	for tool, err := range sess.Tools(ctx, nil) {
		if err != nil {
			sess.Close()
			return fmt.Errorf("list daemon tools: %w", err)
		}
		name := tool.Name
		srv.AddTool(&mcp.Tool{
			Name:         name,
			Title:        tool.Title,
			Description:  tool.Description,
			InputSchema:  tool.InputSchema,
			OutputSchema: tool.OutputSchema,
			Annotations:  tool.Annotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			params := &mcp.CallToolParams{Name: name}
			if len(req.Params.Arguments) > 0 {
				params.Arguments = req.Params.Arguments
			}
			return sess.CallTool(ctx, params)
		})
	}

	// If the daemon session dies while the harness is still attached,
	// exit loudly: silently serving a dead session would let the
	// harness think its leases are still renewing.
	var mu sync.Mutex
	closing := false
	go func() {
		werr := sess.Wait()
		mu.Lock()
		c := closing
		mu.Unlock()
		if !c {
			fmt.Fprintln(os.Stderr, "tuhdoo mcp: daemon session ended:", werr)
			os.Exit(1)
		}
	}()

	// Serve stdio until the harness closes it, then release the daemon
	// session cleanly so the daemon stops renewing this agent's leases
	// right away instead of waiting for keepalive to notice.
	runErr := srv.Run(ctx, &mcp.StdioTransport{})
	mu.Lock()
	closing = true
	mu.Unlock()
	sess.Close()
	if runErr != nil {
		return fmt.Errorf("stdio session: %w", runErr)
	}
	return nil
}
