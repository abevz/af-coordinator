// Package daemonlog retains bounded structured diagnostics for detached dibsd.
package daemonlog

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

const MaxBytes = 1 << 20

// Path isolates logs for each configured socket, away from runtime tmpfs.
func Path(socket string) string {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(root, "dibs", "logs", fmt.Sprintf("dibsd-%x.log", sha256.Sum256([]byte(filepath.Clean(socket)))))
}

// Prepare checks the destination before launching a detached process.
func Prepare(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	err = f.Chmod(0600)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

type Writer struct {
	mu         sync.Mutex
	path       string
	file, lock *os.File
	size       int64
}

func Open(path string) (*Writer, error) {
	if err := Prepare(path); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("daemon log already owned: %w", err)
	}
	w := &Writer{path: path, lock: lock}
	if info, statErr := os.Stat(path + ".1"); statErr == nil {
		err = os.Chmod(path+".1", 0600)
		if err == nil && info.Size() > MaxBytes {
			err = os.Truncate(path+".1", MaxBytes)
		}
	} else if !os.IsNotExist(statErr) {
		err = statErr
	}
	if err != nil {
		w.Close()
		return nil, err
	}
	if err = w.open(); err == nil && w.size > MaxBytes {
		err = w.rotate()
	}
	if err != nil {
		w.Close()
		return nil, err
	}
	return w, nil
}
func (w *Writer) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.file = f
	w.size = info.Size()
	return nil
}
func (w *Writer) rotate() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		w.file = nil
	}
	if err := os.Rename(w.path, w.path+".1"); err != nil {
		return err
	}
	// Bound an old unbounded file when adopting it.
	info, err := os.Stat(w.path + ".1")
	if err != nil {
		return err
	}
	if info.Size() > MaxBytes {
		if err := os.Truncate(w.path+".1", MaxBytes); err != nil {
			return err
		}
	}
	return w.open()
}
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.lock == nil {
		return 0, os.ErrClosed
	}
	n := len(p)
	if n > MaxBytes {
		p = p[n-MaxBytes:]
	}
	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.size+int64(len(p)) > MaxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	written, err := w.file.Write(p)
	w.size += int64(written)
	if err != nil {
		return 0, err
	}
	if written != len(p) {
		return 0, io.ErrShortWrite
	}
	return n, nil
}
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var err error
	if w.file != nil {
		err = w.file.Close()
		w.file = nil
	}
	if w.lock != nil {
		closeErr := w.lock.Close()
		w.lock = nil
		if err == nil {
			err = closeErr
		}
	}
	return err
}
