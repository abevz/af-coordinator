package firstuse

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/abevz/dibs/internal/client"
	"github.com/abevz/dibs/internal/config"
	"github.com/abevz/dibs/internal/daemonlog"
)

// EnsureDaemon starts the installed companion binary when the configured socket
// is unavailable. dibsd's database lock decides which concurrent starter wins.
func EnsureDaemon(ctx context.Context, cfg config.Config, daemonPath string) error {
	if err := config.ValidateSocketPath(cfg.SocketPath); err != nil {
		return err
	}
	c := client.New(cfg.SocketPath)
	if h, err := c.Health(ctx); err == nil {
		if h.Status != "ok" || filepath.Clean(h.DBPath) != filepath.Clean(cfg.DBPath) {
			return fmt.Errorf("daemon at %s is unhealthy or uses another database; run dibs doctor", cfg.SocketPath)
		}
		return nil
	} else {
		// A reachable socket must not be trusted when its database cannot be
		// verified. This also avoids starting a second daemon over that socket.
		if conn, dialErr := net.DialTimeout("unix", cfg.SocketPath, 200*time.Millisecond); dialErr == nil {
			_ = conn.Close()
			return fmt.Errorf("daemon at %s accepts connections but health cannot be verified: %w", cfg.SocketPath, err)
		}
	}
	if daemonPath == "" {
		return fmt.Errorf("dibsd binary is missing; reinstall dibs or put dibsd next to dibs")
	}
	logPath := daemonlog.Path(cfg.SocketPath)
	if err := daemonlog.Prepare(logPath); err != nil {
		return fmt.Errorf("prepare daemon log %s: %w", logPath, err)
	}
	cmd := exec.Command(daemonPath)
	cmd.Env = append(os.Environ(), "DIBS_INTERNAL_AUTOSTART=1", "DIBS_DB="+cfg.DBPath, "DIBS_SOCKET="+cfg.SocketPath)
	// Nil streams are /dev/null: detached output must not keep a CLI-owned file alive.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start dibsd: %w", err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	var exitErr error
	for {
		if h, err := c.Health(ctx); err == nil && h.Status == "ok" {
			if filepath.Clean(h.DBPath) != filepath.Clean(cfg.DBPath) {
				return fmt.Errorf("daemon at %s uses another database; run dibs doctor", cfg.SocketPath)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-exited:
			exitErr = err
			// Another concurrent starter may have acquired the singleton lock.
			// Continue briefly for its socket to become healthy.
			exited = nil
		case <-timer.C:
			message := startupLogTail(logPath)
			if exitErr != nil {
				return fmt.Errorf("dibsd failed to start: %v; %s (log: %s)", exitErr, message, logPath)
			}
			return fmt.Errorf("timed out waiting for dibsd at %s; %s (log: %s)", cfg.SocketPath, message, logPath)
		case <-tick.C:
		}
	}
}

func startupLogTail(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "inspect the daemon log"
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "inspect the daemon log"
	}
	offset := info.Size() - 4096
	if offset < 0 {
		offset = 0
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return "inspect the daemon log"
	}
	data, err := io.ReadAll(io.LimitReader(f, 4096))
	if err != nil {
		return "inspect the daemon log"
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		return "inspect the daemon log"
	}
	last := lines[len(lines)-1]
	if len(last) > 500 {
		last = last[len(last)-500:]
	}
	return last
}

func FindDaemon() string {
	self, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(self), "dibsd")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate
		}
	}
	return ""
}
