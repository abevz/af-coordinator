package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildInstallCanonicalBinariesAndSafeAliasRemoval(t *testing.T) {
	bindir := t.TempDir()
	build := func() {
		cmd := exec.Command("make", "build-install", "BINDIR="+bindir)
		cmd.Dir = filepath.Join("..", "..")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build-install: %v\n%s", err, out)
		}
	}
	build()
	for _, name := range []string{"dibs", "dibsd", "dibs-mcp"} {
		info, err := os.Stat(filepath.Join(bindir, name))
		if err != nil || info.Mode()&0111 == 0 {
			t.Fatalf("missing executable %s", name)
		}
	}
	for _, name := range []string{"afctl", "af-coordinatord", "afc-mcp"} {
		if _, err := os.Lstat(filepath.Join(bindir, name)); !os.IsNotExist(err) {
			t.Fatalf("clean install created alias %s", name)
		}
	}
	if err := os.Symlink("dibs", filepath.Join(bindir, "afctl")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(bindir, "dibsd"), filepath.Join(bindir, "af-coordinatord")); err != nil {
		t.Fatal(err)
	}
	unrelated := []byte("independent executable\n")
	if err := os.WriteFile(filepath.Join(bindir, "afc-mcp"), unrelated, 0755); err != nil {
		t.Fatal(err)
	}
	build()
	for _, name := range []string{"afctl", "af-coordinatord"} {
		if _, err := os.Lstat(filepath.Join(bindir, name)); !os.IsNotExist(err) {
			t.Fatalf("owned alias remains %s", name)
		}
	}
	data, err := os.ReadFile(filepath.Join(bindir, "afc-mcp"))
	if err != nil || string(data) != string(unrelated) {
		t.Fatal("upgrade altered unrelated executable")
	}
}
