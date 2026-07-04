package core

import (
	"fmt"
	"regexp"
	"strings"
)

var validProjectKey = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// Project represents a logical top-level initiative.
type Project struct {
	ID           string `json:"id"`
	Key          string `json:"key"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	NextIssueSeq int64  `json:"next_issue_seq"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ValidateCreateProject checks required fields for a new project.
func ValidateCreateProject(key, name string) error {
	var errs []string
	if key == "" {
		errs = append(errs, "key is required")
	} else if len(key) > 16 {
		errs = append(errs, "key must start with a letter, contain only lowercase letters and digits (no leading/trailing/double hyphens), max 16 chars — key becomes the issue prefix (<key>-<n>); keep it short")
	} else if !validProjectKey.MatchString(key) {
		errs = append(errs, "key must start with a letter, contain only lowercase letters and digits (no leading/trailing/double hyphens), max 16 chars — key becomes the issue prefix (<key>-<n>); keep it short")
	}
	if name == "" {
		errs = append(errs, "name is required")
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation_failed: %s", strings.Join(errs, "; "))
	}
	return nil
}
