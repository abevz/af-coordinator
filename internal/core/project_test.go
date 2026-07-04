package core

import "testing"

func TestValidProjectKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid short", "key", false},
		{"valid with hyphen", "my-project", false},
		{"too long", "this-is-a-51-char-project-key-that-would-ruin-ids", true},
		{"trailing hyphen", "saa-", true},
		{"double hyphen", "bad--project", true},
		{"starts with digit", "1project", true},
		{"uppercase", "Project", true},
		{"max valid", "abcdefgh12345678", false},
		{"max invalid", "abcdefgh123456789", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCreateProject(tt.key, "dummy")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreateProject(%q) error = %v, wantErr = %v", tt.key, err, tt.wantErr)
			}
		})
	}
}
