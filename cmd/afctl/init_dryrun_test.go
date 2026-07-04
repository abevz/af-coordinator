package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old
	return buf.String()
}

func TestInitDryRunJSONOutput(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "AGENTS.md")
	before := []byte("# Repo\n")
	if err := os.WriteFile(p, before, 0644); err != nil {
		t.Fatal(err)
	}

	// --- dry-run JSON ---
	jsonOutput = true
	dryOut := captureStdout(func() {
		runInit([]string{"--path", p, "--dry-run"})
	})
	jsonOutput = false

	// File must be unchanged after dry-run
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Error("dry-run modified the file")
	}

	// JSON must have dry_run=true
	var dryResp map[string]interface{}
	if err := json.Unmarshal([]byte(dryOut), &dryResp); err != nil {
		t.Fatal(err)
	}
	if dryResp["dry_run"] != true {
		t.Errorf("dry_run=%v, want true", dryResp["dry_run"])
	}
	if dryResp["action"] != "updated" {
		t.Errorf("action=%v, want updated", dryResp["action"])
	}

	// --- real run JSON ---
	jsonOutput = true
	realOut := captureStdout(func() {
		runInit([]string{"--path", p})
	})
	jsonOutput = false

	// File must be modified after real run
	afterReal, _ := os.ReadFile(p)
	if bytes.Equal(before, afterReal) {
		t.Error("real run did NOT modify the file")
	}

	// Real JSON must NOT have dry_run field
	var realResp map[string]interface{}
	if err := json.Unmarshal([]byte(realOut), &realResp); err != nil {
		t.Fatal(err)
	}
	if _, has := realResp["dry_run"]; has {
		t.Error("real output should not have dry_run")
	}
}

func TestInitDryRunTextOutput(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(p, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// Dry-run text output must start with "would"
	dryOut := captureStdout(func() {
		runInit([]string{"--path", p, "--dry-run"})
	})
	out := strings.TrimSpace(dryOut)
	if !strings.HasPrefix(out, "would ") {
		t.Errorf("dry-run text output %q does not start with 'would '", out)
	}
}

func TestInitRealRunTextOutputNoWould(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(p, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// Real-run text output must NOT start with "would"
	realOut := captureStdout(func() {
		runInit([]string{"--path", p})
	})
	out := strings.TrimSpace(realOut)
	if strings.HasPrefix(out, "would ") {
		t.Errorf("real text output %q incorrectly starts with 'would '", out)
	}
	if !strings.HasPrefix(out, "updated") {
		t.Errorf("real text output %q should start with 'updated'", out)
	}
}
