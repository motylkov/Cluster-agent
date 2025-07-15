// Package main implements the entry point for the cloud-agent process.
package cmd

import (
	"cloud-agent/internal/agents"
	"cloud-agent/internal/auth"
	comms "cloud-agent/internal/comms"
	conf "cloud-agent/internal/config"
	masteragent "cloud-agent/internal/master"
	"context"
	"encoding/base64"
	"log"
	"net"
	"slices"
	"time"
)

const (
	retrySleepSeconds      = 5
	registerRetrySleepSecs = 1
)

type Cmd struct {
	cfg              *conf.Config
	isMaster         bool
	registered       bool
	masterService    *masteragent.Service
	netService       *comms.NetService
	agentPoolService *agents.AgentPool
	masterChan       chan masteragent.Command
	netListener      net.Listener
	keyPublic        *[32]byte
	keyPrivate       *[32]byte
	keyPublic64      string
}

func NewAgent(conf *conf.Config) (*Cmd, error) {
	// generate new keys
	kPub, kPriv, err := auth.GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	return &Cmd{
		cfg:         conf,
		isMaster:    false,
		registered:  false,
		keyPublic:   kPub,
		keyPrivate:  kPriv,
		keyPublic64: base64.StdEncoding.EncodeToString(kPub[:]),
	}, nil
}

// main is the entry point for the agent process. It loads configuration, starts the server, and manages master/agent roles.
func (cmd *Cmd) Run() {
	var err error
	log.Println("[CLOUD-AGENT] Agent starting...")

	// Initialize AgentList from config (masters only)
	cmd.agentPoolService = agents.NewAgentPool(cmd.cfg, cmd.keyPublic)
	cmd.agentPoolService.Init()
	log.Printf("[DEBUG] agentPoolService: %v", cmd.agentPoolService)

	currentMaster := cmd.agentPoolService.Masters()
	log.Printf("[DEBUG] currentMaster: %v", currentMaster)

	log.Printf("[DEBUG][MAIN] Before StartServer: agentPool.Peer[%s] = %+v, Master = %v", cmd.cfg.SelfID, cmd.agentPoolService.Peer[cmd.cfg.SelfID], cmd.agentPoolService.Peer[cmd.cfg.SelfID].Master)

	// Construct NetService
	cmd.netService = comms.NewNetService(cmd.cfg, cmd.agentPoolService, cmd.keyPublic, cmd.keyPrivate)

	// Start the TCP server for self agent
	cmd.netListener, err = cmd.netService.StartServer()
	if err != nil {
		log.Fatalf("[SERVER] Failed to start server: %v", err)
	}
	defer func() {
		if err := cmd.netListener.Close(); err != nil {
			log.Printf("[SERVER] Error closing listener: %v", err)
		}
	}()

	log.Println("[DEBUG] agentPool after StartServer: %v", cmd.agentPoolService)

	cmd.masterService = masteragent.NewService(cmd.cfg, cmd.agentPoolService)

	// main loop
	for {
		if !cmd.isMaster {
			if len(currentMaster) == 0 {
				currentMaster = cmd.agentPoolService.Masters()
				if len(currentMaster) == 0 {
					// TODO: Discover a master
					log.Println("[MASTER] Master is not defined, attempting to discover...")
					// TODO: Send discovery request to master
					// TODO: Wait for response
					// TODO: If response is valid, set currentMaster to response
					// TODO: If response is not valid, set currentMaster to "" and continue to next iteration
					// TODO: If no response is received after a timeout, set currentMaster to "" and continue to next iteration
					// TODO: If no response is received after a timeout, set currentMaster to "" and continue to next iteration
					currentMaster = cmd.agentPoolService.Masters()
				}
			}
			if len(currentMaster) > 0 {
				if slices.Contains(currentMaster, cmd.cfg.SelfID) {
					log.Println("[MASTER] I am the master!")
					// if master gorutine is not running, start it
					if !cmd.masterService.Running() {
						log.Println("[MASTER] start")
						cmd.masterChan = cmd.masterService.Start(context.Background())
					}
					cmd.isMaster = true
				} else {
					// if master gorutine is running, stop it
					if cmd.masterService.Running() {
						log.Println("[MASTER] stop")
						cmd.masterService.Stop()
						if cmd.masterChan != nil {
							close(cmd.masterChan)
							cmd.masterChan = nil
						}
					}
					currentMaster = cmd.agentPoolService.Masters()
					cmd.isMaster = false
				}

				if !cmd.isMaster {
					if !cmd.registered {
						for _, masterName := range currentMaster {
							agent, ok := cmd.agentPoolService.Peer[masterName]
							log.Printf("[REGDEBUG] select agent %s: %v", masterName, agent)

							if ok && agent.Master {
								client, err := comms.NewAgentClient(agent.Address)
								if err != nil {
									log.Printf("Failed to connect to master %s: %v", masterName, err)
									continue
								}
								defer func() {
									if err := client.Close(); err != nil {
										log.Printf("[CLIENT] Error closing client: %v", err)
									}
								}()

								regToken := cmd.cfg.ClusterToken
								// for future, do not deleate
								//		if agent.Token != "" {
								//			regToken = agent.Token
								//		}
								response, err := cmd.netService.RegisterClient(client, cmd.cfg.SelfID, cmd.cfg.TCPAddress, regToken)
								if err != nil {
									log.Printf("Failed to register with master %s: %v", masterName, err)
								} else {
									if response == "already registered" || response == "updated inactive agent" || response == "registered new agent" || response == "re-registered agent" {
										log.Printf("Successfully registered with master %s", masterName)
										cmd.registered = true
									} else {
										log.Printf("Register with master %s returned: %s", masterName, response)
									}
									// Reload the HMAC token after registration
									comms.InitSelfToken(cmd.cfg.SelfID)
								}
								break // Register with the first master found
							}
							time.Sleep(registerRetrySleepSecs * time.Second) // Wait before next trying
						}
						time.Sleep(retrySleepSeconds * time.Second) // Wait before retrying
					}
				}
			}
		}
	}
}
