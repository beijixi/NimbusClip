package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Config represents the runtime configuration for the agent process.
type Config struct {
	DatabasePath string        `json:"databasePath"`
	ServerURL    string        `json:"serverUrl"`
	UserID       string        `json:"userId"`
	DeviceID     string        `json:"deviceId"`
	SyncInterval time.Duration `json:"syncInterval"`
	IPCAddress   string        `json:"ipcAddress"`
}

const (
	envDatabasePath = "CLIPFLOW_AGENT_DB"
	envServerURL    = "CLIPFLOW_SERVER_URL"
	envUserID       = "CLIPFLOW_USER_ID"
	envDeviceID     = "CLIPFLOW_DEVICE_ID"
	envSyncInterval = "CLIPFLOW_SYNC_INTERVAL"
	envIPCAddress   = "CLIPFLOW_IPC_ADDR"
	envConfigPath   = "CLIPFLOW_AGENT_CONFIG_FILE"
)

// Load reads the configuration from environment variables.
func Load() Config {
	cfg := Config{
		DatabasePath: envOrDefault(envDatabasePath, "clipflow_agent.db"),
		ServerURL:    envOrDefault(envServerURL, "http://localhost:8080"),
		UserID:       os.Getenv(envUserID),
		DeviceID:     os.Getenv(envDeviceID),
		SyncInterval: 5 * time.Second,
		IPCAddress:   envOrDefault(envIPCAddress, "127.0.0.1:7777"),
	}
	if v := os.Getenv(envSyncInterval); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds > 0 {
			cfg.SyncInterval = time.Duration(seconds) * time.Second
		}
	}
	return cfg
}

// DefaultPath resolves the location of the persisted agent configuration file.
func DefaultPath() string {
	if v := os.Getenv(envConfigPath); v != "" {
		return v
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "clipflow", "agent.json")
}

// LoadFromFile reads a configuration from the given file path.
func LoadFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var fileCfg Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return Config{}, err
	}
	if fileCfg.SyncInterval <= 0 {
		fileCfg.SyncInterval = 5 * time.Second
	}
	if fileCfg.ServerURL == "" {
		fileCfg.ServerURL = "http://localhost:8080"
	}
	if fileCfg.IPCAddress == "" {
		fileCfg.IPCAddress = "127.0.0.1:7777"
	}
	if fileCfg.DatabasePath == "" {
		fileCfg.DatabasePath = "clipflow_agent.db"
	}
	return fileCfg, nil
}

// SaveToFile persists the provided configuration at the given path.
func SaveToFile(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

// LoadOrCreate attempts to load configuration from disk and falls back to environment defaults.
func LoadOrCreate(path string) (Config, error) {
	cfg, err := LoadFromFile(path)
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	cfg = Load()
	if err := SaveToFile(path, cfg); err != nil {
		return cfg, fmt.Errorf("save config: %w", err)
	}
	return cfg, nil
}

// Manager provides safe concurrent access to the mutable configuration state.
type Manager struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

// NewManager creates a configuration manager backed by the specified file path.
func NewManager(path string, cfg Config) *Manager {
	return &Manager{path: path, cfg: cfg}
}

// Get returns the current configuration snapshot.
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Update replaces the configuration, persisting the change before exposing it to readers.
func (m *Manager) Update(cfg Config) error {
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = 5 * time.Second
	}
	if err := SaveToFile(m.path, cfg); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	return nil
}

func envOrDefault(key, value string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return value
}
