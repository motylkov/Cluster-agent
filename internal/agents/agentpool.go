// Package agents provides shared types for cloud-agent.
package agents

import (
	"cloud-agent/internal/config"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"sync"
)

const maxErrorCount = 5

// AgentPool is a map of agent IDs to Agent structs.
type AgentPool struct {
	cfg       *config.Config
	Peer      map[string]Agent
	DbActive  bool
	db        *PeersDB
	keyPublic *[32]byte
	//	keyPublic64 string
	selfToken string
	mu        sync.RWMutex
}

func generateRandomToken(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "fallbacktoken"
	}
	return hex.EncodeToString(b)
}

// NewAgentPool creates an AgentPool from the given config.
func NewAgentPool(conf *config.Config, kPub *[32]byte) *AgentPool {
	return &AgentPool{
		cfg:       conf,
		Peer:      make(map[string]Agent),
		DbActive:  false,
		keyPublic: kPub,
		//		keyPublic64: base64.StdEncoding.EncodeToString(kPub[:]),
		selfToken: "",
	}
}

func (pool *AgentPool) Init() {

	// // add self agent
	// pool.Add(pool.cfg.SelfID, Agent{
	// 	Address:   pool.cfg.TCPAddress,
	// 	Token:     pool.selfToken,
	// 	Master:    false,
	// 	PublicKey: pool.keyPublic64,
	// })
	//	log.Printf("[DEBUG] self agentPool: %v", agentPool)

	// read peers from config file (master only)
	for _, peer := range pool.cfg.Peers {
		pool.add(peer.Name, Agent{
			Address:   peer.Addr,
			Master:    peer.Master,
			Token:     "",
			KeyPublic: "",
		})
		log.Printf("[INIT] read agent from config: %s %s %v", peer.Name, peer.Addr, peer.Master)
	}

	var err error
	if pool.cfg.PeersDBPath != "" {
		pool.db, err = InitPeersDB(pool.cfg.PeersDBPath)
		if err != nil {
			log.Printf("[INIT] Failed to open peers DB: %v", err)
		} else {
			log.Println("[INIT] Open peers DB")
			pool.mu.Lock()
			pool.DbActive = true
			pool.mu.Unlock()
		}
	}

	if pool.DbActive {
		// read peers from DB to pool.Peer
		peers, err := pool.db.LoadPeersFromDB()
		if err != nil {
			log.Printf("[AGENTS] Failed to read peers DB: %v", err)
		} else {
			for _, peer := range peers {
				if pool.cfg.SelfID != peer.Name {
					existingAgent, exists := pool.Peer[peer.Name]
					if exists {
						pool.Delete(peer.Name)
						// delete(pool.Peer, peer.Name)
						peer.Master = existingAgent.Master
						peer.Addr = existingAgent.Address

					}
					pool.add(peer.Name, Agent{
						Address:   peer.Addr,
						Token:     peer.Token,
						Master:    peer.Master,
						KeyPublic: peer.KeyPublic,
					})
				} else {
					log.Println("[DEBUG] self peer skip read from DB")
				}
			}
		}
	}

	// Token = generateRandomToken(32)
}

// Upsert adds new or update exist agent to the pool and to the DB. Using for Register.
func (pool *AgentPool) Upsert(id string, agent Agent) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	existingAgent, exists := pool.Peer[id]

	if exists {
		log.Printf("[DEBUG] Exist agent updating %s", id)
		log.Printf("[DEBUG] Old %s: %v", id, existingAgent)
		agent.errorCounter = 0
		agent.active = true
		log.Printf("[DEBUG] New %s: %v", id, agent)
		// Update exist data
		log.Printf("[DEBUG] %s Token update from '%v' to '%v'", id, existingAgent.Token, agent.Token)
		log.Printf("[DEBUG] %s KeyPublic update from '%v' to '%v'", id, existingAgent.KeyPublic, agent.KeyPublic)
		pool.Peer[id] = agent
	} else {
		log.Printf("[DEBUG] Add new agent %s: %v", id, agent)
		// add new agent
		agent.errorCounter = 0
		agent.active = true
		pool.Peer[id] = agent
	}

	if pool.DbActive {
		err := pool.db.UpsertPeer(
			PeerInfoDB{
				Name:      id,
				Addr:      agent.Address,
				Master:    agent.Master,
				KeyPublic: agent.KeyPublic,
				Token:     agent.Token,
			},
		)
		if err != nil {
			log.Printf("[AGENTS] Failed to upsert peer in DB: %v", err)
		}
	}
}

// Add adds a new agent to the list with active status.
func (pool *AgentPool) add(id string, agent Agent) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	_, exists := pool.Peer[id]
	if exists {
		return errors.New("agent " + id + " already exist in pool")
	}

	agent.errorCounter = 0
	agent.active = true
	pool.Peer[id] = agent

	return nil
}

// Del removes an agent from the list and from the DB if active.
func (pool *AgentPool) Delete(id string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	delete(pool.Peer, id)

	// Delete from DB
	if pool.DbActive {
		err := pool.db.RemovePeer(id)
		if err != nil {
			log.Printf("[AGENTS] Failed to remove peer from DB: %v", err)
		}
	}
}

// IsErr reports whether an agent has exceeded the specified error threshold.
func (pool *AgentPool) IsErr(id string, num int) bool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	agent := pool.Peer[id]
	return agent.errorCounter >= num
}

// SetErr increments the error counter and marks the agent as inactive if the threshold is reached.
func (pool *AgentPool) SetErr(id string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	agent := pool.Peer[id]
	agent.errorCounter++
	if agent.errorCounter >= maxErrorCount {
		agent.active = false
	}
	pool.Peer[id] = agent
}

// ClearErr resets the error counter and marks the agent as active.
func (pool *AgentPool) ClearErr(id string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	agent := pool.Peer[id]
	agent.errorCounter = 0
	agent.active = true
	pool.Peer[id] = agent
}

// Active reports whether a specific agent is currently active.
func (pool *AgentPool) Active(id string) bool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	agent := pool.Peer[id]
	return agent.active
}

// ErrorCount returns the current error counter value for a specific agent.
func (pool *AgentPool) ErrorCount(id string) int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	agent := pool.Peer[id]
	return agent.errorCounter
}

func (pool *AgentPool) SetToken(id, token string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	agent := pool.Peer[id]
	agent.Token = token
	pool.Peer[id] = agent

	// update in DB
	if pool.DbActive {
		err := pool.db.UpdateToken(id, token)
		if err != nil {
			log.Printf("[AGENTS] Failed to update peer token in DB: %v", err)
		}
	}
}

// Masters returns a slice of agent names that are marked as master.
func (pool *AgentPool) Masters() []string {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	masters := make([]string, 0)
	for name, agent := range pool.Peer {
		log.Printf("[DEBUG] agent %s: %v", name, agent.Master)
		if agent.Master {
			masters = append(masters, name)
		}
	}
	log.Printf("[DEBUG] master list %v", masters)
	return masters
}

// IsMaster reports whether the agent with the given ID is marked as a master.
func (pool *AgentPool) IsMaster(id string) bool {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	log.Printf("[DEBUG] AgentPool: %v", pool.Peer)
	// log.Printf("[DEBUG] agent %s: %v", id, pool.Peer[id].Master)
	log.Printf("[DEBUG] agent %s: %v", id, pool.Peer[id])
	return pool.Peer[id].Master
}

// GetToken returns the token for the given agent ID.
func (pool *AgentPool) GetToken(id string) string {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	agent, ok := pool.Peer[id]
	if !ok {
		return ""
	}
	return agent.Token
}

func (pool *AgentPool) GetClusterToken(_ string) string {
	return pool.cfg.ClusterToken
}

// // UpsertPeer inserts or updates a peer in the database if DB is active.
// func (a *AgentPool) UpsertPeer(peer config.PeerInfo) error {
// 	a.mu.Lock()
// 	defer a.mu.Unlock()
// 	if !a.DbActive {
// 		return nil
// 	}
// 	return a.db.UpsertPeer(peer)
// }
