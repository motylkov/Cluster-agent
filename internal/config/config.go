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
	TCPAddress string     `yaml:"tcp_address"`
	SelfID     string     `yaml:"self_id"`
	Peers      []PeerInfo `yaml:"peers"`
}

func LoadConfig(path string) (*Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
