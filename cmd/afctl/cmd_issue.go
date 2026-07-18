package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/core"
)

// ─── Issue ───────────────────────────────────────────────────────────────────

func runIssue(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue <create|get|list|ready|claim|heartbeat|release|handoff|update|close|operator-close|operator-reopen>")
	}

	switch args[0] {
	case "create":
		return runIssueCreate(ctx, c, args[1:])
	case "create-form":
		return runIssueCreateForm(ctx, c, args[1:])
	case "get":
		return runIssueGet(ctx, c, args[1:])
	case "list":
		return runIssueList(ctx, c, args[1:])
	case "ready":
		return runIssueReady(ctx, c, args[1:])
	case "claim":
		return runIssueClaim(ctx, c, args[1:])
	case "heartbeat":
		return runIssueHeartbeat(ctx, c, args[1:])
	case "release":
		return runIssueRelease(ctx, c, args[1:])
	case "handoff":
		return runIssueHandoff(ctx, c, args[1:])
	case "update":
		return runIssueUpdate(ctx, c, args[1:])
	case "close":
		return runIssueClose(ctx, c, args[1:])
	case "operator-close":
		return runIssueOperatorClose(ctx, c, args[1:])
	case "operator-reopen":
		return runIssueOperatorReopen(ctx, c, args[1:])
	case "link":
		return runIssueLink(ctx, c, args[1:])
	case "unlink":
		return runIssueUnlink(ctx, c, args[1:])
	case "dependency":
		return runIssueDependency(ctx, c, args[1:])
	case "note":
		return runIssueNote(ctx, c, args[1:])
	case "events":
		return runIssueEvents(ctx, c, args[1:])
	default:
		return fmt.Errorf("unknown issue subcommand: %s\n", args[0])
	}
}

func runIssueCreate(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("%s", "Usage: afctl issue create --project <key> --scope-kind <project|repository|worktree> --title <title> [--type <task|bug|feature|epic|chore>] [--repo <repo>] [--worktree <worktree>] [--external-key <key>] [--description <desc>] [--acceptance <criteria>] [--priority <n>]")
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
		case "--type":
			if i+1 < len(args) {
				req.IssueType = args[i+1]
				i++
			}
		case "--title":
			if i+1 < len(args) {
				req.Title = args[i+1]
				i++
			}
		case "--external-key":
			if i+1 < len(args) {
				req.ExternalKey = args[i+1]
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
		case "--acceptance":
			if i+1 < len(args) {
				req.AcceptanceCriteria = args[i+1]
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
		return fmt.Errorf("error: %v\n", err)
	}
	req.Actor = actor

	issue, err := c.CreateIssue(ctx, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issue)
		return nil
	}
	printIssue(issue)
	return nil
}

func runIssueGet(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue get <issue-id-or-short-id> [--full]")
	}

	fullView := false
	issueID := ""

	for i := 0; i < len(args); i++ {
		if args[i] == "--full" {
			fullView = true
		} else if issueID == "" {
			issueID = args[i]
		}
	}

	if issueID == "" {
		return fmt.Errorf("%s", "Usage: afctl issue get <issue-id-or-short-id> [--full]")
	}

	issue, lease, err := c.GetIssue(ctx, issueID)
	if err != nil {
		fail(err)
	}

	var events []core.Event
	var notes []core.Note
	var links []core.ArtifactRef

	if fullView {
		events, err = c.ListEvents(ctx, issueID)
		if err != nil {
			fail(err)
		}

		notes, err = c.ListNotes(ctx, issueID)
		if err != nil {
			fail(err)
		}

		links, err = c.ListIssueLinks(ctx, issueID)
		if err != nil {
			fail(err)
		}
	}

	if jsonOutput {
		resp := map[string]any{
			"issue": issue,
		}
		if fullView {
			resp["events"] = events
			resp["notes"] = notes
			resp["links"] = links
		}
		if lease != nil {
			resp["lease"] = lease
		}
		json.NewEncoder(os.Stdout).Encode(resp)
		return nil
	}

	if fullView {
		printIssueFull(issue, lease, events, notes, links)
	} else {
		printIssueDetailed(issue, lease)
	}
	return nil
}

const issueListUsage = `Usage: afctl issue list [filters]
       afctl ls [filters]

Filters:
  --project <key[,key...]>       Project key(s); values are ORed
  --type <task|bug|feature|epic|chore[,..]>
                                  Issue type(s); values are ORed
  --status <status[,status...]>  Status value(s); values are ORed
  --repo <repo>                  Repository ID or logical name
  --worktree <worktree>          Worktree ID or path
  --assignee <actor>             Exact assignee
  --external-key <key>           Exact external key
  --limit <n> --offset <n>       Reserved pagination parameters
`

func runIssueList(ctx context.Context, c *client.Client, args []string) error {
	params, help, err := parseIssueListArgs(args)
	if err != nil {
		return err
	}
	if help {
		fmt.Fprint(os.Stdout, issueListUsage)
		return nil
	}

	issues, err := c.ListIssuesWithFilters(ctx, params)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issues)
		return nil
	}
	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return nil
	}
	printIssuesTable(issues)
	return nil
}

func parseIssueListArgs(args []string) (core.IssueListParams, bool, error) {
	var params core.IssueListParams
	for i := 0; i < len(args); i++ {
		flag := args[i]
		if flag == "--help" || flag == "-h" {
			return core.IssueListParams{}, true, nil
		}
		switch flag {
		case "--project", "--status", "--type", "--repo", "--worktree", "--assignee", "--external-key", "--limit", "--offset":
		default:
			return core.IssueListParams{}, false, fmt.Errorf("error: unknown flag: %s", flag)
		}

		value, err := issueListFlagValue(args, i, flag)
		if err != nil {
			return core.IssueListParams{}, false, err
		}

		switch flag {
		case "--project":
			var values []string
			values, err = core.NormalizeIssueListValues([]string{value})
			params.Projects = append(params.Projects, values...)
		case "--status":
			var values []string
			values, err = core.NormalizeIssueListValues([]string{value})
			params.Statuses = append(params.Statuses, values...)
		case "--type":
			var values []string
			values, err = core.NormalizeIssueListValues([]string{value})
			params.IssueTypes = append(params.IssueTypes, values...)
		case "--repo":
			params.Repo = value
		case "--worktree":
			params.Worktree = value
		case "--assignee":
			params.Assignee = value
		case "--external-key":
			params.ExternalKey = value
		case "--limit", "--offset":
			if _, parseErr := strconv.Atoi(value); parseErr != nil {
				return core.IssueListParams{}, false, fmt.Errorf("error: %s requires an integer", flag)
			}
		}
		if err != nil {
			return core.IssueListParams{}, false, fmt.Errorf("error: %s %w", flag, err)
		}
		i++
	}
	return params, false, nil
}

func issueListFlagValue(args []string, index int, flag string) (string, error) {
	if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
		return "", fmt.Errorf("error: %s requires a value", flag)
	}
	return args[index+1], nil
}

func runIssueReady(ctx context.Context, c *client.Client, args []string) error {
	project := ""
	repo := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--project" && i+1 < len(args) {
			project = args[i+1]
			i++
		} else if args[i] == "--repo" && i+1 < len(args) {
			repo = args[i+1]
			i++
		}
	}

	issues, err := c.ListReadyIssues(ctx, project, repo)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issues)
		return nil
	}
	if len(issues) == 0 {
		fmt.Println("No ready issues found.")
		return nil
	}
	printIssuesTable(issues)
	return nil
}

func runIssueClaim(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue claim <issue-id> [--holder <name>|--actor <name>] [--ttl <seconds>] [--session-id <id>]")
	}

	issueID := args[0]
	holder := ""
	ttl := 3600
	sessionID := ""

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
		case "--session-id":
			if i+1 < len(args) {
				sessionID = args[i+1]
				i++
			}
		}
	}

	var err error
	holder, err = resolveActor(holder)
	if err != nil {
		return fmt.Errorf("%s", err)
	}

	resp, err := c.ClaimIssueWithSession(ctx, issueID, holder, ttl, sessionID)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(resp)
		return nil
	}
	fmt.Printf("Lease Token: %s\n", resp.LeaseToken)
	fmt.Printf("Attempt ID:  %s\n", resp.AttemptID)
	fmt.Printf("Expires At:  %s\n", resp.ExpiresAt)
	return nil
}

func runIssueHeartbeat(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue heartbeat <issue-id> --lease-token <token> [--ttl <seconds>]")
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
		return fmt.Errorf("%s", "error: --lease-token is required")
	}

	expiresAt, err := c.HeartbeatLease(ctx, issueID, leaseToken, ttl)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"expires_at": expiresAt})
		return nil
	}
	fmt.Printf("Expires At: %s\n", expiresAt)
	return nil
}

func runIssueRelease(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue release <issue-id> --lease-token <token>")
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
		return fmt.Errorf("%s", "error: --lease-token is required")
	}

	if err := c.ReleaseLease(ctx, issueID, leaseToken); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return nil
	}
	fmt.Println("Lease released.")
	return nil
}

func runIssueHandoff(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue handoff <issue-id> --lease-token <token> --note \"HANDOFF: next steps\"")
	}

	issueID := args[0]
	leaseToken := ""
	note := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--lease-token":
			if i+1 < len(args) {
				leaseToken = args[i+1]
				i++
			}
		case "--note":
			if i+1 < len(args) {
				note = args[i+1]
				i++
			}
		default:
			return fmt.Errorf("unknown flag for issue handoff: %s", args[i])
		}
	}
	if leaseToken == "" {
		return fmt.Errorf("%s", "error: --lease-token is required")
	}
	if err := core.ValidateHandoffRequest(core.HandoffRequest{Note: note}); err != nil {
		return fmt.Errorf("error: %v", err)
	}

	resp, err := c.HandoffLease(ctx, issueID, leaseToken, note)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(resp)
		return nil
	}
	fmt.Printf("Handoff note recorded: %s\nLease released.\n", resp.Note.ID)
	return nil
}

func runIssueUpdate(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue update <issue-id> [--title ...] [--type <task|bug|feature|epic|chore>] [--external-key ...] [--description ...] [--acceptance ...] [--priority N] [--assignee ...] [--status ...] --expected-version N [--lease-token ...]")
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
		case "--type":
			if i+1 < len(args) {
				req.IssueType = args[i+1]
				i++
			}
		case "--external-key":
			if i+1 < len(args) {
				req.ExternalKey = args[i+1]
				i++
			}
		case "--description":
			if i+1 < len(args) {
				req.Description = args[i+1]
				i++
			}
		case "--acceptance":
			if i+1 < len(args) {
				req.AcceptanceCriteria = args[i+1]
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
		return fmt.Errorf("%s", "error: --expected-version is required")
	}

	actor, err := resolveActor("")
	if err != nil {
		return fmt.Errorf("error: %v\n", err)
	}
	req.Actor = actor

	issue, err := c.UpdateIssue(ctx, issueID, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issue)
		return nil
	}
	printIssue(issue)
	return nil
}

func runIssueClose(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue close <issue-id> --resolution done|cancelled --expected-version N --lease-token ... [--branch <name>] [--pr-url <url>] [--commit-sha <sha>] [--note \"what was done\"]")
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
		case "--branch":
			if i+1 < len(args) {
				req.Branch = args[i+1]
				i++
			}
		case "--pr-url":
			if i+1 < len(args) {
				req.PRURL = args[i+1]
				i++
			}
		case "--commit-sha":
			if i+1 < len(args) {
				req.CommitSHA = args[i+1]
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
		case "--note":
			if i+1 < len(args) {
				req.Note = args[i+1]
				i++
			}
		}
	}

	if req.Resolution == "" {
		return fmt.Errorf("%s", "error: --resolution is required (done or cancelled)")
	}
	if req.ExpectedVersion < 0 {
		return fmt.Errorf("%s", "error: --expected-version is required")
	}
	if req.LeaseToken == "" {
		return fmt.Errorf("%s", "error: --lease-token is required")
	}

	actor, err := resolveActor("")
	if err != nil {
		return fmt.Errorf("error: %v\n", err)
	}
	req.Actor = actor

	result, err := c.CloseIssue(ctx, issueID, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(result)
		return nil
	}
	fmt.Println("Issue closed.")
	if result.Branch != "" {
		fmt.Printf("Branch:      %s\n", result.Branch)
	}
	if result.PRURL != "" {
		fmt.Printf("PR URL:      %s\n", result.PRURL)
	}
	if result.CommitSHA != "" {
		fmt.Printf("Commit SHA:  %s\n", result.CommitSHA)
	}
	if result.ExternalKey != "" {
		fmt.Printf("External:    %s\n", result.ExternalKey)
	}
	return nil
}

func runIssueOperatorClose(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue operator-close <issue-id> --resolution done|cancelled --expected-version N --reason \"why operator closure is needed\"")
	}

	issueID := args[0]
	req := core.OperatorCloseIssueRequest{ExpectedVersion: -1}
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
		case "--reason":
			if i+1 < len(args) {
				req.Reason = args[i+1]
				i++
			}
		default:
			return fmt.Errorf("unknown flag for issue operator-close: %s", args[i])
		}
	}
	if req.Resolution == "" {
		return fmt.Errorf("%s", "error: --resolution is required (done or cancelled)")
	}
	if req.ExpectedVersion <= 0 {
		return fmt.Errorf("%s", "error: --expected-version is required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return fmt.Errorf("%s", "error: --reason is required")
	}
	actor, err := resolveActor("")
	if err != nil {
		return fmt.Errorf("error: %v\n", err)
	}
	req.Actor = actor

	result, err := c.OperatorCloseIssue(ctx, issueID, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(result)
		return nil
	}
	fmt.Println("Issue closed by operator.")
	return nil
}

func runIssueOperatorReopen(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue operator-reopen <issue-id> --expected-version N --reason \"why work is reopening\"")
	}

	issueID := args[0]
	req := core.OperatorReopenIssueRequest{ExpectedVersion: -1}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--expected-version":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &req.ExpectedVersion)
				i++
			}
		case "--reason":
			if i+1 < len(args) {
				req.Reason = args[i+1]
				i++
			}
		default:
			return fmt.Errorf("unknown flag for issue operator-reopen: %s", args[i])
		}
	}
	if req.ExpectedVersion <= 0 {
		return fmt.Errorf("%s", "error: --expected-version is required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return fmt.Errorf("%s", "error: --reason is required")
	}
	actor, err := resolveActor("")
	if err != nil {
		return fmt.Errorf("error: %v\n", err)
	}
	req.Actor = actor

	issue, err := c.OperatorReopenIssue(ctx, issueID, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(issue)
		return nil
	}
	fmt.Println("Issue reopened by operator.")
	return nil
}

func runIssueLink(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue link <issue-id> [--artifact <id-or-path> | --path <relative-path>] [--repo <name>] [--kind spec] [--relation implements]")
	}

	issueID := args[0]
	var req core.LinkArtifactRequest
	var path, repo, kind string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			if i+1 < len(args) {
				req.Artifact = args[i+1]
				i++
			}
		case "--path":
			if i+1 < len(args) {
				path = args[i+1]
				i++
			}
		case "--repo":
			if i+1 < len(args) {
				repo = args[i+1]
				i++
			}
		case "--kind":
			if i+1 < len(args) {
				kind = args[i+1]
				i++
			}
		case "--relation":
			if i+1 < len(args) {
				req.Relation = args[i+1]
				i++
			}
		}
	}

	if req.Artifact == "" && path == "" {
		return fmt.Errorf("%s", "error: --artifact or --path is required")
	}
	if req.Artifact != "" && path != "" {
		return fmt.Errorf("%s", "error: cannot specify both --artifact and --path")
	}

	if path != "" {
		if repo == "" {
			issue, _, err := c.GetIssue(ctx, issueID)
			if err != nil {
				return fmt.Errorf("failed to get issue: %w", err)
			}
			if issue.RepositoryID == "" {
				return fmt.Errorf("error: issue is not repository-scoped, --repo is required with --path")
			}
			repo = issue.RepositoryID
		}
		if kind == "" {
			kind = "spec"
		}

		art, err := c.CreateArtifact(ctx, core.CreateArtifactRequest{
			Repo:         repo,
			RelativePath: path,
			Kind:         kind,
		})
		if err != nil {
			return fmt.Errorf("failed to upsert artifact: %w", err)
		}
		req.Artifact = art.ID
	}

	if err := c.LinkArtifact(ctx, issueID, req); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return nil
	}
	fmt.Println("Artifact linked.")
	return nil
}

func runIssueUnlink(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue unlink <issue-id> (--path <relative-path> | --artifact <id-or-path>) [--relation implements]")
	}

	issueID := args[0]
	var req core.UnlinkArtifactRequest

	flagValue := func(i int) (string, error) {
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return "", fmt.Errorf("error: %s requires a value", args[i])
		}
		return args[i+1], nil
	}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--path", "--artifact":
			value, err := flagValue(i)
			if err != nil {
				return err
			}
			req.Artifact = value
			i++
		case "--relation":
			value, err := flagValue(i)
			if err != nil {
				return err
			}
			req.Relation = value
			i++
		}
	}

	if req.Artifact == "" {
		return fmt.Errorf("%s", "error: --path or --artifact is required")
	}

	act, err := resolveActor("")
	if err != nil {
		return err
	}
	req.Actor = act

	if err := c.UnlinkArtifact(ctx, issueID, req); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return nil
	}
	fmt.Println("Artifact unlinked.")
	return nil
}

func runIssueDependency(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue dependency <add|remove> <issue-id> --depends-on <other-issue> [--kind blocks|parent|related|discovered-from]")
	}

	switch args[0] {
	case "add":
		return runIssueDependencyAdd(ctx, c, args[1:])
	case "remove":
		return runIssueDependencyRemove(ctx, c, args[1:])
	default:
		return fmt.Errorf("unknown dependency subcommand: %s\n", args[0])
	}
}

const dependencyAddUsage = "Usage: afctl issue dependency add <issue-id> (--blocked-by <id> | --blocks <id> | --depends-on <id> [--kind blocks|parent|related|discovered-from])"

// dependencyEdge is the resolved, direction-unambiguous form of a dependency
// command: the issue that owns the stored edge, the issue it depends on, the
// kind, and a human-readable confirmation.
type dependencyEdge struct {
	target    string
	dependsOn string
	kind      string
	message   string
}

// resolveDependencyEdge maps the directional flags of `dependency add` onto the
// single stored edge shape (owner depends_on target, kind). Exactly one of
// --blocked-by, --blocks, or --depends-on must be given, so an author never has
// to reason about which side the word "blocks" refers to.
func resolveDependencyEdge(issueID string, args []string) (dependencyEdge, error) {
	var dependsOn, kind, blockedBy, blocks string
	for i := 0; i < len(args); i++ {
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
		case "--blocked-by":
			if i+1 < len(args) {
				blockedBy = args[i+1]
				i++
			}
		case "--blocks":
			if i+1 < len(args) {
				blocks = args[i+1]
				i++
			}
		}
	}

	forms := 0
	for _, v := range []string{dependsOn, blockedBy, blocks} {
		if v != "" {
			forms++
		}
	}
	if forms == 0 {
		return dependencyEdge{}, fmt.Errorf("%s", "error: one of --blocked-by, --blocks, or --depends-on is required")
	}
	if forms > 1 {
		return dependencyEdge{}, fmt.Errorf("%s", "error: --blocked-by, --blocks, and --depends-on are mutually exclusive")
	}
	if (blockedBy != "" || blocks != "") && kind != "" {
		return dependencyEdge{}, fmt.Errorf("%s", "error: --kind cannot be combined with --blocked-by or --blocks (both mean kind=blocks)")
	}

	edge := dependencyEdge{target: issueID}
	switch {
	case blockedBy != "":
		edge.dependsOn, edge.kind = blockedBy, "blocks"
		edge.message = fmt.Sprintf("%s is now blocked by %s", issueID, blockedBy)
	case blocks != "":
		edge.target, edge.dependsOn, edge.kind = blocks, issueID, "blocks"
		edge.message = fmt.Sprintf("%s is now blocked by %s", blocks, issueID)
	default: // --depends-on
		edge.dependsOn, edge.kind = dependsOn, kind
		switch kind {
		case "blocks":
			edge.message = fmt.Sprintf("%s is now blocked by %s", issueID, dependsOn)
		case "":
			edge.message = fmt.Sprintf("%s now depends on %s", issueID, dependsOn)
		default:
			edge.message = fmt.Sprintf("%s now has a %s dependency on %s", issueID, kind, dependsOn)
		}
	}
	return edge, nil
}

func runIssueDependencyAdd(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", dependencyAddUsage)
	}

	edge, err := resolveDependencyEdge(args[0], args[1:])
	if err != nil {
		return err
	}
	act, err := resolveActor("")
	if err != nil {
		return err
	}
	if err := c.AddDependency(ctx, edge.target, core.AddDependencyRequest{
		DependsOn: edge.dependsOn,
		Kind:      edge.kind,
		Actor:     act,
	}); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return nil
	}
	fmt.Println("Dependency added.")
	fmt.Println(edge.message)
	return nil
}

func runIssueDependencyRemove(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue dependency remove <issue-id> (--blocked-by <id> | --blocks <id> | --depends-on <id> [--kind blocks])")
	}

	issueID := args[0]
	var dependsOn, blockedBy, blocks string
	kind := "blocks"
	kindSet := false

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
				kindSet = true
				i++
			}
		case "--blocked-by":
			if i+1 < len(args) {
				blockedBy = args[i+1]
				i++
			}
		case "--blocks":
			if i+1 < len(args) {
				blocks = args[i+1]
				i++
			}
		}
	}

	forms := 0
	for _, v := range []string{dependsOn, blockedBy, blocks} {
		if v != "" {
			forms++
		}
	}
	if forms == 0 {
		return fmt.Errorf("%s", "error: one of --blocked-by, --blocks, or --depends-on is required")
	}
	if forms > 1 {
		return fmt.Errorf("%s", "error: --blocked-by, --blocks, and --depends-on are mutually exclusive")
	}
	if (blockedBy != "" || blocks != "") && kindSet {
		return fmt.Errorf("%s", "error: --kind cannot be combined with --blocked-by or --blocks (both mean kind=blocks)")
	}

	// Mirror the add direction so removal targets the same stored edge.
	target := issueID
	switch {
	case blockedBy != "":
		dependsOn, kind = blockedBy, "blocks"
	case blocks != "":
		target, dependsOn, kind = blocks, issueID, "blocks"
	}

	act, err := resolveActor("")
	if err != nil {
		return err
	}

	if err := c.RemoveDependency(ctx, target, core.RemoveDependencyRequest{
		DependsOn: dependsOn,
		Kind:      kind,
		Actor:     act,
	}); err != nil {
		fail(err)
	}
	if jsonOutput {
		fmt.Println(`{"status":"ok"}`)
		return nil
	}
	fmt.Println("Dependency removed.")
	return nil
}

// ─── Issue Note ─────────────────────────────────────────────────────────────

func runIssueNote(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue note <add|list> <issue-id> [--author <name> --body <text>]")
	}

	switch args[0] {
	case "add":
		return runIssueNoteAdd(ctx, c, args[1:])
	case "list":
		return runIssueNoteList(ctx, c, args[1:])
	default:
		return fmt.Errorf("unknown note subcommand: %s\n", args[0])
	}
}

func runIssueNoteAdd(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue note add <issue-id> [--author <name>|--actor <name>] --body <text>")
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
		return fmt.Errorf("%s", err)
	}
	if body == "" {
		return fmt.Errorf("%s", "error: --body is required")
	}

	note, err := c.CreateNote(ctx, issueID, author, body)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(note)
		return nil
	}
	fmt.Printf("Note ID:    %s\n", note.ID)
	fmt.Printf("Issue ID:   %s\n", note.IssueID)
	fmt.Printf("Author:     %s\n", note.Author)
	fmt.Printf("Body:       %s\n", note.Body)
	fmt.Printf("Created At: %s\n", note.CreatedAt)
	return nil
}

func runIssueNoteList(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue note list <issue-id>")
	}

	issueID := args[0]

	notes, err := c.ListNotes(ctx, issueID)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(notes)
		return nil
	}
	if len(notes) == 0 {
		fmt.Println("No notes found.")
		return nil
	}
	for _, n := range notes {
		fmt.Printf("Note ID:    %s\n", n.ID)
		fmt.Printf("Author:     %s\n", n.Author)
		fmt.Printf("Body:       %s\n", n.Body)
		fmt.Printf("Created At: %s\n\n", n.CreatedAt)
	}
	return nil
}

// ─── Issue Events ──────────────────────────────────────────────────────────

func runIssueEvents(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue events list <issue-id>")
	}

	switch args[0] {
	case "list":
		return runIssueEventsList(ctx, c, args[1:])
	default:
		return fmt.Errorf("unknown events subcommand: %s\n", args[0])
	}
}

func runIssueEventsList(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("%s", "Usage: afctl issue events list <issue-id>")
	}

	issueID := args[0]

	events, err := c.ListEvents(ctx, issueID)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(events)
		return nil
	}
	if len(events) == 0 {
		fmt.Println("No events found.")
		return nil
	}
	for _, e := range events {
		fmt.Printf("Event ID:    %s\n", e.ID)
		fmt.Printf("Actor:       %s\n", e.Actor)
		fmt.Printf("Type:        %s\n", e.EventType)
		fmt.Printf("Payload:     %s\n", e.PayloadJSON)
		fmt.Printf("Created At:  %s\n\n", e.CreatedAt)
	}
	return nil
}
