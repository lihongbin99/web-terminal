package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Auth     AuthConfig     `yaml:"auth"`
	Terminal TerminalConfig `yaml:"terminal"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type AuthConfig struct {
	Username      string        `yaml:"username"`
	Password      string        `yaml:"password"`
	MaxAttempts   int           `yaml:"max_attempts"`
	BlockDuration time.Duration `yaml:"block_duration"`
}

type TerminalConfig struct {
	Shell string `yaml:"shell"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Auth.MaxAttempts == 0 {
		cfg.Auth.MaxAttempts = 5
	}
	if cfg.Auth.BlockDuration == 0 {
		cfg.Auth.BlockDuration = 30 * time.Minute
	}
	if cfg.Terminal.Shell == "" {
		cfg.Terminal.Shell = "cmd.exe"
	}
	return &cfg, nil
}
