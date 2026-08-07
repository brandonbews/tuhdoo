// Package tuhdoo exists to embed repo-root assets that ship inside the
// binary. //go:embed paths cannot climb out of a package directory
// ("..") — so the doc is embedded here, at the root, and cmd/tuhdoo
// imports it.
package tuhdoo

import _ "embed"

// AgentProtocol is docs/agent-protocol.md, byte-for-byte — the
// instruction text a harness loads for agents (printed by `tuhdoo
// protocol`). One canonical text, versioned with the binary the agents
// actually talk to; a test in cmd/tuhdoo pins it to the file.
//
//go:embed docs/agent-protocol.md
var AgentProtocol string
