package main

import (
	comms "agent/internal/comms"
	conf "agent/internal/config"
	masteragent "agent/internal/master"
	"agent/internal/types"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const VERSION = "0.1"

// contains checks if a target value exists in a slice of comparable type
func contains[t comparable](s []t, target t) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}

// isConnectionClosed checks if an error indicates a closed connection
func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "connection is shut down" || msg == "EOF"
}

func main() {
	log.Println("[AGENT] Agent starting...")

	configPath := "../config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	cfg, err := conf.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("[CONFIG] Failed to load config: %v", err)
	}

	currentMaster := make([]string, 0)
	isMaster := false
	registered := false

	var masterChan chan masteragent.Command

	agentList := make(types.AgentList, len(cfg.Peers)+1)
	for _, peer := range cfg.Peers {
		currentMaster = append(currentMaster, peer.Name)
		agentList.Add(peer.Name, types.Agent{Address: peer.Addr, Master: peer.Master})
		agentList[peer.Name] = agentList.ClearErr(peer.Name)
	}

	// Start the TCP server
	listener, err := comms.StartServer(cfg.TCPAddress, cfg.SelfID, &agentList)
	if err != nil {
		log.Fatalf("[SERVER] Failed to start server: %v", err)
	}
	defer listener.Close()

	// Graceful shutdown on interrupt
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdown
		log.Println("[SERVER] Shutting down...")
		// listener.Close()
		os.Exit(0)
	}()

	master := masteragent.NewMasterService(cfg.SelfID, &agentList, cfg.StatusLogInterval)

	// main loop
	for {
		if !isMaster {
			if len(currentMaster) == 0 {
				log.Println("[MASTER] Master is not defined, attempting to discover...")
				// TODO: Discovery request to master
			} else {
				//	if len(currentMaster) > 0 {
				if contains(currentMaster, cfg.SelfID) {
					log.Println("[MASTER] I am the master!")
					if !master.Running() {
						masterChan = master.Start(context.Background())
					}
					isMaster = true
				} else {
					if master.Running() {
						master.Stop()
						if masterChan != nil {
							close(masterChan)
							masterChan = nil
						}
					}
					isMaster = false
				}

				if !isMaster {
					if !registered {
						for _, masterName := range currentMaster {
							agent, ok := agentList[masterName]
							if ok && agent.Master {
								client, err := comms.NewAgentClient(agent.Address)
								if err != nil {
									log.Printf("Failed to connect to master %s: %v", masterName, err)
									continue
								}
								defer client.Close()

								response, err := comms.RegisterClient(client, cfg.SelfID, cfg.TCPAddress)
								if err != nil {
									log.Printf("Failed to register with master %s: %v", masterName, err)
								} else {
									if response == "already registered" || response == "updated inactive agent" || response == "registered new agent" || response == "re-registered agent" {
										log.Printf("Successfully registered with master %s", masterName)
										registered = true
									} else {
										log.Printf("Register with master %s returned: %s", masterName, response)
									}
								}
								break
							}
							time.Sleep(1 * time.Second)
						}
						time.Sleep(5 * time.Second)
					}
				}
			}
		}
	}
}
