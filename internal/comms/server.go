package comms

import (
	"agent/internal/types"
	"errors"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
)

type AgentService struct {
	SelfID    string
	agentList *types.AgentList
}

type Args struct {
	ID      string
	Message string
}

type Reply struct {
	Response string
}

type ResponseReply struct {
	Ack bool
}

func (a *AgentService) Ping(args Args, reply *Reply) error {
	reply.Response = "Pong from " + a.SelfID + ": " + args.Message
	log.Printf("[%s] Pong: %s", a.SelfID, args.Message)
	return nil
}

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
		} else {
			log.Printf("[SERVER] Registration updating for %s: %s", name, reason)
			oldAgent := (*a.agentList)[name]
			if oldAgent.Client != nil {
				oldAgent.Client.Close()
			}
			client, err := NewAgentClient(address)
			if err != nil {
				reply.Response = "failed to create new client connection"
				return nil
			}
			agent := types.Agent{Address: address, Master: false, Client: client}
			agent.ClearErr() // This sets active=true and errorCounter=0
			(*a.agentList)[name] = agent
			reply.Response = "re-registered agent"
			log.Printf("[SERVER] re-registered %s with active=%v, errorCount=%d", name, agent.Active(), agent.ErrorCount())
			return nil
		}
	}

	client, err := NewAgentClient(address)
	if err != nil {
		reply.Response = "failed to create new client connection"
		return nil
	}

	agent := types.Agent{Address: address, Master: false, Client: client}
	agent.ClearErr() // This sets active=true and errorCounter=0
	(*a.agentList)[name] = agent
	reply.Response = "registered new agent"
	log.Printf("[SERVER] registered %s with active=%v, errorCount=%d", name, agent.Active(), agent.ErrorCount())
	return nil
}

func (a *AgentService) authorize(name, address string) bool {
	// real authorization logic
	return true
}

func (a *AgentService) uniq(name, address string) (exists bool, active bool, reason string) {
	agent, ok := (*a.agentList)[name]
	if !ok {
		return false, false, ""
	}

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

func StartServer(address, selfID string, aList *types.AgentList) (net.Listener, error) {
	agent := &AgentService{
		SelfID:    selfID,
		agentList: aList,
	}
	rpc.Register(agent)

	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	go func() {
		log.Printf("[SERVER] Listening on %s", address)
		for {
			conn, err := ln.Accept()
			if err != nil {
				if err.Error() == "use of closed network connection" {
					// Listener closed
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
