package main

import (
	"cloud-agent/cmd"
	conf "cloud-agent/internal/config"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// VERSION is the current version of the cloud-agent.
const VERSION = "0.2"

func main() {
	// Graceful shutdown on interrupt
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdown
		log.Println("[SERVER] Received shutdown signal")
		log.Println("[SERVER] Shutting down...")
		os.Exit(0)
	}()

	// load config
	configPath := "config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	config, err := conf.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("[CONFIG] Failed to load config: %v", err)
	}

	// main runner
	cmd, err := cmd.NewAgent(config)
	if err != nil {
		log.Fatalln(err)
	}
	cmd.Run()
}
