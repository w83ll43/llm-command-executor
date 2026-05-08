package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"llm-command-executor/internal/auth"
	"llm-command-executor/internal/domain"
)

type Config struct {
	HTTP     HTTPConfig           `json:"http"`
	SSH      SSHConfig            `json:"ssh"`
	Servers  []domain.Server      `json:"servers"`
	Commands []domain.CommandSpec `json:"commands"`
	Tokens   []domain.APIToken    `json:"tokens"`
	Policies []domain.Policy      `json:"policies"`
}

type HTTPConfig struct {
	Addr string `json:"addr"`
}

type SSHConfig struct {
	KnownHostsPath           string `json:"known_hosts_path"`
	InsecureSkipHostKeyCheck bool   `json:"insecure_skip_host_key_check"`
}

func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Normalize() error {
	if c.HTTP.Addr == "" {
		c.HTTP.Addr = ":8080"
	}
	if len(c.Servers) == 0 {
		return errors.New("config must contain at least one server")
	}
	if len(c.Commands) == 0 {
		return errors.New("config must contain at least one command")
	}
	if len(c.Tokens) == 0 {
		return errors.New("config must contain at least one token")
	}
	for i := range c.Tokens {
		if c.Tokens[i].TokenHash == "" {
			if c.Tokens[i].Token == "" {
				return fmt.Errorf("token %q must provide token or token_hash", c.Tokens[i].ID)
			}
			c.Tokens[i].TokenHash = auth.HashToken(c.Tokens[i].Token)
			c.Tokens[i].Token = ""
		}
	}
	for i := range c.Commands {
		if c.Commands[i].TimeoutSeconds <= 0 {
			c.Commands[i].TimeoutSeconds = 30
		}
		if c.Commands[i].MaxOutputBytes <= 0 {
			c.Commands[i].MaxOutputBytes = 64 * 1024
		}
	}
	return nil
}
