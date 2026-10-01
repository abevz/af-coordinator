package watch

import (
	"fmt"
	"strings"
	"time"

	"github.com/abevz/dibs/internal/core"
	"github.com/mattn/go-runewidth"
)

func summaryHeading() string {
	return fmt.Sprintf("%-18s %5s %7s %11s %8s  %s", "PROJECT", "READY", "BLOCKED", "IN_PROGRESS", "DEFERRED", "LAST EVENT")
}
func summaryRow(row ProjectSummary) string {
	last := row.LastEventAt
	if last == "" {
		last = "unknown"
	}
	return fmt.Sprintf("%-18s %5d %7d %11d %8d  %s", row.Key, row.Ready, row.Blocked, row.InProgress, row.Deferred, last)
}
func age(since string, now time.Time) string {
	at, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return "unknown"
	}
	elapsed := now.Sub(at)
	if elapsed < 0 {
		elapsed = 0
	}
	return elapsed.Round(time.Second).String()
}
func staleRow(row StaleIssue, now time.Time) string {
	return fmt.Sprintf("%s  %s %s  %s", row.Issue.ShortID, age(row.Since, now), row.Reason, row.Issue.Title)
}

// RenderOnce emits every row, without terminal padding or clipping.
func RenderOnce(snapshot Snapshot, now time.Time) string {
	snapshot = snapshot.At(now)
	project := snapshot.Project
	if project == "" {
		project = "all projects"
	}
	lines := []string{"dibs watch · " + project}
	if snapshot.Project == "" {
		lines = append(lines, "", "PROJECT SUMMARY", summaryHeading())
		for _, row := range snapshot.Projects {
			lines = append(lines, summaryRow(row))
		}
	}
	lines = append(lines, "", fmt.Sprintf("STALE (%d)", len(snapshot.Stale)))
	for _, row := range snapshot.Stale {
		lines = append(lines, "  "+staleRow(row, now))
	}
	lines = append(lines, "", fmt.Sprintf("READY (%d)", len(snapshot.Ready)))
	for _, issue := range snapshot.Ready {
		lines = append(lines, fmt.Sprintf("  %s %s", issue.ShortID, issue.Title))
	}
	lines = append(lines, "", fmt.Sprintf("ACTIVE LEASES (%d) · PID self-reported", len(snapshot.Active)))
	for _, issue := range snapshot.Active {
		process := "PID ?"
		if issue.LeasePID > 0 && issue.LeaseHost != "" {
			process = fmt.Sprintf("PID %d@%s", issue.LeasePID, issue.LeaseHost)
		}
		lines = append(lines, fmt.Sprintf("  %s %s %s %s %s", issue.ShortID, issue.Holder, remaining(issue.LeaseExpiresAt, now), process, issue.Title))
	}
	lines = append(lines, "", fmt.Sprintf("BLOCKED (%d)", len(snapshot.Blocked)))
	for _, row := range snapshot.Blocked {
		lines = append(lines, fmt.Sprintf("  %s by %s %s", row.Issue.ShortID, strings.Join(row.BlockedBy, ","), row.Issue.Title))
	}
	lines = append(lines, "", fmt.Sprintf("RECENT EVENTS (%d)", len(snapshot.Events)))
	for i := len(snapshot.Events) - 1; i >= 0; i-- {
		event := snapshot.Events[i]
		lines = append(lines, fmt.Sprintf("  %s %s %s %s", event.CreatedAt, snapshot.IssueNames[event.IssueID], event.EventType, event.Actor))
	}
	return strings.Join(lines, "\n")
}

// RenderNavigation uses a scrollable project table or project issue list.
func RenderNavigation(snapshot Snapshot, refreshErr error, now time.Time, width, height, selected int) string {
	if width < 38 || height < 12 {
		return Render(snapshot, refreshErr, now, width, height)
	}
	header := strings.Split(Render(snapshot, refreshErr, now, width, height), "\n")[:2]
	lines := append(header, "")
	var rows []string
	if snapshot.Project == "" {
		lines = append(lines, "PROJECT SUMMARY", clip(summaryHeading(), width))
		for _, row := range snapshot.Projects {
			rows = append(rows, summaryRow(row))
		}
	} else {
		snapshot = snapshot.At(now)
		lines = append(lines, clip(fmt.Sprintf("ISSUES (%d) · READY %d · ACTIVE %d · BLOCKED %d · STALE %d", len(snapshot.Issues), len(snapshot.Ready), len(snapshot.Active), len(snapshot.Blocked), len(snapshot.Stale)), width))
		for _, issue := range snapshot.Issues {
			rows = append(rows, issueRow(snapshot, issue, now))
		}
	}
	budget := height - len(lines) - 2
	if snapshot.Project == "" && len(snapshot.Stale) > 0 {
		budget -= 3
	}
	if budget < 1 {
		budget = 1
	}
	start := 0
	if selected >= budget {
		start = selected - budget + 1
	}
	for i := start; i < len(rows) && i < start+budget; i++ {
		marker := "  "
		if i == selected {
			marker = "> "
		}
		lines = append(lines, clip(marker+rows[i], width))
	}
	if len(rows) == 0 {
		lines = append(lines, "  ·")
	}
	if snapshot.Project == "" && len(snapshot.Stale) > 0 {
		lines = append(lines, clip(fmt.Sprintf("STALE (%d) · select a project to inspect", len(snapshot.Stale)), width), clip("  "+staleRow(snapshot.Stale[0], now), width))
	}
	footer := "↑/↓ select · enter open · esc back · q quit · r refresh · no writes"
	return frame(lines, footer, width, height)
}

func issueRow(snapshot Snapshot, issue core.Issue, now time.Time) string {
	lane := strings.ToUpper(issue.Status)
	for _, row := range snapshot.Ready {
		if row.ID == issue.ID {
			lane = "READY"
		}
	}
	for _, row := range snapshot.Blocked {
		if row.Issue.ID == issue.ID {
			lane = "BLOCKED"
		}
	}
	for _, row := range snapshot.Active {
		if row.ID == issue.ID {
			lane = "ACTIVE " + remaining(row.LeaseExpiresAt, now)
		}
	}
	if stale, ok := staleIssue(issue, now); ok {
		lane = "STALE " + age(stale.Since, now) + " " + stale.Reason
	}
	return fmt.Sprintf("%-12s %-20s %s", issue.ShortID, lane, issue.Title)
}

func RenderDetail(detail Detail, detailErr error, loading bool, width, height, offset int) string {
	if width <= 0 {
		width = 100
	}
	if height < 5 {
		height = 5
	}
	lines := []string{clip("dibs watch · "+detail.Issue.ShortID+" · "+detail.Issue.Title, width)}
	var body []string
	if loading {
		body = append(body, "Loading details…")
	}
	if detailErr != nil {
		body = append(body, "ERROR · "+detailErr.Error())
	}
	body = append(body, "Status: "+detail.Issue.Status, "", "DESCRIPTION", detail.Issue.Description, "", "DEPENDENCIES")
	for _, dep := range detail.Issue.Dependencies {
		body = append(body, fmt.Sprintf("%s %s", dep.Kind, dep.DependsOnShortID))
	}
	body = append(body, "", "LAST NOTES")
	for _, note := range detail.Notes {
		body = append(body, note.CreatedAt+" · "+note.Author, note.Body, "")
	}
	var wrapped []string
	for _, chunk := range body {
		for _, line := range strings.Split(chunk, "\n") {
			wrapped = append(wrapped, wrap(line, width)...)
		}
	}
	maxOffset := len(wrapped) - (height - 2)
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	for i := offset; i < len(wrapped) && len(lines) < height-1; i++ {
		lines = append(lines, wrapped[i])
	}
	return frame(lines, "↑/↓ scroll · esc back · q quit · r refresh · no writes", width, height)
}
func wrap(line string, width int) []string {
	if width < 1 {
		width = 1
	}
	var rows []string
	for runewidth.StringWidth(line) > width {
		chunk := runewidth.Truncate(line, width, "")
		if chunk == "" {
			chunk = string([]rune(line)[0])
		}
		rows = append(rows, chunk)
		line = strings.TrimPrefix(line, chunk)
	}
	return append(rows, line)
}
func frame(lines []string, footer string, width, height int) string {
	if len(lines) > height-1 {
		lines = lines[:height-1]
	}
	for len(lines) < height-1 {
		lines = append(lines, "")
	}
	lines = append(lines, clip(footer, width))
	return strings.Join(lines, "\n")
}
