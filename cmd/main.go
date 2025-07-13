// Package main implements the entry point for the cloud-agent process.
package main

import (
	"cloud-agent/internal/agents"
	comms "cloud-agent/internal/comms"
	conf "cloud-agent/internal/config"
	masteragent "cloud-agent/internal/master"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// VERSION is the current version of the cloud-agent.
const VERSION = "0.1"

const (
	retrySleepSeconds      = 5
	registerRetrySleepSecs = 1
)

// contains reports whether target is present in the slice s.
func contains[t comparable](s []t, target t) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}

// main is the entry point for the agent process. It loads configuration, starts the server, and manages master/agent roles.
func main() {
	log.Println("[CLOUD-AGENT] Agent starting...")

	configPath := "../config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	cfg, err := conf.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("[CONFIG] Failed to load config: %v", err)
	}

	isMaster := false
	registered := false

	var masterChan chan masteragent.Command

	// Initialize AgentList from config
	agentList := agents.NewAgentList(cfg.SelfID, cfg.TCPAddress, cfg.Peers)

	// Fill currentMaster slice from agentList
	currentMaster := agentList.Masters()

	// Start the TCP server for this agent
	listener, err := comms.StartServer(cfg.TCPAddress, cfg.SelfID, &agentList, cfg)
	if err != nil {
		log.Fatalf("[SERVER] Failed to start server: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Printf("[SERVER] Error closing listener: %v", err)
		}
	}()

	// Graceful shutdown on interrupt
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdown
		log.Println("[SERVER] Shutting down...")
		if err := listener.Close(); err != nil {
			log.Printf("[SERVER] Error closing listener: %v", err)
		}
		os.Exit(0)
	}()

	master := masteragent.NewService(cfg.SelfID, &agentList, cfg.StatusLogInterval)

	// main loop
	for {
		if !isMaster {
			if len(currentMaster) == 0 {
				// TODO: Discover a master
				log.Println("[MASTER] Master is not defined, attempting to discover...")
				// TODO: Send discovery request to master
				// TODO: Wait for response
				// TODO: If response is valid, set currentMaster to response
				// TODO: If response is not valid, set currentMaster to "" and continue to next iteration
				// TODO: If no response is received after a timeout, set currentMaster to "" and continue to next iteration
				// TODO: If no response is received after a timeout, set currentMaster to "" and continue to next iteration
			} else {
				if contains(currentMaster, cfg.SelfID) {
					log.Println("[MASTER] I am the master!")
					// if master gorutine is not running, start it
					if !master.Running() {
						masterChan = master.Start(context.Background())
					}
					isMaster = true
				} else {
					// if master gorutine is running, stop it
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
								defer func() {
									if err := client.Close(); err != nil {
										log.Printf("[CLIENT] Error closing client: %v", err)
									}
								}()

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
