package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultSocketPath = "~/.local/state/af-coordinator/af-coordinator.sock"
	defaultDBPath     = "~/.local/share/af-coordinator/af-coordinator.db"
)

type Config struct {
	SocketPath string
	DBPath     string
	LogLevel   string
}

func Default() Config {
	return Config{
		SocketPath: expandHome(defaultSocketPath),
		DBPath:     expandHome(defaultDBPath),
		LogLevel:   envOrDefault("AF_COORDINATOR_LOG_LEVEL", "info"),
	}
}

func (c Config) SlogLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func expandHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == "~" {
		return home
	}

	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
