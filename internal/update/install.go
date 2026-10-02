package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var binaries = []string{"dibs", "dibsd", "dibs-mcp"}

type Installer struct {
	Bindir       string
	Source       Source
	GOOS, GOARCH string
	BeforeCommit func() error
}

func DefaultBindir() string {
	if p := os.Getenv("BINDIR"); p != "" {
		return p
	}
	exe, e := os.Executable()
	if e == nil {
		dir := filepath.Dir(exe)
		if filepath.Base(filepath.Dir(dir)) == ".dibs-update" {
			return filepath.Dir(filepath.Dir(dir))
		}
		if filepath.Base(exe) == "dibs" {
			return dir
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

func (i Installer) root() string { return filepath.Join(i.Bindir, ".dibs-update") }
func (i Installer) withLock(work func() error) error {
	// os.Executable resolves Linuxbrew's public symlink through /proc/self/exe.
	// Check the resolved installation directory before creating updater state:
	// Cellar binaries are regular files owned by Homebrew, not symlinks here.
	dir := i.Bindir
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "INSTALL_RECEIPT.json")); err == nil {
		return fmt.Errorf("Homebrew-managed installation; use brew upgrade abevz/dibs/dibs")
	}
	for p := filepath.Clean(dir); ; p = filepath.Dir(p) {
		if filepath.Base(p) == "Cellar" {
			return fmt.Errorf("Homebrew-managed installation; use brew upgrade abevz/dibs/dibs")
		}
		if filepath.Dir(p) == p || p == "." {
			break
		}
	}
	return withLock(filepath.Join(i.root(), "lock"), work)
}

func (i Installer) Upgrade(ctx context.Context, r Result) error {
	if !r.Available {
		return fmt.Errorf("no newer release to install")
	}
	return i.withLock(func() error {
		if i.managed() {
			current, err := i.CurrentVersion()
			if err != nil {
				return err
			}
			if cmp, err := Compare(current, r.Installed); err != nil || cmp != 0 {
				return fmt.Errorf("installed version changed since release check; run dibs update again")
			}
		}
		goos, arch := i.GOOS, i.GOARCH
		if goos == "" {
			goos = runtime.GOOS
		}
		if arch == "" {
			arch = runtime.GOARCH
		}
		if (goos != "linux" && goos != "darwin") || (arch != "amd64" && arch != "arm64") {
			return fmt.Errorf("unsupported platform %s/%s", goos, arch)
		}
		name := fmt.Sprintf("dibs_%s_%s.tar.gz", goos, arch)
		asset, e := r.Release.asset(name)
		if e != nil {
			return e
		}
		sums, e := r.Release.asset("checksums.txt")
		if e != nil {
			return e
		}
		manifest, e := i.Source.get(ctx, sums, 1<<20)
		if e != nil {
			return e
		}
		archive, e := i.Source.get(ctx, asset, 512<<20)
		if e != nil {
			return e
		}
		if e = verifyChecksum(archive, manifest, name); e != nil {
			return e
		}
		gen, e := os.MkdirTemp(i.root(), "generation-")
		if e != nil {
			return e
		}
		keep := false
		defer func() {
			if !keep {
				_ = os.RemoveAll(gen)
			}
		}()
		if e = extract(archive, gen); e != nil {
			return e
		}
		if e = atomicWrite(filepath.Join(gen, "version"), []byte(r.Latest), 0600); e != nil {
			return e
		}
		old, e := i.adopt(r.Installed)
		if e != nil {
			return e
		}
		// The rollback target belongs to the new generation. Publishing current
		// commits binaries and their rollback target together; a failed later
		// upgrade cannot overwrite the existing rollback target.
		if e = atomicWrite(filepath.Join(gen, "previous"), []byte(old), 0600); e != nil {
			return e
		}
		if i.BeforeCommit != nil {
			if e = i.BeforeCommit(); e != nil {
				return e
			}
		}
		// A single pointer rename changes resolution of all three public paths.
		if e = i.pointer("current", filepath.Base(gen)); e != nil {
			keep = true
			return e
		}
		keep = true
		return nil
	})
}

func (i Installer) managed() bool {
	for _, n := range binaries {
		link, err := os.Readlink(filepath.Join(i.Bindir, n))
		if err != nil || link != filepath.Join(".dibs-update", "current", n) {
			return false
		}
	}
	return true
}

func verifyChecksum(archive, manifest []byte, name string) error {
	var expected string
	for _, line := range strings.Split(string(manifest), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			if expected != "" {
				return fmt.Errorf("duplicate checksum for %s", name)
			}
			expected = f[0]
		}
	}
	decoded, e := hex.DecodeString(expected)
	if e != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("valid checksum for %s missing", name)
	}
	actual := sha256.Sum256(archive)
	if !bytes.Equal(actual[:], decoded) {
		return fmt.Errorf("checksum mismatch for %s", name)
	}
	return nil
}

func extract(archive []byte, dir string) error {
	gz, e := gzip.NewReader(bytes.NewReader(archive))
	if e != nil {
		return e
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	seen := map[string]bool{}
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		name := h.Name
		if name != "dibs" && name != "dibsd" && name != "dibs-mcp" && name != "LICENSE" {
			return fmt.Errorf("unexpected archive entry %q", name)
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return fmt.Errorf("non-regular archive entry %q", name)
		}
		if seen[name] || h.Size <= 0 || h.Size > 256<<20 {
			return fmt.Errorf("invalid archive entry %q", name)
		}
		seen[name] = true
		b, e := io.ReadAll(io.LimitReader(tr, h.Size+1))
		if e != nil {
			return e
		}
		if int64(len(b)) != h.Size {
			return io.ErrUnexpectedEOF
		}
		mode := os.FileMode(0755)
		if name == "LICENSE" {
			mode = 0644
		}
		if e = atomicWrite(filepath.Join(dir, name), b, mode); e != nil {
			return e
		}
	}
	for _, n := range append(append([]string{}, binaries...), "LICENSE") {
		if !seen[n] {
			return fmt.Errorf("archive missing %s", n)
		}
	}
	return syncDir(dir)
}

func (i Installer) pointer(name, target string) error {
	if filepath.Base(target) != target || !strings.HasPrefix(target, "generation-") {
		return fmt.Errorf("invalid update generation")
	}
	tmp, e := os.CreateTemp(i.root(), ".pointer-")
	if e != nil {
		return e
	}
	p := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(p)
	defer os.Remove(p)
	if e = os.Symlink(target, p); e != nil {
		return e
	}
	if e = os.Rename(p, filepath.Join(i.root(), name)); e != nil {
		return e
	}
	return syncDir(i.root())
}
func (i Installer) generation(pointer string) (string, error) {
	g, e := os.Readlink(filepath.Join(i.root(), pointer))
	if e != nil {
		return "", e
	}
	return g, i.validateGeneration(g)
}

func (i Installer) validateGeneration(g string) error {
	if filepath.Base(g) != g || !strings.HasPrefix(g, "generation-") {
		return fmt.Errorf("invalid update generation")
	}
	for _, n := range binaries {
		info, e := os.Lstat(filepath.Join(i.root(), g, n))
		if e != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("incomplete rollback generation")
		}
	}
	return nil
}

// Adoption keeps the original bytes throughout the conversion: both direct
// files and the new symlinks resolve to the same old binary generation.
func (i Installer) adopt(installed string) (string, error) {
	managed := true
	for _, n := range binaries {
		p := filepath.Join(i.Bindir, n)
		info, e := os.Lstat(p)
		if e != nil {
			return "", e
		}
		link, e := os.Readlink(p)
		want := filepath.Join(".dibs-update", "current", n)
		if info.Mode()&os.ModeSymlink != 0 {
			if e != nil || link != want {
				return "", fmt.Errorf("%s is managed by another installer; use that installation manager", p)
			}
		} else if !info.Mode().IsRegular() {
			return "", fmt.Errorf("%s is not a regular binary", p)
		} else {
			managed = false
		}
	}
	if managed {
		return i.generation("current")
	}
	gen, e := os.MkdirTemp(i.root(), "generation-")
	if e != nil {
		return "", e
	}
	for _, n := range binaries {
		b, e := os.ReadFile(filepath.Join(i.Bindir, n))
		if e != nil {
			return "", e
		}
		if len(b) == 0 {
			return "", fmt.Errorf("existing binary %s empty", n)
		}
		if e = atomicWrite(filepath.Join(gen, n), b, 0755); e != nil {
			return "", e
		}
	}
	// Before the first publication, rollback is an idempotent restoration of
	// this original snapshot, including an interrupted adoption/upgrade.
	if e = atomicWrite(filepath.Join(gen, "version"), []byte(installed), 0600); e != nil {
		return "", e
	}
	if e = atomicWrite(filepath.Join(gen, "previous"), []byte(filepath.Base(gen)), 0600); e != nil {
		return "", e
	}
	if e = i.pointer("current", filepath.Base(gen)); e != nil {
		return "", e
	}
	for _, n := range binaries {
		p := filepath.Join(i.Bindir, n)
		link, e := os.Readlink(p)
		if e == nil && link == filepath.Join(".dibs-update", "current", n) {
			continue
		}
		tmp, e := os.CreateTemp(i.Bindir, ".dibs-link-")
		if e != nil {
			return "", e
		}
		path := tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(path)
		if e = os.Symlink(filepath.Join(".dibs-update", "current", n), path); e != nil {
			return "", e
		}
		if e = os.Rename(path, p); e != nil {
			_ = os.Remove(path)
			return "", e
		}
	}
	return filepath.Base(gen), syncDir(i.Bindir)
}

func (i Installer) Rollback() error {
	return i.withLock(func() error {
		current, e := i.generation("current")
		if e != nil {
			return e
		}
		b, e := os.ReadFile(filepath.Join(i.root(), current, "previous"))
		if e != nil {
			return fmt.Errorf("no previous binaries: %w", e)
		}
		previous := string(b)
		e = i.validateGeneration(previous)
		if e != nil {
			return fmt.Errorf("no complete previous binaries: %w", e)
		}
		for _, n := range binaries {
			link, e := os.Readlink(filepath.Join(i.Bindir, n))
			if e != nil || link != filepath.Join(".dibs-update", "current", n) {
				return fmt.Errorf("installation changed outside dibs update; refusing rollback")
			}
		}
		return i.pointer("current", previous)
	})
}

func (i Installer) CurrentVersion() (string, error) {
	g, e := i.generation("current")
	if e != nil {
		return "", e
	}
	b, e := os.ReadFile(filepath.Join(i.root(), g, "version"))
	if e != nil {
		return "", e
	}
	v := strings.TrimSpace(string(b))
	_, e = parseVersion(v)
	return v, e
}
