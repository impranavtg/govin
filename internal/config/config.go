// Package config manages a small JSON config file at ~/.govin/config.json.
// It stores the currently active group so users don't need --group on every command.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ActiveGroup string `json:"active_group,omitempty"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".govin", "config.json"), nil
}

func Load() (*Config, error) {
	p, err := configPath()
	if err != nil {
		return &Config{}, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return &Config{}, nil // no config yet is fine
	}
	var c Config
	json.Unmarshal(data, &c)
	return &c, nil
}

func Save(c *Config) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// ActiveGroup returns the active group name, or "" if none set.
func ActiveGroup() string {
	c, _ := Load()
	return c.ActiveGroup
}

// SetActiveGroup persists the active group.
func SetActiveGroup(name string) error {
	c, _ := Load()
	c.ActiveGroup = name
	return Save(c)
}
