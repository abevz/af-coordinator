package doctor

import (
	"github.com/abevz/dibs/internal/update"
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluateUpdateOffline(t *testing.T) {
	c := update.Cache{Path: filepath.Join(t.TempDir(), "update.json")}
	if r := EvaluateUpdate(c, "v1.0.0"); r.Status != "ok" {
		t.Fatal(r)
	}
	b := []byte(`{"entries":{"v1.0.0/stable":{"result":{"installed":"v1.0.0","latest":"v1.1.0","available":true}}}}`)
	if err := os.WriteFile(c.Path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if r := EvaluateUpdate(c, "v1.0.0"); r.Status != "WARN" || r.Message != "update available: v1.1.0" {
		t.Fatal(r)
	}
	if r := EvaluateUpdate(c, "v1.2.0"); r.Status != "ok" {
		t.Fatal(r)
	}
}
