// Package agents provides shared types for cloud-agent.
package agents

import (
	"cloud-agent/internal/config"
)

const maxErrorCount = 5

// AgentList is a map of agent IDs to Agent structs.
type AgentList map[string]Agent

// NewAgentList creates an AgentList from the given self info and peer list.
func NewAgentList(selfID, selfAddr string, peers []config.PeerInfo) AgentList {
	agentList := make(AgentList, len(peers)+1)

	// add self agent
	agentList.Add(selfID, Agent{Address: selfAddr})

	for _, peer := range peers {
		agentList.Add(peer.Name, Agent{Address: peer.Addr, Master: peer.Master})
		agentList[peer.Name] = agentList.ClearErr(peer.Name)
	}

	return agentList
}

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

// Masters returns a slice of agent names that are marked as master.
func (a AgentList) Masters() []string {
	masters := make([]string, 0)
	for name, agent := range a {
		if agent.Master {
			masters = append(masters, name)
		}
	}
	return masters
}
