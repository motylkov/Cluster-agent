package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type PeerInfo struct {
	Name   string `yaml:"name"`
	Addr   string `yaml:"addr"`
	Master bool   `yaml:"master"`
}

type Config struct {
	TCPAddress        string     `yaml:"tcp_address"`
	SelfID            string     `yaml:"self_id"`
	Peers             []PeerInfo `yaml:"peers"`
	NumMasters        int        `yaml:"num_masters"`
	ElectionDelay     int        `yaml:"election_delay"`
	StatusLogInterval int        `yaml:"status_log_interval"`
	MasterTimeout     int        `yaml:"master_timeout"` // in seconds
}

// LoadConfig loads configuration from a YAML file and sets default values
func LoadConfig(path string) (*Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
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
