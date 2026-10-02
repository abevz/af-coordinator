package firstuse

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/abevz/dibs/internal/api"
	"github.com/abevz/dibs/internal/client"
	"github.com/abevz/dibs/internal/config"
	"github.com/abevz/dibs/internal/core"
	"github.com/abevz/dibs/internal/daemonlog"
)

func TestEnsureDaemonRejectsLongSocketBeforeStarting(t *testing.T) {
	path := "/" + strings.Repeat("x", len(syscall.RawSockaddrUnix{}.Path))
	cfg := config.Config{SocketPath: path, DBPath: filepath.Join(t.TempDir(), "db")}
	err := EnsureDaemon(context.Background(), cfg, "/bin/false")
	if err == nil || !strings.Contains(err.Error(), "DIBS_SOCKET") {
		t.Fatalf("EnsureDaemon error = %v, want path-length hint", err)
	}
	if _, statErr := os.Stat(path + ".startup.log"); !os.IsNotExist(statErr) {
		t.Fatalf("startup log created before validation: %v", statErr)
	}
}

func TestEnsureDaemonUsesHealthyMatchingSocket(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(dir, "data.db"), SocketPath: filepath.Join(dir, "daemon.sock")}
	serverDB := cfg.DBPath
	listener, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "db_path": serverDB})
	})
	server := &http.Server{Handler: mux}
	defer server.Close()
	go server.Serve(listener)
	if err := EnsureDaemon(context.Background(), cfg, ""); err != nil {
		t.Fatal(err)
	}
	cfg.DBPath = filepath.Join(dir, "other.db")
	if err := EnsureDaemon(context.Background(), cfg, ""); err == nil || !strings.Contains(err.Error(), "another database") {
		t.Fatalf("mismatched database error = %v", err)
	}
}

func TestStopDaemonWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(dir, "data.db"), SocketPath: filepath.Join(dir, "absent.sock")}
	if err := StopDaemon(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.SocketPath + ".pid"); !os.IsNotExist(err) {
		t.Fatalf("unexpected pid file: %v", err)
	}
}

func TestStopDaemonDoesNotMistakeUnhealthySocketForStopped(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(dir, "data.db"), SocketPath: filepath.Join(dir, "daemon.sock")}
	listener, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	})}
	defer server.Close()
	go server.Serve(listener)
	if err := StopDaemon(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "health is unavailable") {
		t.Fatalf("stop on unhealthy socket = %v", err)
	}
}

func TestEnsureDaemonRefusesUnverifiableReachableSocket(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(dir, "data.db"), SocketPath: filepath.Join(dir, "daemon.sock")}
	listener, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	})}
	defer server.Close()
	go server.Serve(listener)
	if err := EnsureDaemon(context.Background(), cfg, ""); err == nil || !strings.Contains(err.Error(), "health cannot be verified") {
		t.Fatalf("unverifiable daemon accepted: %v", err)
	}
}

func TestWaitForStoppedWaitsForDatabaseLock(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(dir, "data.db"), SocketPath: filepath.Join(dir, "daemon.sock")}
	lock, err := api.AcquireDatabaseLock(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- waitForStoppedOwned(context.Background(), cfg, "") }()
	select {
	case err := <-done:
		t.Fatalf("returned while database lock held: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not return after database lock released")
	}
}

func TestDegradedMatchingDaemonCanBeStopped(t *testing.T) {
	cfg := config.Config{DBPath: "/tmp/dibs-test.db", SocketPath: "/tmp/dibs-test.sock"}
	if err := checkStopHealth(core.Health{Status: "degraded", DBPath: cfg.DBPath}, cfg); err != nil {
		t.Fatal(err)
	}
	if err := checkStopHealth(core.Health{Status: "degraded", DBPath: "/tmp/other.db"}, cfg); err == nil {
		t.Fatal("mismatched database accepted")
	}
}

func TestStartupLogTailReadsOnlyFinalLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.log")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 8192)+"\nlast diagnostic\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := startupLogTail(path); got != "last diagnostic" {
		t.Fatalf("tail = %q", got)
	}
}

func TestVerifiedStopCleansOwnedArtifacts(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DBPath: filepath.Join(dir, "data.db"), SocketPath: filepath.Join(dir, "daemon.sock")}
	for _, suffix := range []string{".pid", ".startup.log"} {
		if err := os.WriteFile(cfg.SocketPath+suffix, []byte("12345\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := waitForStoppedOwned(context.Background(), cfg, "12345"); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{".pid", ".startup.log"} {
		if _, err := os.Stat(cfg.SocketPath + suffix); !os.IsNotExist(err) {
			t.Fatalf("owned artifact remains: %s (%v)", suffix, err)
		}
	}
}

func TestStopArtifactCleanupRequiresOwnership(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "daemon.sock")
	for _, suffix := range []string{".pid", ".startup.log"} {
		if err := os.WriteFile(socket+suffix, []byte("new-owner\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, pid := range []string{"", "previous-owner"} {
		err := cleanupStoppedArtifacts(socket, pid)
		if pid != "" && err == nil {
			t.Fatal("changed pid accepted")
		}
		for _, suffix := range []string{".pid", ".startup.log"} {
			if _, err := os.Stat(socket + suffix); err != nil {
				t.Fatalf("unowned artifact removed: %v", err)
			}
		}
	}
}

// Build the actual companion so this regression fails if cmd/dibsd ever
// routes detached structured logs back to its discarded stderr.
func TestActualAutostartDaemonLogsAndCleanup(t *testing.T) {
	dir := t.TempDir()
	daemon := filepath.Join(dir, "dibsd")
	// The companion is built outside the test binary's import graph. Read its
	// entrypoint explicitly so changes to the wiring invalidate Go's test cache.
	if _, err := os.ReadFile(filepath.Join("..", "..", "cmd", "dibsd", "main.go")); err != nil {
		t.Fatal(err)
	}
	buildCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(buildCtx, "go", "build", "-o", daemon, "github.com/abevz/dibs/cmd/dibsd")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build scratch companion: %v\n%s", err, out)
	}
	t.Setenv("HOME", dir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("DIBS_OPERATOR_TOKEN", "")
	t.Setenv("DIBS_LOG_LEVEL", "info")
	t.Setenv("DIBS_NO_UPDATE_NOTIFIER", "1")
	cfg := config.Config{SocketPath: filepath.Join(dir, "d.sock"), DBPath: filepath.Join(dir, "data.db")}
	t.Setenv("DIBS_DB", cfg.DBPath)
	t.Setenv("DIBS_SOCKET", cfg.SocketPath)
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			if err := StopDaemon(context.Background(), cfg); err != nil {
				t.Errorf("stop scratch companion: %v", err)
			}
		}
	})
	if err := EnsureDaemon(context.Background(), cfg, daemon); err != nil {
		t.Fatal(err)
	}
	path := daemonlog.Path(cfg.SocketPath)
	before, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(before), "daemon started") {
		t.Fatalf("real startup log: %q, %v", before, err)
	}
	if _, err := client.New(cfg.SocketPath).CreateProject(context.Background(), "scratch", "Scratch", ""); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || len(after) <= len(before) || !strings.Contains(string(after), `operation="POST /v1/projects"`) {
		t.Fatalf("real continuing mutation log: %q, %v", after, err)
	}
	if _, err := os.Stat(cfg.SocketPath + ".startup.log"); !os.IsNotExist(err) {
		t.Fatalf("socket-adjacent startup log created: %v", err)
	}
	if err := os.WriteFile(cfg.SocketPath+".startup.log", []byte("legacy startup diagnostic\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := StopDaemon(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	stopped = true
	for _, suffix := range []string{"", ".pid", ".startup.log"} {
		if _, err := os.Stat(cfg.SocketPath + suffix); !os.IsNotExist(err) {
			t.Fatalf("owned artifact remains %s: %v", suffix, err)
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("retained state log: %v", err)
	}
}
