// Package agents provides shared types for cloud-agent.
package agents

import (
	"net/rpc"
)

// Agent represents a node in the cluster with its connection and status.
type Agent struct {
	Address      string      // IP:Port
	Client       *rpc.Client // for JSON-RPC connection
	Master       bool
	Token        string // Shared secret for authentication
	KeyPublic    string // Agent's public key (base64-encoded)
	active       bool
	errorCounter int
}
