// Package agenttypes provides shared types for cloud-agent.
package agenttypes

import "net/rpc"

const maxErrorCount = 5

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

// AgentList is a map of agent IDs to Agent structs.
type AgentList map[string]Agent

// Add adds a new agent to the list with active status.
func (a AgentList) Add(id string, agent Agent) {
	agent.errorCounter = 0
	agent.active = true
	a[id] = agent
}

// Del removes an agent from the list (not implemented).
func (a AgentList) Del(_ string) {
	// todo: delete id from AgentList map
}

// IsErr reports whether an agent has exceeded the specified error threshold.
func (a AgentList) IsErr(id string, num int) bool {
	agent := a[id]
	return agent.errorCounter >= num
}

// SetErr increments the error counter for a specific agent.
func (a AgentList) SetErr(id string) {
	agent := a[id]
	agent.SetErr()
	a[id] = agent
}

// ClearErr resets the error counter for a specific agent and returns the updated agent.
func (a AgentList) ClearErr(id string) Agent {
	agent := a[id]
	agent.ClearErr()
	return agent
}

// Active reports whether a specific agent is currently active.
func (a AgentList) Active(id string) bool {
	agent := a[id]
	return agent.Active()
}
