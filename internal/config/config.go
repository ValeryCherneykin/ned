// Package config loads ned's optional configuration from ~/.ned/config.yml.
//
// The config file is not required — ned works without it.
// When present it provides host aliases, default user/port/identity,
// eliminating the need to type full connection strings every time.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure of ~/.ned/config.yml.
type Config struct {
	// Defaults apply to all hosts unless overridden per-host.
	Defaults Defaults `yaml:"defaults"`
	// Hosts maps short alias names to full connection details.
	Hosts map[string]Host `yaml:"hosts"`
}

// Defaults holds connection parameters applied to every host
// that does not specify its own value.
type Defaults struct {
	User     string `yaml:"user"`
	Port     string `yaml:"port"`
	Identity string `yaml:"identity"`
}

// Host holds the connection parameters for a single named host alias.
type Host struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Port     string `yaml:"port"`
	Identity string `yaml:"identity"`
}

// Load reads ~/.ned/config.yml and returns the parsed Config.
// Returns an empty Config (not an error) when the file does not exist.
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Config{}, nil
	}

	path := filepath.Join(home, ".ned", "config.yml")

	data, err := os.ReadFile(path) //nolint:gosec
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

// ResolveAlias looks up alias in the hosts map and returns the Host and true,
// or an empty Host and false if the alias is not defined.
func (c *Config) ResolveAlias(alias string) (Host, bool) {
	if c.Hosts == nil {
		return Host{}, false
	}

	h, ok := c.Hosts[alias]

	return h, ok
}
