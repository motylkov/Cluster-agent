// Package comms provides client and server communication utilities for cloud-agent.
package comms

import (
	"cloud-agent/internal/agents"
	"cloud-agent/internal/config"
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
)

// AgentService provides RPC methods for agent commands.
type AgentService struct {
	SelfID    string
	agentList *agents.AgentList
	config    *config.Config
}

// Args represents arguments for RPC methods.
type Args struct {
	ID      string
	Message string
}

// Reply represents a standard reply for RPC methods.
type Reply struct {
	Response string
}

// ResponseReply represents an acknowledgement reply.
type ResponseReply struct {
	Ack bool
}

// Ping is an exported method for AgentService to satisfy net/rpc requirements.
// It responds to ping requests from other agents.
func (a *AgentService) Ping(args Args, reply *Reply) error {
	// Authorize: only master can send ping
	agent, ok := a.agentList.Peer[args.ID]
	if !ok || !agent.Master {
		return errors.New("unauthorized: only master can send command")
	}

	reply.Response = "Pong from " + a.SelfID + ": " + args.Message
	log.Printf("[%s] Pong: %s", a.SelfID, args.Message)
	return nil
}

// Register adds a new agent to the agentList with checks.
func (a *AgentService) Register(args Args, reply *Reply) error {
	if !a.agentList.IsMaster(a.SelfID) {
		log.Printf("[Agent] I'm (%s) not a master: uncorrect register request from %s", a.SelfID, args.ID)
		reply.Response = "I'm not a master"
		return nil
	}
	name := args.ID
	address := args.Message
	if !a.authorize(name, address) {
		return errors.New("authorization failed")
	}
	exists, active, reason := a.uniq(name, address)
	if exists {
		if active {
			log.Printf("[SERVER] Registration refused for %s: %s", name, reason)
			reply.Response = reason
			return nil
		}
		log.Printf("[SERVER] Registration updating for %s: %s", name, reason)
		oldAgent := a.agentList.Peer[name]
		if oldAgent.Client != nil {
			if err := oldAgent.Client.Close(); err != nil {
				log.Printf("[SERVER] Error closing old client: %v", err)
			}
		}
		client, err := NewAgentClient(address)
		if err != nil {
			reply.Response = "failed to create new client connection"
			return fmt.Errorf("failed to create new client connection: %w", err)
		}
		// Create agent with active state and clear error counter
		agent := agents.Agent{Address: address, Master: false, Client: client}
		agent.ClearErr() // This sets active=true and errorCounter=0
		a.agentList.Peer[name] = agent
		reply.Response = "re-registered agent"
		log.Printf("[SERVER] re-registered %s with active=%v, errorCount=%d", name, agent.Active(), agent.ErrorCount())
		if name != a.SelfID {
			if agent.Master {
				peerInfo := config.PeerInfo{Name: name, Addr: address, Master: agent.Master}
				if err := a.config.UpdatePeer(peerInfo); err != nil {
					log.Printf("[SERVER] Failed to update peer in config: %v", err)
				} else if err := a.config.SaveConfig(); err != nil {
					log.Printf("[SERVER] Failed to save config: %v", err)
				}
			}
			a.agentList.Add(name, agent)
		}
		return nil
	}
	// Add new agent
	client, err := NewAgentClient(address)
	if err != nil {
		reply.Response = "failed to create new client connection"
		return fmt.Errorf("failed to create new client connection: %w", err)
	}
	// Create agent with active state and clear error counter
	agent := agents.Agent{Address: address, Master: false, Client: client}
	agent.ClearErr() // This sets active=true and errorCounter=0
	a.agentList.Peer[name] = agent
	reply.Response = "registered new agent"
	log.Printf("[SERVER] registered %s with active=%v, errorCount=%d", name, agent.Active(), agent.ErrorCount())
	if name != a.SelfID {
		if agent.Master {
			peerInfo := config.PeerInfo{Name: name, Addr: address, Master: agent.Master}
			if err := a.config.UpdatePeer(peerInfo); err != nil {
				log.Printf("[SERVER] Failed to update peer in config: %v", err)
			} else if err := a.config.SaveConfig(); err != nil {
				log.Printf("[SERVER] Failed to save config: %v", err)
			}
		}
		a.agentList.Add(name, agent)
	}
	return nil
}

// authorize is a stub for agent registration authorization.
func (a *AgentService) authorize(_ string, _ string) bool {
	// TODO: implement real authorization logic
	return true
}

// uniq checks for an existing agent and connection status.
func (a *AgentService) uniq(name, address string) (exists bool, active bool, reason string) {
	agent, ok := a.agentList.Peer[name]
	if !ok {
		return false, false, ""
	}
	// Check if the address matches
	if agent.Address == address {
		if agent.Client != nil {
			// Try to ping the agent to check if the connection is active
			pingArgs := Args{ID: name, Message: "ping"}
			var pingReply Reply
			err := agent.Client.Call("AgentService.Ping", pingArgs, &pingReply)
			if err == nil {
				return true, true, "address already used and connection is active"
			}
		}
		return true, false, "address already used but connection is not active"
	}
	// Address does not match, check if the old address is still active
	if agent.Client != nil {
		pingArgs := Args{ID: name, Message: "ping"}
		var pingReply Reply
		err := agent.Client.Call("AgentService.Ping", pingArgs, &pingReply)
		if err == nil {
			return true, true, "name already used at another address and connection is active"
		}
	}
	return true, false, "name already used at another address but connection is not active"
}

// StartServer starts the RPC server and returns the listener for graceful shutdown.
func StartServer(cfg *config.Config, aList *agents.AgentList) (net.Listener, error) {
	agent := &AgentService{
		SelfID:    cfg.SelfID,
		agentList: aList,
		config:    cfg,
	}
	if err := rpc.Register(agent); err != nil {
		return nil, fmt.Errorf("failed to register RPC service: %w", err)
	}

	ln, err := net.Listen("tcp", cfg.TCPAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", cfg.TCPAddress, err)
	}

	go func() {
		log.Printf("[SERVER] Listening on %s", cfg.TCPAddress)
		for {
			conn, err := ln.Accept()
			if err != nil {
				if err.Error() == "use of closed network connection" {
					// Listener closed, exit loop
					break
				}
				log.Println("Accept error:", err)
				continue
			}
			go rpc.ServeCodec(jsonrpc.NewServerCodec(conn))
		}
		log.Printf("[SERVER] on %s is stopped", cfg.TCPAddress)
	}()
	return ln, nil
}
