package main

import (
	comms "agent/internal/comms"
	conf "agent/internal/config"
	"log"
	"os"
	"os/signal"
	"syscall"
)

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

	listener, err := comms.StartServer(cfg.TCPAddress, cfg.SelfID)
	if err != nil {
		log.Fatalf("[SERVER] Failed to start server: %v", err)
	}
	defer listener.Close()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdown
		log.Println("[SERVER] Shutting down...")
		// listener.Close()
		os.Exit(0)
	}()

	// main loop
	for {

	}
}
