package config

import (
	"os"
	"strconv"
	"time"
)

// Config represents the runtime configuration for the agent process.
type Config struct {
	DatabasePath string
	ServerURL    string
	UserID       string
	DeviceID     string
	SyncInterval time.Duration
	IPCAddress   string
}

const (
	envDatabasePath = "CLIPFLOW_AGENT_DB"
	envServerURL    = "CLIPFLOW_SERVER_URL"
	envUserID       = "CLIPFLOW_USER_ID"
	envDeviceID     = "CLIPFLOW_DEVICE_ID"
	envSyncInterval = "CLIPFLOW_SYNC_INTERVAL"
	envIPCAddress   = "CLIPFLOW_IPC_ADDR"
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

func envOrDefault(key, value string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return value
}
