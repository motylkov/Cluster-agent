// Package comms provides client and server communication utilities for cloud-agent.
package comms

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
)

// NewAgentClient creates a new JSON-RPC client connection to the given address (IP:Port).
func NewAgentClient(address string) (*rpc.Client, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}
	client := rpc.NewClientWithCodec(jsonrpc.NewClientCodec(conn))
	return client, nil
}

// Send sends a synchronous ping command to the RPC client and returns the reply.
func Send(c *rpc.Client, command string) (reply Reply, err error) {
	if c == nil {
		return reply, errors.New("client is nil")
	}
	args := Args{Message: command}
	err = c.Call("AgentService.Ping", args, &reply)
	// log?
	if err != nil {
		return reply, fmt.Errorf("failed to call AgentService.Ping: %w", err)
	}
	return reply, nil
}

// SendAsync sends an asynchronous ping command to the RPC client and invokes the callback with the reply and error.
func SendAsync(c *rpc.Client, command string, handle func(*Reply, error)) {
	if c == nil {
		handle(nil, errors.New("client is nil"))
		return
	}
	args := Args{Message: command}
	var reply Reply
	// c.Go("AgentService.Ping", args, &reply, nil)
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

// SendAsyncWithErrors sends a ping command asynchronously and calls the callback with the reply and error.
func SendAsyncWithErrors(c *rpc.Client, command string, handle func(*Reply, error)) {
	if c == nil {
		handle(nil, errors.New("client is nil"))
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

// RegisterClient sends a registration message to the server with the client's name and address.
func RegisterClient(c *rpc.Client, name, address string) (string, error) {
	args := Args{ID: name, Message: address}
	var reply Reply
	err := c.Call("AgentService.Register", args, &reply)
	if err != nil {
		return "", fmt.Errorf("failed to call AgentService.Register: %w", err)
	}
	return reply.Response, nil
}
