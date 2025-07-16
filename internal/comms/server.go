// Package comms provides client and server communication utilities for cloud-agent.
package comms

import (
	"cloud-agent/internal/agents"
	"cloud-agent/internal/config"
	rpcservice "cloud-agent/internal/rpc"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
)

// NetService provides RPC methods for agent commands.
type NetService struct {
	SelfID     string
	agentPool  *agents.AgentPool
	config     *config.Config
	RemoteAddr string
	privateKey *[32]byte
	publicKey  *[32]byte
}

// Args represents arguments for RPC methods.
type Args struct {
	//	Sender         string // The name/ID of the sending agent
	ID             string
	ClusterToken   string // Shared secret for registration only
	AgentPublicKey string // Agent's ephemeral public key (base64-encoded)
	Message        string
	Address        string
	Signature      string // HMAC signature for message authentication
}

// Reply represents a standard reply for RPC methods.
type Reply struct {
	Response         string
	EncryptedHMACKey string // HMAC key encrypted with agent's public key (base64-encoded, for registration)
}

// ResponseReply represents an acknowledgement reply.
type ResponseReply struct {
	Ack bool
}

// uniq checks for an existing agent and connection status.
func (a *NetService) uniq(name, address string) (exists bool, active bool, reason string) {
	agent, ok := a.agentPool.Peer[name]
	if !ok {
		return false, false, ""
	}
	// Check if the address matches
	if agent.Address == address {
		if agent.Client != nil {
			// Try to ping the agent to check if the connection is active
			pingArgs := Args{ID: name, Message: "ping"}
			var pingReply Reply
			err := agent.Client.Call("JSONRPCService.Ping", pingArgs, &pingReply)
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
		err := agent.Client.Call("JSONRPCService.Ping", pingArgs, &pingReply)
		if err == nil {
			return true, true, "name already used at another address and connection is active"
		}
	}
	return true, false, "name already used at another address but connection is not active"
}

// NewAgentService constructs a new netService with all dependencies.
func NewNetService(cfg *config.Config, agentPool *agents.AgentPool, pubKey *[32]byte, privKey *[32]byte) *NetService {
	log.Printf("[DEBUG][NewNetService]: agentPool.Peer[%s] = %+v, Master = %v", cfg.SelfID, agentPool.Peer[cfg.SelfID], agentPool.Peer[cfg.SelfID].Master)

	return &NetService{
		SelfID:     cfg.SelfID,
		agentPool:  agentPool,
		config:     cfg,
		publicKey:  pubKey,
		privateKey: privKey,
	}
}

// StartServer starts the RPC server for the given netService and returns the listener for graceful shutdown.
func (service *NetService) StartServer() (net.Listener, error) {
	ln, err := net.Listen("tcp", service.config.TCPAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", service.config.TCPAddress, err)
	}

	log.Printf("[DEBUG] Before gorutine in StartServer: agentPool.Peer[%s] = %+v, Master = %v", service.SelfID, service.agentPool.Peer[service.SelfID], service.agentPool.Peer[service.SelfID].Master)

	go func() {
		log.Printf("[SERVER] Listening on %s", service.config.TCPAddress)
		log.Printf("[DEBUG] In 1 gorutine StartServer: agentPool.Peer[%s] = %+v, Master = %v", service.SelfID, service.agentPool.Peer[service.SelfID], service.agentPool.Peer[service.SelfID].Master)

		jsonrpcService := rpcservice.NewJSONRPCService(service.config, service.agentPool, service.publicKey)

		for {
			log.Printf("[DEBUG] Control point: Master = %v", service.agentPool.Peer[service.SelfID].Master)

			conn, err := ln.Accept()
			if err != nil {
				if err.Error() == "use of closed network connection" {
					// Listener closed, exit loop
					break
				}
				log.Println("Accept error:", err)
				continue
			}
			go func() {
				log.Printf("[DEBUG] In 2 gorutine StartServer: agentPool.Peer[%s] = %+v, Master = %v", service.SelfID, service.agentPool.Peer[service.SelfID], service.agentPool.Peer[service.SelfID].Master)
				server := rpc.NewServer()
				if err := server.Register(jsonrpcService); err != nil {
					log.Printf("[SERVER] Failed to register RPC service for connection %s: %v", conn.RemoteAddr().String(), err)
					_ = conn.Close()
					return
				}
				server.ServeCodec(jsonrpc.NewServerCodec(conn))
			}()
		}
		log.Printf("[SERVER] on %s is stopped", service.config.TCPAddress)
	}()
	return ln, nil
}
