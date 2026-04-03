package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type ServerConfig struct {
	DatabaseURL           string `json:"database_url"`
	ListenAddr            string `json:"listen_addr"`
	IllegalTrafficForward string `json:"illegal_traffic_forward"`
	EnableTLS             bool   `json:"enable_tls"`
	TLSCert               string `json:"tls_cert"`
	TLSKey                string `json:"tls_key"`
	WebBaseDir            string `json:"web_base_dir"`
	WebAddr               string `json:"web_addr"`
	WebUsername           string `json:"web_username"`
	WebPassword           string `json:"web_password"`
	Quiet                 bool   `json:"quiet"`
	LogDir                string `json:"log_dir"`
}

func (c *ServerConfig) Normalize() {
	if c.LogDir == "" {
		c.LogDir = "logs"
	}
}

func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open config file: %w", err)
	}
	var cfg ServerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	cfg.Normalize()
	return &cfg, nil
}
