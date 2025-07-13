// Package agents provides shared types for cloud-agent.
package agents

import (
	"cloud-agent/internal/config"
	"log"
)

const maxErrorCount = 5

// AgentList is a map of agent IDs to Agent structs.
type AgentList struct {
	Peer     map[string]Agent
	DbActive bool
	dbPath   string
	db       *PeersDB
}

// NewAgentList creates an AgentList from the given config.
func NewAgentList(cfg *config.Config) *AgentList {
	var agentList AgentList
	agentList.Peer = make(map[string]Agent) // Ensure map is initialized

	// add self agent
	agentList.Add(cfg.SelfID, Agent{Address: cfg.TCPAddress})

	for _, peer := range cfg.Peers {
		agentList.Add(peer.Name, Agent{Address: peer.Addr, Master: peer.Master})
		agentList.Peer[peer.Name] = agentList.ClearErr(peer.Name)
	}

	var err error
	if cfg.PeersDBPath != "" {
		agentList.dbPath = cfg.PeersDBPath
		agentList.db, err = InitPeersDB(cfg.PeersDBPath)
		if err != nil {
			log.Printf("[AGENTS] Failed to open peers DB: %v", err)
		} else {
			agentList.DbActive = true
		}
	}

	if agentList.DbActive {
		// read peers from DB to agentList.Peer
		peers, err := agentList.db.LoadPeersFromDB()
		if err != nil {
			log.Printf("[AGENTS] Failed to read peers DB: %v", err)
		} else {
			for _, peer := range peers {
				agentList.Add(peer.Name, Agent{Address: peer.Addr, Master: peer.Master})
				agentList.Peer[peer.Name] = agentList.ClearErr(peer.Name)
			}
		}
	}

	return &agentList
}

// Add adds a new agent to the list with active status.
func (a AgentList) Add(id string, agent Agent) {
	agent.errorCounter = 0
	agent.active = true
	a.Peer[id] = agent

	if a.DbActive {
		err := a.db.UpsertPeer(
			config.PeerInfo{
				Name:   id,
				Addr:   agent.Address,
				Master: agent.Master,
			},
		)
		if err != nil {
			log.Printf("[AGENTS] Failed to upsert peer in DB: %v", err)
		}
	}
}

// Del removes an agent from the list and from the DB if active.
func (a AgentList) Del(id string) {
	delete(a.Peer, id)

	// Delete from DB
	if a.DbActive {
		err := a.db.RemovePeer(id)
		if err != nil {
			log.Printf("[AGENTS] Failed to remove peer from DB: %v", err)
		}
	}
}

// IsErr reports whether an agent has exceeded the specified error threshold.
func (a AgentList) IsErr(id string, num int) bool {
	agent := a.Peer[id]
	return agent.errorCounter >= num
}

// SetErr increments the error counter for a specific agent.
func (a AgentList) SetErr(id string) {
	agent := a.Peer[id]
	agent.SetErr()
	a.Peer[id] = agent
}

// ClearErr resets the error counter for a specific agent and returns the updated agent.
func (a AgentList) ClearErr(id string) Agent {
	agent := a.Peer[id]
	agent.ClearErr()
	return agent
}

// Active reports whether a specific agent is currently active.
func (a AgentList) Active(id string) bool {
	agent := a.Peer[id]
	return agent.Active()
}

// Masters returns a slice of agent names that are marked as master.
func (a AgentList) Masters() []string {
	masters := make([]string, 0)
	for name, agent := range a.Peer {
		if agent.Master {
			masters = append(masters, name)
		}
	}
	return masters
}
