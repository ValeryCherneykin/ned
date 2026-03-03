package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Defaults Defaults        `yaml:"defaults"`
	Hosts    map[string]Host `yaml:"hosts"`
}

type Defaults struct {
	User     string `yaml:"user"`
	Port     string `yaml:"port"`
	Identity string `yaml:"identity"`
}

type Host struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Port     string `yaml:"port"`
	Identity string `yaml:"identity"`
}

func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Config{}, nil
	}

	path := filepath.Join(home, ".ned", "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) ResolveAlias(alias string) (Host, bool) {
	if c.Hosts == nil {
		return Host{}, false
	}
	h, ok := c.Hosts[alias]
	return h, ok
}
