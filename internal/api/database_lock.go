package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// DatabaseLock is a process-scoped advisory lock for one canonical SQLite
// database path. The lock file is intentionally persistent: deleting it while
// another process holds the inode would allow a second lock domain to appear.
type DatabaseLock struct {
	file *os.File
	path string
}

// AcquireDatabaseLock prevents two daemon processes from coordinating through
// the same SQLite database. The caller must keep the returned lock alive until
// after the listener and database have closed.
func AcquireDatabaseLock(dbPath string) (*DatabaseLock, error) {
	canonical, err := canonicalDatabasePath(dbPath)
	if err != nil {
		return nil, err
	}
	lockPath := canonical + ".lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open database lock %s: %w", lockPath, err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("chmod database lock %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		owner := lockOwner(file)
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			if owner != "" {
				return nil, fmt.Errorf("database is already owned by another daemon (pid %s): %s", owner, canonical)
			}
			return nil, fmt.Errorf("database is already owned by another daemon: %s", canonical)
		}
		return nil, fmt.Errorf("lock database %s: %w", canonical, err)
	}
	if err := file.Truncate(0); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, fmt.Errorf("truncate database lock %s: %w", lockPath, err)
	}
	if _, err := file.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, fmt.Errorf("record database lock owner %s: %w", lockPath, err)
	}
	return &DatabaseLock{file: file, path: lockPath}, nil
}

func (l *DatabaseLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	closeErr := file.Close()
	if unlockErr != nil {
		return fmt.Errorf("unlock database %s: %w", l.path, unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close database lock %s: %w", l.path, closeErr)
	}
	return nil
}

func canonicalDatabasePath(dbPath string) (string, error) {
	if strings.TrimSpace(dbPath) == "" || dbPath == ":memory:" {
		return "", fmt.Errorf("a file-backed sqlite database path is required for daemon locking")
	}
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return canonical, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve database path: %w", err)
	}

	realDir, err := filepath.EvalSymlinks(filepath.Dir(absPath))
	if err != nil {
		return "", fmt.Errorf("resolve database directory: %w", err)
	}
	candidate := filepath.Join(realDir, filepath.Base(absPath))
	info, err := os.Lstat(candidate)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(candidate)
		if err != nil {
			return "", fmt.Errorf("read database symlink: %w", err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(realDir, target)
		}
		return canonicalDatabasePath(target)
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect database path: %w", err)
	}
	return candidate, nil
}

func lockOwner(file *os.File) string {
	data := make([]byte, 32)
	count, err := file.ReadAt(data, 0)
	if err != nil && count == 0 {
		return ""
	}
	return strings.TrimSpace(string(data[:count]))
}
