package doctor

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/abevz/af-coordinator/internal/build"
	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/config"
	"github.com/abevz/af-coordinator/internal/core"
	_ "modernc.org/sqlite"
)

type Result struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok" or "WARN"
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// OSExec is an interface for mocking exec.Command and environment lookup
type OSExec interface {
	Command(name string, arg ...string) ([]byte, error)
	LookupEnv(key string) (string, bool)
}

type realExec struct{}

func (realExec) Command(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	if runtime.GOOS == "linux" && name == "systemctl" && isSystemctlUserCommand(arg) {
		cmd.Env = append(os.Environ(), systemdUserEnv(os.LookupEnv, os.Stat, os.Getuid())...)
	}
	return cmd.CombinedOutput()
}

func (realExec) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func isSystemctlUserCommand(args []string) bool {
	for _, arg := range args {
		if arg == "--user" {
			return true
		}
	}
	return false
}

func systemdUserEnv(lookup func(string) (string, bool), stat func(string) (os.FileInfo, error), uid int) []string {
	var env []string

	runtimeDir, hasRuntimeDir := lookup("XDG_RUNTIME_DIR")
	if !hasRuntimeDir || runtimeDir == "" {
		candidate := fmt.Sprintf("/run/user/%d", uid)
		if info, err := stat(candidate); err == nil && info.IsDir() {
			runtimeDir = candidate
			env = append(env, "XDG_RUNTIME_DIR="+runtimeDir)
		}
	}

	if runtimeDir == "" {
		return env
	}

	if busAddress, hasBusAddress := lookup("DBUS_SESSION_BUS_ADDRESS"); hasBusAddress && busAddress != "" {
		return env
	}

	busPath := filepath.Join(runtimeDir, "bus")
	if info, err := stat(busPath); err == nil && info.Mode()&os.ModeSocket != 0 {
		env = append(env, "DBUS_SESSION_BUS_ADDRESS=unix:path="+busPath)
	}

	return env
}

func EvaluateDaemon(ctx context.Context, c *client.Client) (Result, *core.Health) {
	h, err := c.Health(ctx)
	if err != nil {
		return Result{
			Name:    "Daemon reachable",
			Status:  "WARN",
			Message: "Daemon is unreachable",
			Hint:    "Start or restart af-coordinatord: " + restartDaemonHint(),
		}, nil
	}
	return Result{
		Name:    "Daemon reachable",
		Status:  "ok",
		Message: "Daemon is reachable and responding",
	}, &h
}

func EvaluateVersionSkew(h *core.Health) Result {
	if h == nil || h.Version == "" {
		return Result{
			Name:    "Version skew",
			Status:  "WARN",
			Message: "Daemon version is unknown",
			Hint:    "Restart af-coordinatord",
		}
	}
	if h.Version != build.Version {
		return Result{
			Name:    "Version skew",
			Status:  "WARN",
			Message: fmt.Sprintf("afctl version %s != daemon version %s", build.Version, h.Version),
			Hint:    "Restart af-coordinatord: " + restartDaemonHint(),
		}
	}
	return Result{
		Name:    "Version skew",
		Status:  "ok",
		Message: "Client and daemon versions match",
	}
}

func EvaluateBackup(ctx context.Context, e OSExec, backupDir string, now time.Time) Result {
	return evaluateBackup(ctx, e, backupDir, now, runtime.GOOS, os.Getuid())
}

func evaluateBackup(ctx context.Context, e OSExec, backupDir string, now time.Time, goos string, uid int) Result {
	switch goos {
	case "darwin":
		out, err := e.Command("launchctl", "print", fmt.Sprintf("gui/%d/com.abevz.af-coordinator-backup", uid))
		if err != nil || strings.TrimSpace(string(out)) == "" {
			return Result{
				Name:    "Backup agent",
				Status:  "WARN",
				Message: "Backup LaunchAgent is not loaded",
				Hint:    "Run: make install-backup",
			}
		}
	case "linux":
		out, err := e.Command("systemctl", "--user", "is-enabled", "af-coordinator-backup.timer")
		if err != nil || strings.TrimSpace(string(out)) != "enabled" {
			return Result{
				Name:    "Backup timer",
				Status:  "WARN",
				Message: "Backup timer is not enabled",
				Hint:    "Run: systemctl --user enable --now af-coordinator-backup.timer",
			}
		}
	default:
		return Result{
			Name:    "Backup",
			Status:  "WARN",
			Message: "Automated backup is not supported for this OS",
			Hint:    "Use manual SQLite VACUUM INTO backups",
		}
	}

	return evaluateBackupFiles(ctx, backupDir, now)
}

func evaluateBackupFiles(ctx context.Context, backupDir string, now time.Time) Result {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{
				Name:    "Backup files",
				Status:  "WARN",
				Message: "Backup directory does not exist",
				Hint:    "Wait for the backup job to run or trigger it manually",
			}
		}
		return Result{
			Name:    "Backup files",
			Status:  "WARN",
			Message: fmt.Sprintf("Cannot read backup directory: %v", err),
			Hint:    "Check permissions of " + backupDir,
		}
	}

	var latestBackup string
	var latestModTime time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestModTime) {
			latestModTime = info.ModTime()
			latestBackup = filepath.Join(backupDir, entry.Name())
		}
	}

	if latestBackup == "" {
		return Result{
			Name:    "Backup files",
			Status:  "WARN",
			Message: "No backup databases found",
			Hint:    "Run the backup job manually",
		}
	}

	if now.Sub(latestModTime) > 48*time.Hour {
		return Result{
			Name:    "Backup files",
			Status:  "WARN",
			Message: "Last backup is older than 48 hours",
			Hint:    "Check backup job logs",
		}
	}

	db, err := sql.Open("sqlite", latestBackup)
	if err != nil {
		return Result{
			Name:    "Backup integrity",
			Status:  "WARN",
			Message: fmt.Sprintf("Failed to open latest backup: %v", err),
			Hint:    "Check backup file: " + latestBackup,
		}
	}
	defer db.Close()

	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return Result{
			Name:    "Backup integrity",
			Status:  "WARN",
			Message: fmt.Sprintf("Integrity check query failed: %v", err),
			Hint:    "Check backup file: " + latestBackup,
		}
	}

	if integrity != "ok" {
		return Result{
			Name:    "Backup integrity",
			Status:  "WARN",
			Message: fmt.Sprintf("Integrity check failed: %s", integrity),
			Hint:    "Investigate the corruption in " + latestBackup,
		}
	}

	return Result{
		Name:    "Backup",
		Status:  "ok",
		Message: "Backup automation enabled, recent backup present and intact",
	}
}

func restartDaemonHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "launchctl kickstart -k gui/$(id -u)/com.abevz.af-coordinatord"
	case "linux":
		return "systemctl --user restart af-coordinatord"
	default:
		return "run af-coordinatord in the foreground or restart your local service manager"
	}
}

func EvaluateDuplicates(e OSExec) Result {
	pathEnv, ok := e.LookupEnv("PATH")
	if !ok {
		return Result{Name: "Duplicate binaries", Status: "WARN", Message: "PATH environment variable not set"}
	}
	dirs := filepath.SplitList(pathEnv)

	findDups := func(bin string) []string {
		var found []string
		seenPath := make(map[string]bool)
		for _, dir := range dirs {
			if dir == "" {
				continue
			}
			full := filepath.Join(dir, bin)
			if info, err := os.Stat(full); err == nil && !info.IsDir() {
				if !seenPath[full] {
					if info.Mode()&0111 != 0 { // is executable
						found = append(found, full)
						seenPath[full] = true
					}
				}
			}
		}

		if len(found) <= 1 {
			return nil
		}

		// check if they differ in content
		firstHash := ""
		differ := false
		for _, f := range found {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(data))
			if firstHash == "" {
				firstHash = hash
			} else if hash != firstHash {
				differ = true
				break
			}
		}

		if differ {
			return found
		}
		return nil
	}

	afctlDups := findDups("afctl")
	daemonDups := findDups("af-coordinatord")

	var msgs []string
	if len(afctlDups) > 1 {
		msgs = append(msgs, fmt.Sprintf("Multiple afctl found: %s", strings.Join(afctlDups, ", ")))
	}
	if len(daemonDups) > 1 {
		msgs = append(msgs, fmt.Sprintf("Multiple af-coordinatord found: %s", strings.Join(daemonDups, ", ")))
	}

	if len(msgs) > 0 {
		return Result{
			Name:    "Duplicate binaries",
			Status:  "WARN",
			Message: strings.Join(msgs, "; "),
			Hint:    "Remove stale binaries from PATH, or ensure Makefile targets are correct (make build-install targets ~/.local/bin)",
		}
	}

	return Result{
		Name:    "Duplicate binaries",
		Status:  "ok",
		Message: "No duplicate binaries in PATH",
	}
}

func EvaluateConfigMismatch(h *core.Health, cfg config.Config) Result {
	if h == nil {
		return Result{Name: "Config mismatch", Status: "WARN", Message: "Daemon health data unavailable"}
	}

	if h.SocketPath != cfg.SocketPath {
		return Result{
			Name:    "Socket path mismatch",
			Status:  "WARN",
			Message: fmt.Sprintf("Client socket (%s) != Daemon socket (%s)", cfg.SocketPath, h.SocketPath),
			Hint:    "Check AF_COORDINATOR_SOCKET env var consistency",
		}
	}
	if h.DBPath != cfg.DBPath {
		return Result{
			Name:    "DB path mismatch",
			Status:  "WARN",
			Message: fmt.Sprintf("Client expected DB (%s) != Daemon DB (%s)", cfg.DBPath, h.DBPath),
			Hint:    "Check AF_COORDINATOR_DB env var consistency",
		}
	}

	return Result{
		Name:    "Config match",
		Status:  "ok",
		Message: "Client config matches daemon runtime state",
	}
}

func RunAll(ctx context.Context, c *client.Client, cfg config.Config) []Result {
	var results []Result

	resDaemon, h := EvaluateDaemon(ctx, c)
	results = append(results, resDaemon)
	results = append(results, EvaluateVersionSkew(h))

	e := realExec{}
	home, err := os.UserHomeDir()
	if err == nil {
		backupDir := filepath.Join(home, "backups", "af-coordinator")
		results = append(results, EvaluateBackup(ctx, e, backupDir, time.Now()))
	} else {
		results = append(results, Result{Name: "Backup", Status: "WARN", Message: "Cannot get user home dir"})
	}

	results = append(results, EvaluateDuplicates(e))
	results = append(results, EvaluateConfigMismatch(h, cfg))

	return results
}
