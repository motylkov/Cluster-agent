// Package comms provides client and server communication utilities for cloud-agent.
package comms

import (
	"cloud-agent/internal/agenttypes"
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
	agentList *agenttypes.AgentList
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
	reply.Response = "Pong from " + a.SelfID + ": " + args.Message
	log.Printf("[%s] Pong: %s", a.SelfID, args.Message)
	return nil
}

// Register adds a new agent to the agentList with checks.
func (a *AgentService) Register(args Args, reply *Reply) error {
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
		oldAgent := (*a.agentList)[name]
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
		agent := agenttypes.Agent{Address: address, Master: false, Client: client}
		agent.ClearErr() // This sets active=true and errorCounter=0
		(*a.agentList)[name] = agent
		reply.Response = "re-registered agent"
		log.Printf("[SERVER] re-registered %s with active=%v, errorCount=%d", name, agent.Active(), agent.ErrorCount())
		return nil
	}
	// Add new agent
	client, err := NewAgentClient(address)
	if err != nil {
		reply.Response = "failed to create new client connection"
		return fmt.Errorf("failed to create new client connection: %w", err)
	}
	// Create agent with active state and clear error counter
	agent := agenttypes.Agent{Address: address, Master: false, Client: client}
	agent.ClearErr() // This sets active=true and errorCounter=0
	(*a.agentList)[name] = agent
	reply.Response = "registered new agent"
	log.Printf("[SERVER] registered %s with active=%v, errorCount=%d", name, agent.Active(), agent.ErrorCount())
	return nil
}

// authorize is a stub for agent registration authorization.
func (a *AgentService) authorize(_ string, _ string) bool {
	// TODO: implement real authorization logic
	return true
}

// uniq checks for an existing agent and connection status.
func (a *AgentService) uniq(name, address string) (exists bool, active bool, reason string) {
	agent, ok := (*a.agentList)[name]
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
func StartServer(address, selfID string, aList *agenttypes.AgentList) (net.Listener, error) {
	agent := &AgentService{
		SelfID:    selfID,
		agentList: aList,
	}
	if err := rpc.Register(agent); err != nil {
		return nil, fmt.Errorf("failed to register RPC service: %w", err)
	}

	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	go func() {
		log.Printf("[SERVER] Listening on %s", address)
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
		log.Printf("[SERVER] on %s is stopped", address)
	}()
	return ln, nil
}
