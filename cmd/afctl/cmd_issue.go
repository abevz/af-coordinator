package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/core"
)

// ─── Issue ───────────────────────────────────────────────────────────────────

func runIssue(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue <create|get|list|ready|claim|heartbeat|release>")
		os.Exit(1)
	}

	switch args[0] {
	case "create":
		runIssueCreate(ctx, c, args[1:])
	case "get":
		runIssueGet(ctx, c, args[1:])
	case "list":
		runIssueList(ctx, c, args[1:])
	case "ready":
		runIssueReady(ctx, c, args[1:])
	case "claim":
		runIssueClaim(ctx, c, args[1:])
	case "heartbeat":
		runIssueHeartbeat(ctx, c, args[1:])
	case "release":
		runIssueRelease(ctx, c, args[1:])
	case "update":
		runIssueUpdate(ctx, c, args[1:])
	case "close":
		runIssueClose(ctx, c, args[1:])
	case "link":
		runIssueLink(ctx, c, args[1:])
	case "dependency":
		runIssueDependency(ctx, c, args[1:])
	case "note":
		runIssueNote(ctx, c, args[1:])
	case "events":
		runIssueEvents(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown issue subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runIssueCreate(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue create --project <key> --scope-kind <project|repository|worktree> --title <title> [--repo <repo>] [--worktree <worktree>] [--description <desc>] [--priority <n>]")
		os.Exit(1)
	}

	var req core.CreateIssueRequest
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				req.Project = args[i+1]
				i++
			}
		case "--scope-kind":
			if i+1 < len(args) {
				req.ScopeKind = args[i+1]
				i++
			}
		case "--title":
			if i+1 < len(args) {
				req.Title = args[i+1]
				i++
			}
		case "--repo":
			if i+1 < len(args) {
				req.Repo = args[i+1]
				i++
			}
		case "--worktree":
			if i+1 < len(args) {
				req.Worktree = args[i+1]
				i++
			}
		case "--description":
			if i+1 < len(args) {
				req.Description = args[i+1]
				i++
			}
		case "--priority":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &req.Priority)
				i++
			}
		}
	}

	actor, err := resolveActor("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	req.Actor = actor

	issue, err := c.CreateIssue(ctx, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issue)
		return
	}
	printIssue(issue)
}

func runIssueGet(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue get <issue-id-or-short-id>")
		os.Exit(1)
	}

	issueID := args[0]
	issue, lease, err := c.GetIssue(ctx, issueID)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		resp := map[string]any{
			"issue": issue,
		}
		if lease != nil {
			resp["lease"] = lease
		}
		json.NewEncoder(os.Stdout).Encode(resp)
		return
	}
	printIssueDetailed(issue, lease)
}

func runIssueList(ctx context.Context, c *client.Client, args []string) {
	project := ""
	repo := ""
	worktree := ""
	status := ""
	assignee := ""
	limit := 0
	offset := 0

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				project = args[i+1]
				i++
			}
		case "--repo":
			if i+1 < len(args) {
				repo = args[i+1]
				i++
			}
		case "--worktree":
			if i+1 < len(args) {
				worktree = args[i+1]
				i++
			}
		case "--status":
			if i+1 < len(args) {
				status = args[i+1]
				i++
			}
		case "--assignee":
			if i+1 < len(args) {
				assignee = args[i+1]
				i++
			}
		case "--limit":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &limit)
				i++
			}
		case "--offset":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &offset)
				i++
			}
		}
	}

	// limit/offset are defined for future use; pass them through when the API supports pagination
	_ = limit
	_ = offset

	issues, err := c.ListIssues(ctx, project, repo, worktree, status, assignee)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issues)
		return
	}
	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return
	}
	printIssuesTable(issues)
}

func runIssueReady(ctx context.Context, c *client.Client, args []string) {
	project := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--project" && i+1 < len(args) {
			project = args[i+1]
		}
	}

	issues, err := c.ListReadyIssues(ctx, project)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issues)
		return
	}
	if len(issues) == 0 {
		fmt.Println("No ready issues found.")
		return
	}
	printIssuesTable(issues)
}

func runIssueClaim(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue claim <issue-id> [--holder <name>|--actor <name>] [--ttl <seconds>]")
		os.Exit(1)
	}

	issueID := args[0]
	holder := ""
	ttl := 3600

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--holder", "--actor":
			if i+1 < len(args) {
				holder = args[i+1]
				i++
			}
		case "--ttl":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &ttl)
				i++
			}
		}
	}

	var err error
	holder, err = resolveActor(holder)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	resp, err := c.ClaimIssue(ctx, issueID, holder, ttl)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(resp)
		return
	}
	fmt.Printf("Lease Token: %s\n", resp.LeaseToken)
	fmt.Printf("Expires At:  %s\n", resp.ExpiresAt)
}

func runIssueHeartbeat(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue heartbeat <issue-id> --lease-token <token> [--ttl <seconds>]")
		os.Exit(1)
	}

	issueID := args[0]
	leaseToken := ""
	ttl := 3600

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--lease-token":
			if i+1 < len(args) {
				leaseToken = args[i+1]
				i++
			}
		case "--ttl":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &ttl)
				i++
			}
		}
	}

	if leaseToken == "" {
		fmt.Fprintln(os.Stderr, "error: --lease-token is required")
		os.Exit(1)
	}

	expiresAt, err := c.HeartbeatLease(ctx, issueID, leaseToken, ttl)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"expires_at": expiresAt})
		return
	}
	fmt.Printf("Expires At: %s\n", expiresAt)
}

func runIssueRelease(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue release <issue-id> --lease-token <token>")
		os.Exit(1)
	}

	issueID := args[0]
	leaseToken := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--lease-token":
			if i+1 < len(args) {
				leaseToken = args[i+1]
				i++
			}
		}
	}

	if leaseToken == "" {
		fmt.Fprintln(os.Stderr, "error: --lease-token is required")
		os.Exit(1)
	}

	if err := c.ReleaseLease(ctx, issueID, leaseToken); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return
	}
	fmt.Println("Lease released.")
}

func runIssueUpdate(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue update <issue-id> [--title ...] [--description ...] [--priority N] [--assignee ...] [--status ...] --expected-version N [--lease-token ...]")
		os.Exit(1)
	}

	issueID := args[0]
	var req core.UpdateIssueRequest
	req.ExpectedVersion = -1

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--title":
			if i+1 < len(args) {
				req.Title = args[i+1]
				i++
			}
		case "--description":
			if i+1 < len(args) {
				req.Description = args[i+1]
				i++
			}
		case "--priority":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &req.Priority)
				i++
			}
		case "--assignee":
			if i+1 < len(args) {
				req.Assignee = args[i+1]
				i++
			}
		case "--status":
			if i+1 < len(args) {
				req.Status = args[i+1]
				i++
			}
		case "--expected-version":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &req.ExpectedVersion)
				i++
			}
		case "--lease-token":
			if i+1 < len(args) {
				req.LeaseToken = args[i+1]
				i++
			}
		}
	}

	if req.ExpectedVersion < 0 {
		fmt.Fprintln(os.Stderr, "error: --expected-version is required")
		os.Exit(1)
	}

	actor, err := resolveActor("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	req.Actor = actor

	issue, err := c.UpdateIssue(ctx, issueID, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issue)
		return
	}
	printIssue(issue)
}

func runIssueClose(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue close <issue-id> --resolution done|cancelled --expected-version N --lease-token ...")
		os.Exit(1)
	}

	issueID := args[0]
	var req core.CloseIssueRequest
	req.ExpectedVersion = -1

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--resolution":
			if i+1 < len(args) {
				req.Resolution = args[i+1]
				i++
			}
		case "--expected-version":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &req.ExpectedVersion)
				i++
			}
		case "--lease-token":
			if i+1 < len(args) {
				req.LeaseToken = args[i+1]
				i++
			}
		}
	}

	if req.Resolution == "" {
		fmt.Fprintln(os.Stderr, "error: --resolution is required (done or cancelled)")
		os.Exit(1)
	}
	if req.ExpectedVersion < 0 {
		fmt.Fprintln(os.Stderr, "error: --expected-version is required")
		os.Exit(1)
	}
	if req.LeaseToken == "" {
		fmt.Fprintln(os.Stderr, "error: --lease-token is required")
		os.Exit(1)
	}

	actor, err := resolveActor("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	req.Actor = actor

	if err := c.CloseIssue(ctx, issueID, req); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return
	}
	fmt.Println("Issue closed.")
}

func runIssueLink(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue link <issue-id> --artifact <artifact-id> [--relation implements|...]")
		os.Exit(1)
	}

	issueID := args[0]
	var req core.LinkArtifactRequest

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			if i+1 < len(args) {
				req.Artifact = args[i+1]
				i++
			}
		case "--relation":
			if i+1 < len(args) {
				req.Relation = args[i+1]
				i++
			}
		}
	}

	if req.Artifact == "" {
		fmt.Fprintln(os.Stderr, "error: --artifact is required")
		os.Exit(1)
	}

	if err := c.LinkArtifact(ctx, issueID, req); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return
	}
	fmt.Println("Artifact linked.")
}

func runIssueDependency(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue dependency <add|remove> <issue-id> --depends-on <other-issue> [--kind blocks|parent|related|discovered-from]")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runIssueDependencyAdd(ctx, c, args[1:])
	case "remove":
		runIssueDependencyRemove(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown dependency subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runIssueDependencyAdd(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue dependency add <issue-id> --depends-on <other-issue> [--kind blocks|parent|related|discovered-from]")
		os.Exit(1)
	}

	issueID := args[0]
	var req core.AddDependencyRequest

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--depends-on":
			if i+1 < len(args) {
				req.DependsOn = args[i+1]
				i++
			}
		case "--kind":
			if i+1 < len(args) {
				req.Kind = args[i+1]
				i++
			}
		}
	}

	if req.DependsOn == "" {
		fmt.Fprintln(os.Stderr, "error: --depends-on is required")
		os.Exit(1)
	}

	if err := c.AddDependency(ctx, issueID, req); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return
	}
	fmt.Println("Dependency added.")
}

func runIssueDependencyRemove(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue dependency remove <issue-id> --depends-on <other-issue> [--kind blocks]")
		os.Exit(1)
	}

	issueID := args[0]
	dependsOn := ""
	kind := "blocks"

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--depends-on":
			if i+1 < len(args) {
				dependsOn = args[i+1]
				i++
			}
		case "--kind":
			if i+1 < len(args) {
				kind = args[i+1]
				i++
			}
		}
	}

	if dependsOn == "" {
		fmt.Fprintln(os.Stderr, "error: --depends-on is required")
		os.Exit(1)
	}

	if err := c.RemoveDependency(ctx, issueID, dependsOn, kind); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return
	}
	fmt.Println("Dependency removed.")
}

// ─── Issue Note ─────────────────────────────────────────────────────────────

func runIssueNote(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue note <add|list> <issue-id> [--author <name> --body <text>]")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runIssueNoteAdd(ctx, c, args[1:])
	case "list":
		runIssueNoteList(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown note subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runIssueNoteAdd(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue note add <issue-id> [--author <name>|--actor <name>] --body <text>")
		os.Exit(1)
	}

	issueID := args[0]
	author := ""
	body := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--author", "--actor":
			if i+1 < len(args) {
				author = args[i+1]
				i++
			}
		case "--body":
			if i+1 < len(args) {
				body = args[i+1]
				i++
			}
		}
	}

	var err error
	author, err = resolveActor(author)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if body == "" {
		fmt.Fprintln(os.Stderr, "error: --body is required")
		os.Exit(1)
	}

	note, err := c.CreateNote(ctx, issueID, author, body)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(note)
		return
	}
	fmt.Printf("Note ID:    %s\n", note.ID)
	fmt.Printf("Issue ID:   %s\n", note.IssueID)
	fmt.Printf("Author:     %s\n", note.Author)
	fmt.Printf("Body:       %s\n", note.Body)
	fmt.Printf("Created At: %s\n", note.CreatedAt)
}

func runIssueNoteList(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue note list <issue-id>")
		os.Exit(1)
	}

	issueID := args[0]

	notes, err := c.ListNotes(ctx, issueID)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(notes)
		return
	}
	if len(notes) == 0 {
		fmt.Println("No notes found.")
		return
	}
	for _, n := range notes {
		fmt.Printf("Note ID:    %s\n", n.ID)
		fmt.Printf("Author:     %s\n", n.Author)
		fmt.Printf("Body:       %s\n", n.Body)
		fmt.Printf("Created At: %s\n\n", n.CreatedAt)
	}
}

// ─── Issue Events ──────────────────────────────────────────────────────────

func runIssueEvents(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue events list <issue-id>")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		runIssueEventsList(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown events subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runIssueEventsList(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl issue events list <issue-id>")
		os.Exit(1)
	}

	issueID := args[0]

	events, err := c.ListEvents(ctx, issueID)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(events)
		return
	}
	if len(events) == 0 {
		fmt.Println("No events found.")
		return
	}
	for _, e := range events {
		fmt.Printf("Event ID:    %s\n", e.ID)
		fmt.Printf("Actor:       %s\n", e.Actor)
		fmt.Printf("Type:        %s\n", e.EventType)
		fmt.Printf("Payload:     %s\n", e.PayloadJSON)
		fmt.Printf("Created At:  %s\n\n", e.CreatedAt)
	}
}
