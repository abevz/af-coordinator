// Package testsocket hands tests a unix socket path that fits sun_path.
//
// A unix socket path is limited to 107 usable bytes. t.TempDir() embeds the
// test's own name, and under a sandboxed TMPDIR (a long nested worktree tmp
// directory) that alone can push the path over the limit before the socket
// file name is even added: net.Listen then fails with "bind: invalid
// argument", a failure about path length that reads like a failure about
// sockets (afc-104, afc-105, afc-117).
//
// Sockets from this package go directly under the process temp directory
// with a short unique name, so the test's own name never enters the path.
package testsocket

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// maxUnixPath is the usable length of sun_path: 108 bytes including the
// terminating NUL.
const maxUnixPath = 107

var counter atomic.Int64

// Path returns a unique unix socket path inside the process temp directory
// and registers its removal. It fails the test if the path would exceed
// sun_path, because a loud failure naming the real limit is worth more than
// the kernel's "invalid argument".
func Path(tb testing.TB) string {
	tb.Helper()
	return PathNamed(tb, "s")
}

// PathNamed is Path with a caller-chosen prefix, for tests that assert on
// the socket's name. Keep the prefix short: it is part of the length budget.
func PathNamed(tb testing.TB, prefix string) string {
	tb.Helper()

	name := fmt.Sprintf("%s%d-%d.sock", prefix, os.Getpid(), counter.Add(1))
	path := filepath.Join(os.TempDir(), name)
	if len(path) > maxUnixPath {
		tb.Fatalf(
			"unix socket path is %d bytes, over the %d-byte sun_path limit: %s\n"+
				"TMPDIR is too long for socket tests; shorten it rather than moving the socket deeper",
			len(path), maxUnixPath, path,
		)
	}
	tb.Cleanup(func() { _ = os.Remove(path) })
	return path
}

// Dir returns a short-lived directory for a caller that needs to place more
// than one path (e.g. a socket plus a lock file) under a common short root,
// rather than directly under the process temp directory.
func Dir(tb testing.TB) string {
	tb.Helper()
	dir, err := os.MkdirTemp("", "afsock-")
	if err != nil {
		tb.Fatalf("os.MkdirTemp: %v", err)
	}
	tb.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
