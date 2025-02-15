package comms

import (
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
)

type AgentService struct {
	SelfID string
}

type Args struct {
	ID      string
	Message string
}

type Reply struct {
	Response string
}

func (a *AgentService) Ping(args Args, reply *Reply) error {
	reply.Response = "Pong from " + a.SelfID + ": " + args.Message
	log.Printf("[%s] Pong: %s", a.SelfID, args.Message)
	return nil
}

func StartServer(address, selfID string) (net.Listener, error) {
	agent := &AgentService{
		SelfID: selfID,
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
