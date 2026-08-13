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
		SocketPath: expandHome(envOrDefault("AF_COORDINATOR_SOCKET", defaultSocketPath)),
		DBPath:     expandHome(envOrDefault("AF_COORDINATOR_DB", defaultDBPath)),
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

// UsesDefaultSocketPath reports whether the daemon owns the socket directory
// layout and may safely normalize its permissions. Custom path parents remain
// operator-managed because they may intentionally be shared.
func (c Config) UsesDefaultSocketPath() bool {
	return filepath.Clean(c.SocketPath) == filepath.Clean(expandHome(defaultSocketPath))
}

// UsesDefaultDBPath reports whether the daemon owns the database directory
// layout and may safely normalize its permissions. Custom path parents remain
// operator-managed because chmodding an arbitrary configured directory could
// affect unrelated files.
func (c Config) UsesDefaultDBPath() bool {
	return filepath.Clean(c.DBPath) == filepath.Clean(expandHome(defaultDBPath))
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
