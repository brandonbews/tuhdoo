package main

// The stdio MCP shim (002 T4): `tuhdoo mcp` bridges a harness's stdio
// to the daemon's /mcp endpoint, so any harness config is just command
// + args. The shim adds no behavior of its own — it mirrors the
// daemon's tools verbatim and forwards calls — because the daemon
// session is what T5's session-bound leases hang off: as long as the
// shim process is alive and connected, the agent's leases renew.
//
// Identity (D7): with no flags, the human half derives from git
// identity (the local part of user.email) and the daemon mints the
// agent half at session bind from the harness's initialize
// clientInfo.name. That name only arrives over stdio, so the daemon
// session is opened inside the initialize middleware below, not up
// front. --as <principal> overrides the whole principal.

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

// actorTransport injects the identity headers on every request to the
// daemon — including the first POST of the session, where the daemon
// binds the principal. client, when set, is the harness's
// clientInfo.name; its presence asks the daemon to mint the agent half.
type actorTransport struct {
	base   http.RoundTripper
	actor  string
	client string
}

func (t actorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("X-Tuhdoo-Actor", t.actor)
	if t.client != "" {
		req.Header.Set("X-Tuhdoo-Agent-Name", t.client)
	}
	return t.base.RoundTrip(req)
}

func runMCP(args []string) int {
	override := ""
	switch {
	case len(args) == 0:
		// The zero-config default: auto-derive the principal.
	case len(args) == 2 && args[0] == "--as":
		override = args[1]
	case len(args) == 1 && strings.HasPrefix(args[0], "--as="):
		override = strings.TrimPrefix(args[0], "--as=")
	default:
		fmt.Fprintln(os.Stderr, "usage: tuhdoo mcp [--as <principal>]   (default: principal auto-derived from git identity)")
		return 1
	}
	if override != "" {
		if err := daemon.ValidateActor(override); err != nil {
			fmt.Fprintln(os.Stderr, "tuhdoo mcp:", err)
			return 1
		}
	}

	r, err := openRepo()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo mcp:", err)
		return 1
	}
	human := ""
	if override == "" {
		human, err = gitEmailLocalPart(r.root)
		if err == nil {
			err = daemon.ValidateActor(human)
		}
		if err != nil {
			// Fail loudly at connect (T5 acceptance): a session with no
			// honest principal must not come up at all.
			fmt.Fprintf(os.Stderr, "tuhdoo mcp: cannot derive a principal from git identity: %v\n"+
				"set git user.email, or pass one explicitly: tuhdoo mcp --as <human>/<agent>\n", err)
			return 1
		}
	}
	c, err := ensureDaemon(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo mcp:", err)
		return 1
	}

	if err := bridgeMCP(c.socket, override, human); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo mcp:", err)
		return 1
	}
	return 0
}

// bridgeMCP serves the harness over stdio and binds the daemon session
// on the harness's first request — the request that carries the
// clientInfo.name the daemon mints the agent principal from: initialize
// params on the pre-2026 protocol, per-request _meta (surfaced as the
// session's InitializeParams) on the 2026-07-28 protocol, whose clients
// send no initialize at all. override, when non-empty, is the full
// --as principal; otherwise human is the git-derived root and the
// daemon completes it.
func bridgeMCP(socket, override, human string) error {
	srv := mcp.NewServer(&mcp.Implementation{Name: "tuhdoo-shim", Version: version}, nil)

	var mu sync.Mutex
	var sess *mcp.ClientSession
	closing := false

	// bind connects to the daemon and mirrors its tools, once; callers
	// after the first get the existing session. Serialized by mu — the
	// SDK dispatches calls asynchronously, so two first-requests can
	// race here.
	bind := func(ctx context.Context, req mcp.Request) (*mcp.ClientSession, error) {
		mu.Lock()
		defer mu.Unlock()
		if sess != nil {
			return sess, nil
		}

		// The harness names itself differently per protocol vintage:
		// initialize params (pre-2026), per-request _meta on the
		// opening server/discover (2026-07-28), or — for any other
		// first method — the session state the SDK populates from
		// _meta before dispatch.
		clientName := ""
		switch p := req.GetParams().(type) {
		case *mcp.InitializeParams:
			if p != nil && p.ClientInfo != nil {
				clientName = p.ClientInfo.Name
			}
		case *mcp.DiscoverParams:
			if p != nil {
				clientName = metaClientName(p.GetMeta())
			}
		}
		if clientName == "" {
			if ss, ok := req.GetSession().(*mcp.ServerSession); ok {
				if ip := ss.InitializeParams(); ip != nil && ip.ClientInfo != nil {
					clientName = ip.ClientInfo.Name
				}
			}
		}
		ds, err := connectDaemon(ctx, socket, override, human, clientName)
		if err != nil {
			return nil, err
		}
		// Mirror before the first response goes out: the harness only
		// asks for tools after its opening request round-trips.
		if err := mirrorTools(ctx, srv, ds); err != nil {
			ds.Close()
			return nil, err
		}

		// If the daemon session dies while the harness is still
		// attached, exit loudly: silently serving a dead session
		// would let the harness think its leases are still renewing.
		go func() {
			werr := ds.Wait()
			mu.Lock()
			c := closing
			mu.Unlock()
			if !c {
				fmt.Fprintln(os.Stderr, "tuhdoo mcp: daemon session ended:", werr)
				os.Exit(1)
			}
		}()
		sess = ds
		return ds, nil
	}

	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			ds, err := bind(ctx, req)
			if err != nil {
				// Loud on stderr and to the harness: a shim with no
				// daemon session must not look connected.
				fmt.Fprintln(os.Stderr, "tuhdoo mcp:", err)
				return nil, err
			}
			res, err := next(ctx, method, req)
			if err == nil {
				// The daemon's instructions ride whichever handshake
				// result this protocol version uses.
				instr := ds.InitializeResult().Instructions
				switch r := res.(type) {
				case *mcp.InitializeResult:
					r.Instructions = instr
				case *mcp.DiscoverResult:
					r.Instructions = instr
				}
			}
			return res, err
		}
	})

	// Serve stdio until the harness closes it, then release the daemon
	// session cleanly so the daemon stops renewing this agent's leases
	// right away instead of waiting for keepalive to notice.
	runErr := srv.Run(context.Background(), &mcp.StdioTransport{})
	mu.Lock()
	closing = true
	ds := sess
	mu.Unlock()
	if ds != nil {
		ds.Close()
	}
	if runErr != nil {
		return fmt.Errorf("stdio session: %w", runErr)
	}
	return nil
}

// metaClientName extracts clientInfo.name from a _meta map. The value
// arrives typed when built in-process and as a generic JSON map after
// wire transit; both occur.
func metaClientName(meta map[string]any) string {
	switch ci := meta[mcp.MetaKeyClientInfo].(type) {
	case *mcp.Implementation:
		if ci != nil {
			return ci.Name
		}
	case map[string]any:
		if n, ok := ci["name"].(string); ok {
			return n
		}
	}
	return ""
}

// connectDaemon opens the shim's session against the daemon's MCP
// endpoint with the identity headers bound.
func connectDaemon(ctx context.Context, socket, override, human, clientName string) (*mcp.ClientSession, error) {
	tr := actorTransport{base: unixTransport(socket), actor: override}
	if override == "" {
		tr.actor = human
		tr.client = clientName
		if tr.client == "" {
			// A harness that named no client still gets a minted,
			// session-unique principal (the daemon falls back the same
			// way; this just keeps the header honest).
			tr.client = "agent"
		}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "tuhdoo-shim", Version: version}, nil)
	sess, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   "http://tuhdoo/mcp",
		HTTPClient: &http.Client{Transport: tr},
		// The SDK default of 5 reconnect attempts (~20s of backoff) is
		// sized for flaky networks. This is a unix socket: a refused
		// connection means the daemon is gone, and there is no
		// supervisor to bring it back mid-retry — so retry briefly,
		// then fail loudly (see the ds.Wait goroutine in bridgeMCP).
		MaxRetries: 2,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon MCP endpoint: %w", err)
	}
	return sess, nil
}

// mirrorTools registers the daemon's tools on the shim's server,
// name-for-name, schema-for-schema. Handlers forward raw arguments and
// return the daemon's result unchanged (content, structured content,
// isError).
func mirrorTools(ctx context.Context, srv *mcp.Server, sess *mcp.ClientSession) error {
	for tool, err := range sess.Tools(ctx, nil) {
		if err != nil {
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
	return nil
}
