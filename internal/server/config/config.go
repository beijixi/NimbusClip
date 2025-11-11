package config

import (
	"os"
)

// Config represents the runtime configuration for the server.
type Config struct {
	Addr        string
	DatabaseDSN string
}

const (
	envAddr        = "CLIPFLOW_SERVER_ADDR"
	envDatabaseDSN = "CLIPFLOW_SERVER_DSN"
)

// Load reads configuration from environment variables, applying defaults when necessary.
func Load() Config {
	cfg := Config{
		Addr:        getenvDefault(envAddr, ":8080"),
		DatabaseDSN: getenvDefault(envDatabaseDSN, "file:clipflow_server.db?_pragma=busy_timeout(5000)"),
	}
	return cfg
}

func getenvDefault(key, value string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return value
}
