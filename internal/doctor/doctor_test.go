package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abevz/af-coordinator/internal/build"
	"github.com/abevz/af-coordinator/internal/config"
	"github.com/abevz/af-coordinator/internal/core"
)

type mockExec struct {
	cmdOut []byte
	cmdErr error
	env    map[string]string
}

func (m mockExec) Command(name string, arg ...string) ([]byte, error) {
	return m.cmdOut, m.cmdErr
}

func (m mockExec) LookupEnv(key string) (string, bool) {
	v, ok := m.env[key]
	return v, ok
}

func TestEvaluateVersionSkew(t *testing.T) {
	tests := []struct {
		name     string
		h        *core.Health
		expected string
	}{
		{
			name:     "nil health",
			h:        nil,
			expected: "WARN",
		},
		{
			name:     "empty version",
			h:        &core.Health{},
			expected: "WARN",
		},
		{
			name:     "mismatch",
			h:        &core.Health{Version: "old-version"},
			expected: "WARN",
		},
		{
			name:     "match",
			h:        &core.Health{Version: build.Version},
			expected: "ok",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := EvaluateVersionSkew(tc.h)
			if res.Status != tc.expected {
				t.Errorf("expected %s, got %s: %s", tc.expected, res.Status, res.Message)
			}
		})
	}
}

func TestEvaluateConfigMismatch(t *testing.T) {
	cfg := config.Config{
		SocketPath: "/tmp/sock",
		DBPath:     "/tmp/db",
	}

	tests := []struct {
		name     string
		h        *core.Health
		expected string
	}{
		{
			name:     "match",
			h:        &core.Health{SocketPath: "/tmp/sock", DBPath: "/tmp/db"},
			expected: "ok",
		},
		{
			name:     "socket mismatch",
			h:        &core.Health{SocketPath: "/tmp/other", DBPath: "/tmp/db"},
			expected: "WARN",
		},
		{
			name:     "db mismatch",
			h:        &core.Health{SocketPath: "/tmp/sock", DBPath: "/tmp/other"},
			expected: "WARN",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := EvaluateConfigMismatch(tc.h, cfg)
			if res.Status != tc.expected {
				t.Errorf("expected %s, got %s: %s", tc.expected, res.Status, res.Message)
			}
		})
	}
}

func TestEvaluateDuplicates(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	afctl1 := filepath.Join(dir1, "afctl")
	os.WriteFile(afctl1, []byte("dummy1"), 0755)

	afctl2 := filepath.Join(dir2, "afctl")
	os.WriteFile(afctl2, []byte("dummy2"), 0755)

	daemon := filepath.Join(dir1, "af-coordinatord")
	os.WriteFile(daemon, []byte("dummy"), 0644) // not executable

	e := mockExec{
		env: map[string]string{
			"PATH": dir1 + string(os.PathListSeparator) + dir2,
		},
	}

	res := EvaluateDuplicates(e)
	if res.Status != "WARN" {
		t.Errorf("expected WARN, got %s: %s", res.Status, res.Message)
	}
}

func TestEvaluateBackup(t *testing.T) {
	ctx := context.Background()
	e := mockExec{
		cmdOut: []byte("enabled\n"),
	}

	dir1 := t.TempDir()
	res := EvaluateBackup(ctx, e, filepath.Join(dir1, "not-exist"), time.Now())
	if res.Status != "WARN" {
		t.Errorf("expected WARN for missing dir")
	}

	res = EvaluateBackup(ctx, e, dir1, time.Now())
	if res.Status != "WARN" {
		t.Errorf("expected WARN for empty dir")
	}
}
