package ghsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/abevz/dibs/internal/client"
	"github.com/abevz/dibs/internal/core"
	"github.com/abevz/dibs/internal/github"
)

type PublishError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *PublishError) Error() string { return e.Code + ": " + e.Message }

type PublishResult struct {
	OK         bool          `json:"ok"`
	Already    bool          `json:"already"`
	CommentURL string        `json:"comment_url"`
	Error      *PublishError `json:"error"`
	ShortID    string        `json:"-"`
}

// Failure keeps the same code/message shape in CLI JSON and MCP tool errors.
func Failure(err error) PublishResult {
	code, message := "publish_failed", err.Error()
	var ghErr *github.Error
	var syncErr *PublishError
	var clientErr *client.ClientError
	switch {
	case errors.As(err, &ghErr):
		code, message = ghErr.Code, ghErr.Message()
	case errors.As(err, &syncErr):
		code, message = syncErr.Code, syncErr.Message
	case errors.As(err, &clientErr):
		code, message = clientErr.Code, clientErr.Message
	}
	return PublishResult{Error: &PublishError{Code: code, Message: message}}
}

func RequireGitHubExternalKey(ctx context.Context, c Coordinator, issueID string) error {
	issue, _, err := c.GetIssue(ctx, issueID)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(issue.ExternalKey, "github:") {
		return fmt.Errorf("issue %s has no GitHub external key; import or link a GitHub issue before --publish", issue.ShortID)
	}
	_, err = github.ParseIssueRef(strings.TrimPrefix(issue.ExternalKey, "github:"))
	if err != nil {
		return fmt.Errorf("invalid GitHub external key: %w", err)
	}
	return nil
}

func Publish(ctx context.Context, c Coordinator, gh github.Client, issueID string) (PublishResult, error) {
	return PublishWithLeaseToken(ctx, c, gh, issueID, "")
}

func PublishWithLeaseToken(ctx context.Context, c Coordinator, gh github.Client, issueID, leaseToken string) (PublishResult, error) {
	issue, _, err := c.GetIssue(ctx, issueID)
	if err != nil {
		return PublishResult{}, err
	}
	return PublishIssue(ctx, c, gh, issue, leaseToken)
}

// PublishCloseResult ties MCP close replay to the close returned by the daemon.
// A replay after a newer close may report an existing old comment, but must
// never publish the newer close while returning the old close's result.
func PublishCloseResult(ctx context.Context, c Coordinator, gh github.Client, issueID string, closeResult core.CloseIssueResult, leaseGeneration int64, leaseToken string) (PublishResult, error) {
	issue, _, err := c.GetIssue(ctx, issueID)
	if err != nil {
		return PublishResult{}, err
	}
	events, err := c.ListEvents(ctx, issue.ID)
	if err != nil {
		return PublishResult{}, err
	}
	var latestEvent core.Event
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType == "issue_closed" || events[i].EventType == "issue_operator_closed" {
			latestEvent = events[i]
			break
		}
	}
	if latestEvent.ID == "" {
		return PublishResult{}, fmt.Errorf("closed issue has no close event")
	}
	if latestEvent.EventType == "issue_closed" {
		latestGeneration, err := closeLeaseGeneration(latestEvent)
		if err != nil {
			return PublishResult{}, err
		}
		if latestEvent.CreatedAt == closeResult.ClosedAt && latestGeneration == leaseGeneration {
			source, err := publishSource(ctx, gh, issue)
			if err != nil {
				return PublishResult{}, err
			}
			closed, err := LatestClose(events)
			if err != nil {
				return PublishResult{}, err
			}
			return publishSelectedClose(ctx, c, gh, issue, source, events, closed, leaseToken)
		}
	}
	stale := &PublishError{Code: "stale_close", Message: "a newer close exists; publish it with publish_issue"}
	var oldEvent core.Event
	for _, event := range events {
		if event.EventType != "issue_closed" || event.CreatedAt != closeResult.ClosedAt {
			continue
		}
		generation, err := closeLeaseGeneration(event)
		if err != nil {
			return PublishResult{}, err
		}
		if generation == leaseGeneration {
			oldEvent = event
			break
		}
	}
	if oldEvent.ID == "" {
		return PublishResult{}, stale
	}
	if !strings.HasPrefix(issue.ExternalKey, "github:") {
		return PublishResult{}, stale
	}
	ref, err := github.ParseIssueRef(strings.TrimPrefix(issue.ExternalKey, "github:"))
	if err != nil {
		return PublishResult{}, err
	}
	source, err := gh.GetIssue(ctx, ref)
	if err != nil {
		return PublishResult{}, err
	}
	comments, err := gh.ListComments(ctx, source.CommentsURL)
	if err != nil {
		return PublishResult{}, err
	}
	marker := fmt.Sprintf("<!-- dibs:publish issue=%s close_event=%s -->", issue.ID, oldEvent.ID)
	for _, comment := range comments {
		if strings.Contains(comment.Body, marker) {
			return PublishResult{OK: true, Already: true, CommentURL: comment.HTMLURL}, nil
		}
	}
	return PublishResult{}, stale
}

func closeLeaseGeneration(event core.Event) (int64, error) {
	var payload struct {
		LeaseGeneration int64 `json:"lease_generation"`
	}
	if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
		return 0, fmt.Errorf("invalid close event: %w", err)
	}
	return payload.LeaseGeneration, nil
}

func PublishIssue(ctx context.Context, c Coordinator, gh github.Client, issue core.Issue, leaseTokens ...string) (PublishResult, error) {
	source, err := publishSource(ctx, gh, issue)
	if err != nil {
		return PublishResult{}, err
	}
	events, err := c.ListEvents(ctx, issue.ID)
	if err != nil {
		return PublishResult{}, err
	}
	closed, err := LatestClose(events)
	if err != nil {
		return PublishResult{}, err
	}
	return publishSelectedClose(ctx, c, gh, issue, source, events, closed, leaseTokens...)
}

func publishSource(ctx context.Context, gh github.Client, issue core.Issue) (github.Issue, error) {
	if issue.Status != "done" && issue.Status != "cancelled" {
		return github.Issue{}, fmt.Errorf("issue %s is not closed; close it before publishing", issue.ShortID)
	}
	if !strings.HasPrefix(issue.ExternalKey, "github:") {
		return github.Issue{}, fmt.Errorf("issue %s has no GitHub external key", issue.ShortID)
	}
	ref, err := github.ParseIssueRef(strings.TrimPrefix(issue.ExternalKey, "github:"))
	if err != nil {
		return github.Issue{}, fmt.Errorf("invalid GitHub external key: %w", err)
	}
	source, err := gh.GetIssue(ctx, ref)
	if err != nil {
		return github.Issue{}, err
	}
	if source.Locked {
		return github.Issue{}, &github.Error{Code: "locked", Remedy: "unlock the GitHub issue or ask a repository maintainer to do so"}
	}
	if source.IsPullRequest() {
		return github.Issue{}, fmt.Errorf("GitHub source is a pull request, not an issue")
	}
	return source, nil
}

func publishSelectedClose(ctx context.Context, c Coordinator, gh github.Client, issue core.Issue, source github.Issue, events []core.Event, closed CloseRecord, leaseTokens ...string) (PublishResult, error) {
	if closed.Event.ID == "" {
		return PublishResult{}, fmt.Errorf("close event has no ID")
	}
	note := ""
	if HasCloseNoteEvent(events, closed.Event) {
		notes, err := c.ListNotes(ctx, issue.ID)
		if err != nil {
			return PublishResult{}, err
		}
		note = ClosingNote(events, notes, closed.Event)
	}
	if err := ValidatePublicText(note, closed.Branch, leaseTokens...); err != nil {
		return PublishResult{}, err
	}
	marker := fmt.Sprintf("<!-- dibs:publish issue=%s close_event=%s -->", issue.ID, closed.Event.ID)
	comments, err := gh.ListComments(ctx, source.CommentsURL)
	if err != nil {
		return PublishResult{}, err
	}
	for _, comment := range comments {
		if strings.Contains(comment.Body, marker) {
			return PublishResult{OK: true, Already: true, CommentURL: comment.HTMLURL}, nil
		}
	}
	body := RenderPublishComment(issue.ShortID, closed, note, marker)
	comment, err := gh.CreateComment(ctx, source.CommentsURL, body)
	if err != nil {
		return PublishResult{}, err
	}
	return PublishResult{OK: true, CommentURL: comment.HTMLURL}, nil
}

// Only exact token values are checked; the closer's other text is published as written.
func ValidatePublicText(note, branch string, leaseTokens ...string) error {
	for _, key := range []string{"DIBS_LEASE_TOKEN", "DIBS_OPERATOR_TOKEN", "AF_OPERATOR_TOKEN"} {
		value := os.Getenv(key)
		if value != "" && (strings.Contains(note, value) || strings.Contains(branch, value)) {
			return &PublishError{Code: "secret_in_text", Message: "this close cannot be published: closing note or branch contains the current " + key + " value"}
		}
	}
	for _, value := range leaseTokens {
		if value != "" && (strings.Contains(note, value) || strings.Contains(branch, value)) {
			return &PublishError{Code: "secret_in_text", Message: "this close cannot be published: closing note or branch contains the active lease token"}
		}
	}
	return nil
}

func RenderPublishComment(shortID string, closed CloseRecord, note, marker string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**dibs:** `%s` closed as **%s**.\n", shortID, closed.Resolution)
	if closed.PRURL != "" || closed.CommitSHA != "" || closed.Branch != "" {
		b.WriteString("\n")
	}
	if closed.PRURL != "" {
		fmt.Fprintf(&b, "- PR: %s\n", closed.PRURL)
	}
	if closed.CommitSHA != "" {
		fmt.Fprintf(&b, "- Commit: `%s`", closed.CommitSHA)
		if closed.Branch != "" {
			fmt.Fprintf(&b, " on `%s`", closed.Branch)
		}
		b.WriteString("\n")
	} else if closed.Branch != "" {
		fmt.Fprintf(&b, "- Branch: `%s`\n", closed.Branch)
	}
	if note != "" {
		b.WriteString("\n")
		runes := []rune(note)
		if len(runes) > 2000 {
			note = string(runes[:1999]) + "…"
		}
		for _, line := range strings.Split(note, "\n") {
			fmt.Fprintf(&b, "> %s\n", line)
		}
	}
	fmt.Fprintf(&b, "\n%s", marker)
	return b.String()
}
