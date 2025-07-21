// Package comms provides client and server communication utilities for cloud-agent.
package comms

import (
	auth "cloud-agent/internal/auth"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
)

var selfToken string // Package-level variable to store the agent's HMAC token
var selfPubKey *[32]byte
var selfPrivKey *[32]byte
var selfPubKeyB64 string

type ReplyCallbackFunc func(*Reply, error)

// NewAgentClient creates a new JSON-RPC client connection to the given address (IP:Port).
func NewAgentClient(address string) (*rpc.Client, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}
	client := rpc.NewClientWithCodec(jsonrpc.NewClientCodec(conn))
	return client, nil
}

func ComputeHMAC(sender, message, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(sender + message))
	return hex.EncodeToString(mac.Sum(nil))
}

// SendRPC is a universal RPC call helper for any method name.
func SendRPC(c *rpc.Client, method string, args interface{}, reply interface{}) error {
	if c == nil {
		return errors.New("client is nil")
	}
	return c.Call(method, args, reply)
}

// Send sends a synchronous command to the RPC client and returns the reply.
func Send(c *rpc.Client, method, sender, token string) (reply Reply, err error) {
	if c == nil {
		return reply, errors.New("client is nil")
	}
	if token == "" {
		token = selfToken
	}
	log.Printf("[CLIENT] Send: HMAC token for sender %s: %s", sender, token)
	signature := auth.ComputeHMAC(sender, method, token)
	args := Args{Message: method, ID: sender, Signature: signature}
	err = SendRPC(c, method, args, &reply)
	if err != nil {
		return reply, err
	}
	return reply, nil
}

// SendAsync sends an asynchronous command to the RPC client and invokes the callback with the reply and error.
func SendAsync(c *rpc.Client, method, sender, token string, handle func(*Reply, error)) {
	if c == nil {
		handle(nil, errors.New("client is nil"))
		return
	}
	if token == "" {
		token = selfToken
	}
	log.Printf("[CLIENT] SendAsync: HMAC token for sender %s: %s", sender, token)
	signature := auth.ComputeHMAC(sender, method, token)
	args := Args{Message: method, ID: sender, Signature: signature}
	var reply Reply
	call := c.Go(method, args, &reply, nil)
	go func() {
		<-call.Done
		if call.Error != nil {
			log.Println("Async call error:", call.Error)
		} else {
			handle(&reply, call.Error)
		}
	}()
}

// SendAsyncWithErrors sends a command asynchronously and calls the callback with the reply and error.
func SendAsyncWithErrors(c *rpc.Client, method, sender, token string, handle func(*Reply, error)) {
	if c == nil {
		handle(nil, errors.New("client is nil"))
		return
	}
	if token == "" {
		token = selfToken
	}
	log.Printf("[CLIENT] SendAsyncWithErrors: HMAC token for sender %s: %s", sender, token)
	signature := auth.ComputeHMAC(sender, method, token)
	args := Args{Message: method, ID: sender, Signature: signature}
	var reply Reply
	call := c.Go(method, args, &reply, nil)
	go func() {
		<-call.Done
		handle(&reply, call.Error)
	}()
}

func (service *NetService) saveToken(sender, token string) {
	log.Printf("[CLIENT] saveToken %s %s", sender, token)
	service.agentPool.SetToken(sender, token)
}

// RegisterClient sends a registration message to the server with the client's name and address.
func (service *NetService) RegisterClient(c *rpc.Client, sender, address, token string) (string, error) {
	if c == nil {
		return "", errors.New("client is nil")
	}
	publicKey64 := base64.StdEncoding.EncodeToString(service.publicKey[:])
	log.Printf("[CLIENT] publicKey %v", service.publicKey)
	log.Printf("[CLIENT] publicKey64 %s", publicKey64)

	log.Printf("[CLIENT] Sending registration from %s addr %s to master", sender, address)
	signature := auth.ComputeHMAC(sender, publicKey64, token)
	args := Args{ID: sender, ClusterToken: token, AgentPublicKey: publicKey64, Address: address, Signature: signature}

	var reply Reply
	err := c.Call("JSONRPCService.Register", args, &reply)
	if err != nil {
		return "", fmt.Errorf("failed to call AgentService.Register: %w", err)
	}
	if reply.EncryptedHMACKey != "" {
		decrypted, err := auth.DecryptWithPrivateKey(reply.EncryptedHMACKey, service.privateKey)
		if err != nil {
			log.Printf("[CLIENT] Failed to decrypt HMAC key from master: %v", err)
		} else {
			selfToken = base64.StdEncoding.EncodeToString(decrypted)
			// save token in agentPool and DB
			service.agentPool.SetToken(sender, selfToken)
			log.Printf("[CLIENT] Registered and received HMAC token: %s", selfToken)
		}
	} else {
		log.Printf("[CLIENT] Registration failed: %s", reply.Response)
	}
	return reply.Response, nil
}
