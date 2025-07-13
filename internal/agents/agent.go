// Package agents provides shared types for cloud-agent.
package agents

import (
	"net/rpc"
)

// Agent represents a node in the cluster with its connection and status.
type Agent struct {
	Address      string      // IP:Port
	Client       *rpc.Client // for JSON-RPC connection
	Master       bool        // for future
	active       bool
	errorCounter int
}

// SetErr increments the error counter and marks the agent as inactive if the threshold is reached.
func (a *Agent) SetErr() {
	a.errorCounter++
	if a.errorCounter >= maxErrorCount {
		a.active = false
	}
}

// ClearErr resets the error counter and marks the agent as active.
func (a *Agent) ClearErr() {
	a.errorCounter = 0
	a.active = true
}

// Active reports whether the agent is currently active.
func (a *Agent) Active() bool {
	return a.active
}

// ErrorCount returns the current error counter value.
func (a *Agent) ErrorCount() int {
	return a.errorCounter
}
