// Package config provides configuration loading and types for cloud-agent.
package config

import (
	"os"

	"fmt"

	"gopkg.in/yaml.v3"
)

// PeerInfo represents information about a peer agent.
type PeerInfo struct {
	Name   string `yaml:"name"`
	Addr   string `yaml:"addr"`
	Master bool   `yaml:"master"`
}

// Config holds the configuration for the cloud-agent.
type Config struct {
	TCPAddress        string     `yaml:"tcp_address"`
	SelfID            string     `yaml:"self_id"`
	Peers             []PeerInfo `yaml:"peers"`
	NumMasters        int        `yaml:"num_masters"`
	ElectionDelay     int        `yaml:"election_delay"`
	StatusLogInterval int        `yaml:"status_log_interval"`
	MasterTimeout     int        `yaml:"master_timeout"` // in seconds
}

// LoadConfig loads configuration from a YAML file and sets default values.
func LoadConfig(path string) (*Config, error) {
	var cfg Config
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	if cfg.ElectionDelay == 0 {
		cfg.ElectionDelay = 5
	}
	if cfg.StatusLogInterval == 0 {
		cfg.StatusLogInterval = 5
	}
	if cfg.MasterTimeout == 0 {
		cfg.MasterTimeout = 60
	}
	return &cfg, nil
}
