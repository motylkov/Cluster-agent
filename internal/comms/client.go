package comms

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
)

func NewAgentClient(address string) (*rpc.Client, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	client := rpc.NewClientWithCodec(jsonrpc.NewClientCodec(conn))

	return client, nil
}

func Send(c *rpc.Client, command string) (reply Reply, err error) {
	if c == nil {
		return reply, fmt.Errorf("client is nil")
	}
	args := Args{Message: command}
	err = c.Call("AgentService.Ping", args, &reply)
	// log?
	return reply, err
}

func SendAsync(c *rpc.Client, command string, handle func(*Reply, error)) {
	if c == nil {
		handle(nil, fmt.Errorf("client is nil"))
		return
	}

	args := Args{Message: command}
	var reply Reply

	call := c.Go("AgentService.Ping", args, &reply, nil)
	go func() {
		<-call.Done
		if call.Error != nil {
			log.Println("Async Ping error:", call.Error)
		} else {
			handle(&reply, call.Error)
		}
	}()
}

func SendAsyncWithErrors(c *rpc.Client, command string, handle func(*Reply, error)) {
	if c == nil {
		handle(nil, fmt.Errorf("client is nil"))
		return
	}
	args := Args{Message: command}
	var reply Reply
	call := c.Go("AgentService.Ping", args, &reply, nil)
	go func() {
		<-call.Done
		handle(&reply, call.Error)
	}()
}

func RegisterClient(c *rpc.Client, name, address string) (string, error) {
	args := Args{ID: name, Message: address}
	var reply Reply
	err := c.Call("AgentService.Register", args, &reply)
	return reply.Response, err
}
