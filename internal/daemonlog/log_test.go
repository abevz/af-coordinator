package daemonlog

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPathUsesStateDirectoryAndIsolatesSockets(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", root)
	a, b := Path("/run/a.sock"), Path("/run/b.sock")
	if filepath.Dir(a) != filepath.Join(root, "dibs", "logs") || a == b {
		t.Fatalf("paths: %s, %s", a, b)
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", root)
	if !strings.HasPrefix(Path("/run/a.sock"), filepath.Join(root, ".local", "state", "dibs", "logs")+string(os.PathSeparator)) {
		t.Fatal("wrong HOME fallback")
	}
}

func TestRotationRetainsLatestAndBoundsAllWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "dibsd.log")
	w, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, data := range [][]byte{bytes.Repeat([]byte("a"), MaxBytes), []byte("new record\n"), bytes.Repeat([]byte("b"), MaxBytes*3)} {
		n, err := w.Write(data)
		if err != nil || n != len(data) {
			t.Fatalf("write = %d, %v", n, err)
		}
	}
	for _, suffix := range []string{"", ".1"} {
		info, err := os.Stat(path + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > MaxBytes || info.Mode().Perm() != 0600 {
			t.Fatalf("%s: %d bytes %v", suffix, info.Size(), info.Mode())
		}
	}
	data, err := os.ReadFile(path + ".1")
	if err != nil || string(data) != "new record\n" {
		t.Fatalf("previous log: %q, %v", data, err)
	}
	data, _ = os.ReadFile(path)
	if !bytes.Equal(data, bytes.Repeat([]byte("b"), MaxBytes)) {
		t.Fatal("oversized write not bounded to tail")
	}
	info, _ := os.Stat(filepath.Dir(path))
	if info.Mode().Perm() != 0700 {
		t.Fatal("log directory not private")
	}
}

func TestExclusiveOwnershipAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dibsd.log")
	w, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Open(path); err == nil {
		other.Close()
		t.Fatal("concurrent writer accepted")
	}
	if _, err := w.Write([]byte("first\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("closed")); err == nil {
		t.Fatal("write after close accepted")
	}
	w, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.Write([]byte("second\n"))
	b, _ := os.ReadFile(path)
	if string(b) != "first\nsecond\n" {
		t.Fatalf("reopen: %q", b)
	}
}

func TestConcurrentWritesStayBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dibsd.log")
	w, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				if _, err := w.Write(bytes.Repeat([]byte("x"), 8192)); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	for _, suffix := range []string{"", ".1"} {
		info, err := os.Stat(path + suffix)
		if err != nil || info.Size() > MaxBytes {
			t.Fatalf("bound: %v, %v", info, err)
		}
	}
}

func TestAdoptsOversizedExistingLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dibsd.log")
	os.WriteFile(path, bytes.Repeat([]byte("x"), MaxBytes*2), 0644)
	w, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, suffix := range []string{"", ".1"} {
		info, err := os.Stat(path + suffix)
		if err != nil || info.Size() > MaxBytes || info.Mode().Perm() != 0600 {
			t.Fatalf("adoption: %v, %v", info, err)
		}
	}
}
