package config

import (
	"path/filepath"
	"testing"
)

func TestConfigRecognizesOnlyOwnedDefaultRuntimePaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AF_COORDINATOR_SOCKET", "")
	t.Setenv("AF_COORDINATOR_DB", "")

	defaults := Default()
	if !defaults.UsesDefaultSocketPath() || !defaults.UsesDefaultDBPath() {
		t.Fatalf("default paths not recognized as daemon-owned: %+v", defaults)
	}

	custom := Config{
		SocketPath: filepath.Join(t.TempDir(), "custom.sock"),
		DBPath:     filepath.Join(t.TempDir(), "custom.db"),
	}
	if custom.UsesDefaultSocketPath() || custom.UsesDefaultDBPath() {
		t.Fatalf("custom paths recognized as daemon-owned: %+v", custom)
	}
}
