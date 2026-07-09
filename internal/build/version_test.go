package build

import "testing"

func TestVersionCanBeOverriddenByLdflags(t *testing.T) {
	original := Version
	t.Cleanup(func() {
		Version = original
	})

	Version = "test-version"
	if Version != "test-version" {
		t.Fatalf("expected Version to be mutable for release ldflags")
	}
}
