package rpc

import (
	"cloud-agent/internal/agents"
	auth "cloud-agent/internal/auth"
	"cloud-agent/internal/config"
	"encoding/base64"
	"errors"
	"log"
)

type JSONRPCService struct {
	SelfID     string
	agentPool  *agents.AgentPool
	config     *config.Config
	RemoteAddr string
	publicKey  *[32]byte
}

func NewJSONRPCService(cfg *config.Config, pool *agents.AgentPool, pubKey *[32]byte) *JSONRPCService {
	return &JSONRPCService{
		SelfID:    cfg.SelfID,
		agentPool: pool,
		config:    cfg,
		publicKey: pubKey,
	}
}

func (a *JSONRPCService) Ping(args struct {
	ID        string
	Sender    string
	Message   string
	Signature string
}, reply *struct {
	Response string
}) error {
	log.Printf("[PING] Received ping from: %s, remote address: %s, args: %+v", args.Sender, a.RemoteAddr, args)
	if !auth.VerifyHMAC(a.agentPool.GetToken, args.Sender, args.Message, args.Signature) {
		return errors.New("unauthorized: invalid HMAC signature")
	}
	// Authorize: only master can send ping
	agent, ok := a.agentPool.Peer[args.Sender]
	log.Printf("[DEBUG ping] agent %s ok: %v", args.Sender, ok)
	log.Printf("[DEBUG ping] agent %s master: %v", args.Sender, agent.Master)
	if !ok || !agent.Master {
		return errors.New("unauthorized: only master can send command")
	}

	reply.Response = "Pong from " + a.SelfID + ": " + args.Message
	log.Printf("[%s] Pong: %s", a.SelfID, args.Message)
	return nil
}

func (a *JSONRPCService) Register(args struct {
	ID string
	//	Sender         string
	ClusterToken   string
	AgentPublicKey string
	Message        string
	Signature      string
}, reply *struct {
	Response         string
	EncryptedHMACKey string
}) error {
	log.Printf("[DEBUG] Expected cluster token: %s, got: %s", a.config.ClusterToken, args.ClusterToken)

	if args.ClusterToken != a.config.ClusterToken {
		return errors.New("unauthorized: invalid cluster token")
	}

	if args.Signature == "" {
		return errors.New("unauthorized: not defined HMAC signature for registration")
	}

	if !auth.VerifyHMAC(a.agentPool.GetClusterToken, args.ID, args.AgentPublicKey, args.Signature) {
		return errors.New("unauthorized: invalid HMAC signature for registration")
	}

	// a.agentPool.GetToken

	// commented for debug
	//log.Printf("[DEBUG] Master is: %v", a.agentPool.Peer[a.SelfID].Master)
	//if !a.agentPool.IsMaster(a.SelfID) {
	//	log.Printf("[Agent] I'm (%s) not a master: uncorrect register request from %s", a.SelfID, args.ID)
	//	reply.Response = "I'm not a master"
	//	return nil
	//}

	// name := args.ID
	// address := args.Message
	// if !auth.Authorize(name, address) {
	// 	return errors.New("authorization failed")
	// }

	// Decode agent's public key
	var agentPubKey [32]byte
	if args.AgentPublicKey == "" {
		reply.Response = "missing agent public key"
		return nil
	}
	pubBytes, err := base64.StdEncoding.DecodeString(args.AgentPublicKey)
	if err != nil || len(pubBytes) != 32 {
		reply.Response = "invalid agent public key"
		return nil
	}
	copy(agentPubKey[:], pubBytes)

	// Generate a unique HMAC key for the agent
	hmacKey := auth.GenerateRandomToken(32)
	hmacKeyBytes, _ := base64.StdEncoding.DecodeString(hmacKey)

	// // Encrypt the HMAC key with the agent's public key
	encryptedHMAC, err := auth.EncryptWithPublicKey(hmacKeyBytes, &agentPubKey)
	if err != nil {
		reply.Response = "failed to encrypt HMAC key"
		return nil
	}

	// // Store the agent's public key and HMAC key in agentList/DB
	// log.Println("[------------------------------------]")
	// log.Printf("[DEBUG] Master set false for %s", name)
	// log.Println("[------------------------------------]")
	// agent := agents.Agent{Address: address, Master: false, Token: hmacKey, PublicKey: args.AgentPublicKey}
	// a.agentPool.ClearErr(name)
	// a.agentPool.Peer[name] = agent
	// if a.agentPool.DbActive {
	// 	err := a.agentPool.UpsertPeer(config.PeerInfo{
	// 		Name:   name,
	// 		Addr:   address,
	// 		Master: false,
	// 		Token:  hmacKey,
	// 		// Optionally add PublicKey to PeerInfo if you extend the schema
	// 	})
	// 	if err != nil {
	// 		log.Printf("[SERVER] Failed to upsert peer in DB: %v", err)
	// 	}
	// }

	reply.Response = "registered new agent"
	reply.EncryptedHMACKey = encryptedHMAC
	log.Printf("[SERVER] registered agent %s with new HMAC key", args.ID)
	return nil
}
