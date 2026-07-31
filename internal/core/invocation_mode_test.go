package core

import "testing"

// TestNormalizeInvocationMode pins the conservative default that afc-95 exists
// for. The audit chain previously answered "who" and was silent on "how", and
// the silence read as an answer: a reader took a sequence of worker claims for
// daemon activity that never happened. So an omitted mode must record absence,
// never a guess, and must never be inferred from the process tree.
func TestNormalizeInvocationMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		want    string
		wantErr bool
	}{
		{"omitted defaults to unknown", "", InvocationModeUnknown, false},
		{"interactive is preserved", InvocationModeInteractive, InvocationModeInteractive, false},
		{"scheduled is preserved", InvocationModeScheduled, InvocationModeScheduled, false},
		{"unknown may be stated explicitly", InvocationModeUnknown, InvocationModeUnknown, false},
		{"unrecognized value is rejected", "daemon", "", true},
		{"case is significant", "Interactive", "", true},
		{"whitespace is not trimmed into validity", " interactive", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeInvocationMode(tt.mode)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeInvocationMode(%q) = %q, want an error", tt.mode, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeInvocationMode(%q): %v", tt.mode, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeInvocationMode(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

// TestUnknownIsNotScheduled pins the distinction the issue turns on. "unknown"
// is a statement of absence; reading it as an unattended run is exactly the
// false conclusion that was written into a closing note and a P1 issue.
func TestUnknownIsNotScheduled(t *testing.T) {
	if InvocationModeUnknown == InvocationModeScheduled {
		t.Fatal("unknown must remain distinguishable from scheduled")
	}
	if !ValidInvocationMode(InvocationModeUnknown) {
		t.Fatal("unknown must be a valid recorded value, not an error state")
	}
}

func TestValidInvocationMode(t *testing.T) {
	for _, mode := range InvocationModes {
		if !ValidInvocationMode(mode) {
			t.Errorf("ValidInvocationMode(%q) = false, want true for a listed mode", mode)
		}
	}
	for _, mode := range []string{"", "auto", "manual", "cron", "INTERACTIVE"} {
		if ValidInvocationMode(mode) {
			t.Errorf("ValidInvocationMode(%q) = true, want false", mode)
		}
	}
}
