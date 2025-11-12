package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config defines the persisted preferences for the desktop UI application.
type Config struct {
	AgentEndpoint string `json:"agentEndpoint"`
	WindowHotkey  string `json:"windowHotkey"`
	Theme         string `json:"theme"`
	PageSize      int    `json:"pageSize"`
}

// Default returns a configuration populated with sensible defaults.
func Default() Config {
	return Config{
		AgentEndpoint: "http://127.0.0.1:7777",
		WindowHotkey:  "cmd+shift+v",
		Theme:         "system",
		PageSize:      50,
	}
}

// DefaultPath resolves the path where UI preferences are stored.
func DefaultPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "clipflow", "ui.json")
}

// LoadFromFile reads configuration from disk.
func LoadFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return applyDefaults(cfg), nil
}

// SaveToFile persists configuration to disk, creating parent directories if required.
func SaveToFile(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadOrDefault attempts to read configuration and falls back to defaults.
func LoadOrDefault(path string) (Config, error) {
	cfg, err := LoadFromFile(path)
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	cfg = Default()
	if err := SaveToFile(path, cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func applyDefaults(cfg Config) Config {
	defaults := Default()
	if cfg.AgentEndpoint == "" {
		cfg.AgentEndpoint = defaults.AgentEndpoint
	}
	if cfg.WindowHotkey == "" {
		cfg.WindowHotkey = defaults.WindowHotkey
	}
	if cfg.Theme == "" {
		cfg.Theme = defaults.Theme
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = defaults.PageSize
	}
	return cfg
}
