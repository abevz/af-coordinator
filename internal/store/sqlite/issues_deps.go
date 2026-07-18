package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/abevz/af-coordinator/internal/core"
)

func populateDependencies(ctx context.Context, db *sql.DB, issues []core.Issue) ([]core.Issue, error) {
	if len(issues) == 0 {
		return issues, nil
	}
	for idx := range issues {
		issues[idx].Blocked = issues[idx].Status == "blocked"
	}

	ids := make([]string, len(issues))
	idMap := make(map[string]int)
	for i, issue := range issues {
		ids[i] = issue.ID
		idMap[issue.ID] = i
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT source.id, source.short_id, target.id, target.short_id, target.status, d.kind
		FROM dependencies d
		JOIN issues source ON source.id = d.issue_id
		JOIN issues target ON target.id = d.depends_on_issue_id
		WHERE d.issue_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dependencies: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var issueID, issueShortID, dependsOnID, dependsOnShortID, dependsOnStatus, kind string
		if err := rows.Scan(&issueID, &issueShortID, &dependsOnID, &dependsOnShortID, &dependsOnStatus, &kind); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		if idx, ok := idMap[issueID]; ok {
			if kind == "blocks" && !isTerminalIssueStatus(dependsOnStatus) {
				issues[idx].Blocked = true
				if !containsShortID(issues[idx].BlockedBy, dependsOnShortID) {
					issues[idx].BlockedBy = append(issues[idx].BlockedBy, dependsOnShortID)
				}
			}
			issues[idx].Dependencies = append(issues[idx].Dependencies, core.Dependency{
				IssueID:          issueID,
				IssueShortID:     issueShortID,
				DependsOnID:      dependsOnID,
				DependsOnShortID: dependsOnShortID,
				Kind:             kind,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependencies: %w", err)
	}

	return issues, nil
}

func isTerminalIssueStatus(status string) bool {
	return status == "done" || status == "cancelled"
}

func containsShortID(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
