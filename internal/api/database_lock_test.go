package api

import (
	"bufio"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abevz/af-coordinator/internal/testsocket"
)

func TestDatabaseLockExcludesSecondProcess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "coordinator.db")
	cmd := exec.Command(os.Args[0], "-test.run=^TestDatabaseLockHelperProcess$")
	cmd.Env = append(os.Environ(),
		"AF_TEST_DATABASE_LOCK_HELPER=1",
		"AF_TEST_DATABASE_LOCK_PATH="+dbPath,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("helper readiness = %q, %v", line, err)
	}

	if _, err := AcquireDatabaseLock(dbPath); err == nil || !strings.Contains(err.Error(), "already owned") {
		t.Fatalf("second-process lock error = %v, want already owned", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}

	replacement, err := AcquireDatabaseLock(dbPath)
	if err != nil {
		t.Fatalf("lock after helper exit: %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDatabaseLockHelperProcess(t *testing.T) {
	if os.Getenv("AF_TEST_DATABASE_LOCK_HELPER") != "1" {
		return
	}
	lock, err := AcquireDatabaseLock(os.Getenv("AF_TEST_DATABASE_LOCK_PATH"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err := os.Stdout.WriteString("locked\n"); err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestDatabaseLockUsesCanonicalPathAndSurvivesRelease(t *testing.T) {
	realDir := t.TempDir()
	aliasRoot := t.TempDir()
	aliasDir := filepath.Join(aliasRoot, "db-alias")
	if err := os.Symlink(realDir, aliasDir); err != nil {
		t.Fatal(err)
	}
	realDB := filepath.Join(realDir, "coordinator.db")
	aliasDB := filepath.Join(aliasDir, "coordinator.db")

	first, err := AcquireDatabaseLock(realDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if _, err := AcquireDatabaseLock(aliasDB); err == nil || !strings.Contains(err.Error(), "already owned") {
		t.Fatalf("alias lock error = %v, want already owned", err)
	}
	info, err := os.Stat(realDB + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("lock mode = %o, want 600", info.Mode().Perm())
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	replacement, err := AcquireDatabaseLock(aliasDB)
	if err != nil {
		t.Fatalf("replacement lock after release: %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(realDB + ".lock"); err != nil {
		t.Fatalf("persistent lock file missing after release: %v", err)
	}
}

func TestRemoveStaleSocketProbesBeforeUnlink(t *testing.T) {
	socketPath := testsocket.PathNamed(t, "coordinator")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := removeStaleSocket(socketPath); err == nil || !strings.Contains(err.Error(), "already listening") {
		t.Fatalf("live socket probe error = %v, want already listening", err)
	}
	if _, err := os.Stat(socketPath); err != nil {
		t.Fatalf("live socket was removed: %v", err)
	}
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("live listener became unreachable: %v", err)
	}
	_ = conn.Close()

	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := removeStaleSocket(socketPath); err != nil {
		t.Fatalf("remove stale socket: %v", err)
	}
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("stale socket still exists: %v", err)
	}
}
