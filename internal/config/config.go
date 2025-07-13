// Package config provides configuration loading and types for cloud-agent.
package config

import (
	"os"

	"fmt"

	"gopkg.in/yaml.v3"
)

const configFilePerm = 0600

// PeerInfo represents information about a peer agent.
type PeerInfo struct {
	Name   string `yaml:"name"`
	Addr   string `yaml:"addr"`
	Master bool   `yaml:"master"`
}

// Config holds the configuration for the cloud-agent.
type Config struct {
	path              string
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
	cfg.path = path
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

// UpdatePeer updates a peer's info in the config, except for the self peer.
func (c *Config) UpdatePeer(peer PeerInfo) error {
	if peer.Name == c.SelfID {
		return fmt.Errorf("cannot update self peer (%s)", c.SelfID)
	}
	for i, p := range c.Peers {
		if p.Name == peer.Name {
			c.Peers[i] = peer
			return nil
		}
	}
	// If not found, add as new peer
	c.Peers = append(c.Peers, peer)
	return nil
}

// SaveConfig writes the config to the given file path in YAML format.
func (c *Config) SaveConfig(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(c.path, data, configFilePerm); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}
