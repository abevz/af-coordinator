package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/abevz/af-coordinator/internal/build"
	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/config"
	"github.com/abevz/af-coordinator/internal/core"
)

var jsonOutput bool
var defaultActor string

func init() {
	defaultActor = os.Getenv("AF_COORDINATOR_ACTOR")
}

func main() {
	cfg := config.Default()

	// Parse global flags (--json, --actor) from os.Args before command dispatch.
	args := os.Args[1:]
	var filtered []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--actor":
			if i+1 < len(args) {
				defaultActor = args[i+1]
				i++
			}
		default:
			filtered = append(filtered, args[i])
		}
	}

	if len(filtered) < 1 {
		printUsage()
		os.Exit(1)
	}
	if filtered[0] == "-h" || filtered[0] == "--help" || filtered[0] == "help" {
		printUsage()
		return
	}

	c := client.New(cfg.SocketPath)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if shouldCheckDaemonRevision(filtered) {
		if h, err := c.Health(ctx); err == nil {
			if h.Revision != "" && h.Revision != "unknown" && build.Revision != "unknown" && h.Revision != build.Revision {
				fmt.Fprintf(os.Stderr, "afctl revision %s != daemon revision %s; restart af-coordinatord\n", shortRev(build.Revision), shortRev(h.Revision))
			}
		}
	}

	var err error
	switch filtered[0] {
	case "health":
		err = runHealth(ctx, c)
	case "doctor":
		err = runDoctor(ctx, c, cfg, filtered[1:])
	case "protocol":
		runProtocol()
	case "init":
		err = runInit(filtered[1:])
	case "project":
		err = runProject(ctx, c, filtered[1:])
	case "repo":
		err = runRepo(ctx, c, filtered[1:])
	case "worktree":
		err = runWorktree(ctx, c, filtered[1:])
	case "artifact-root":
		err = runArtifactRoot(ctx, c, filtered[1:])
	case "artifact":
		err = runArtifact(ctx, c, filtered[1:])
	case "export":
		err = runExport(ctx, c, filtered[1:])
	case "stats":
		err = runStats(ctx, c, filtered[1:])
	case "issue":
		err = runIssue(ctx, c, filtered[1:])
	case "ls":
		err = runLs(ctx, c, filtered[1:])
	case "show":
		err = runShow(ctx, c, filtered[1:])
	default:
		printUsage()
		os.Exit(1)
	}
	if err != nil {
		fail(err)
	}
}

func shouldCheckDaemonRevision(args []string) bool {
	if len(args) == 0 || args[0] == "init" || args[0] == "protocol" {
		return false
	}
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return false
		}
	}
	return true
}

func shortRev(rev string) string {
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: afctl [--json] [--actor <name>] <command>

Global flags:
  --json                Output in JSON format (default: human-readable)
  --actor <name>        Set the acting identity (default: AF_COORDINATOR_ACTOR env)

Commands:
  health                Check daemon health
  doctor                Run environment diagnostics
  protocol              Print the agent protocol contract
  init                  Initialize or update AGENTS.md with coordinator block
  project               Manage projects
    add                 Create a new project
    list                List all projects
  repo                  Manage repositories
    add                 Register a new repository
    list                List repositories
  worktree              Manage worktrees
    register            Register or update a worktree
    list                List worktrees
    unregister          Remove a safe-to-delete worktree record
    prune               Remove stale worktree records for missing paths
  artifact-root         Manage artifact roots
    add                 Register an artifact root in a repository
    list                List artifact roots
  artifact              Manage artifacts
    register            Register an artifact file
    list                List artifacts
  export                Export coordinator state
    jsonl               Stream normalized JSONL to stdout
  stats [filters]       Show read-only project execution statistics
  issue                 Manage issues
    create              Create a new issue
    get                 Get an issue by ID or short_id [--full]
    list                List issues with optional filters
    ready               List ready (actionable, unleased) issues
    claim               Claim an issue (acquire a lease)
    heartbeat           Extend an existing lease
    release             Release a claimed lease
    handoff             Add a required HANDOFF note and release atomically
    update              Update issue fields (title, description, priority, assignee, status)
    close               Close an issue (resolution: done or cancelled) [--branch name] [--pr-url URL] [--commit-sha SHA] [--note "text"]
    operator-close      Force-close an issue without a lease token (AF_OPERATOR_TOKEN + --reason)
    operator-reopen     Reopen a terminal issue without a lease token (AF_OPERATOR_TOKEN + --reason)
    operator-release    Force-clear a stuck in_progress lease and reopen without closing (AF_OPERATOR_TOKEN + --reason)
    link                Link an artifact to an issue
    note                Manage notes on an issue
      add              Add a note to an issue
      list             List notes on an issue
    events              Show activity timeline for an issue
      list             List events for an issue
    dependency          Manage issue dependencies
      add               Add a dependency between two issues
      remove            Remove a dependency between two issues
  ls [filters]           List issues (shortcut for issue list; use --help for filters)
  show <issue-id> [--full] Show issue details (shortcut for issue get)
`)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func fail(err error) {
	var clientErr *client.ClientError
	if errors.As(err, &clientErr) {
		if jsonOutput {
			resp := core.APIErrorResponse{Error: core.NewAPIError(clientErr.Code, clientErr.Message)}
			json.NewEncoder(os.Stderr).Encode(resp)
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", clientErr)
		}
		os.Exit(mapExitCode(clientErr.Code))
	}
	// Транспортная/не-API ошибка
	if jsonOutput {
		json.NewEncoder(os.Stderr).Encode(core.APIErrorResponse{
			Error: core.NewAPIError("internal_error", err.Error()),
		})
	} else {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	os.Exit(1)
}

func mapExitCode(code string) int {
	switch code {
	case core.ErrNotFound:
		return 5
	case core.ErrLeaseHeld:
		return 3
	case core.ErrLeaseExpired:
		return 4
	case core.ErrConflict:
		return 2
	case core.ErrDependencyCycle:
		return 6
	default:
		return 1
	}
}

func mapExitCodeErr(err error) int {
	if err == nil {
		return 0
	}
	var clientErr *client.ClientError
	if errors.As(err, &clientErr) {
		return mapExitCode(clientErr.Code)
	}
	return 1
}

func resolveActor(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if defaultActor != "" {
		return defaultActor, nil
	}
	if agent := getParentAgent(); agent != "" {
		return agent, nil
	}
	if sysUser := os.Getenv("USER"); sysUser != "" {
		return sysUser, nil
	}
	return "", fmt.Errorf("actor is required: set --actor flag or AF_COORDINATOR_ACTOR environment variable")
}

func getParentAgent() string {
	pid := os.Getppid()
	for pid > 1 {
		statPath := fmt.Sprintf("/proc/%d/stat", pid)
		data, err := os.ReadFile(statPath)
		if err != nil {
			break
		}
		s := string(data)
		idx1 := strings.IndexByte(s, '(')
		idx2 := strings.LastIndexByte(s, ')')
		if idx1 != -1 && idx2 != -1 && idx2 > idx1 {
			comm := s[idx1+1 : idx2]
			// Skip common shells and daemons
			if comm != "bash" && comm != "sh" && comm != "zsh" && comm != "tmux" && comm != "tmux: server" && comm != "su" && comm != "sudo" && comm != "sshd" && comm != "systemd" && comm != "init" {
				return fmt.Sprintf("%s-%d", comm, pid)
			}
			parts := strings.Split(s[idx2+2:], " ")
			if len(parts) >= 2 {
				ppid, _ := strconv.Atoi(parts[1])
				if ppid == 0 || ppid == pid {
					break
				}
				pid = ppid
				continue
			}
		}
		break
	}
	return ""
}

// ─── Health ──────────────────────────────────────────────────────────────────

func runHealth(ctx context.Context, c *client.Client) error {
	health, err := c.Health(ctx)
	if err != nil {
		return err
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(health)
		return nil
	}
	fmt.Printf("Name:       %s\n", health.Name)
	fmt.Printf("Status:     %s\n", health.Status)
	fmt.Printf("Revision:   %s\n", health.Revision)
	fmt.Printf("DBPath:     %s\n", health.DBPath)
	fmt.Printf("SocketPath: %s\n", health.SocketPath)
	fmt.Printf("Time:       %s\n", health.Time.UTC().Format(time.RFC3339))
	return nil
}

// runLs is a top-level shortcut for `afctl issue list`.
func runLs(ctx context.Context, c *client.Client, args []string) error {
	return runIssueList(ctx, c, args)
}

// runShow is a top-level shortcut for `afctl issue get`.
func runShow(ctx context.Context, c *client.Client, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: afctl show <issue-id-or-short-id> [--full]")
	}
	return runIssueGet(ctx, c, args)
}

func printIssue(i core.Issue) {
	fmt.Printf("ID:           %s\n", i.ID)
	fmt.Printf("Short ID:     %s\n", i.ShortID)
	fmt.Printf("Project ID:   %s\n", i.ProjectID)
	if i.RepositoryID != "" {
		fmt.Printf("Repository ID:%s\n", i.RepositoryID)
	}
	if i.WorktreeID != "" {
		fmt.Printf("Worktree ID:  %s\n", i.WorktreeID)
	}
	fmt.Printf("Scope:        %s\n", i.ScopeKind)
	fmt.Printf("Type:         %s\n", i.IssueType)
	fmt.Printf("Title:        %s\n", i.Title)
	if i.ExternalKey != "" {
		fmt.Printf("External Key: %s\n", i.ExternalKey)
	}
	if i.Description != "" {
		fmt.Printf("Description:  %s\n", i.Description)
	}
	if i.AcceptanceCriteria != "" {
		fmt.Printf("Acceptance:   %s\n", i.AcceptanceCriteria)
	}
	fmt.Printf("Status:       %s\n", i.Status)
	fmt.Printf("Priority:     %d\n", i.Priority)
	if i.Assignee != "" {
		fmt.Printf("Assignee:     %s\n", i.Assignee)
	}
	if i.Holder != "" {
		fmt.Printf("Holder:       %s\n", i.Holder)
	}
	fmt.Printf("Version:      %d\n", i.Version)
	if i.ClaimedAt != "" {
		fmt.Printf("Claimed At:   %s\n", i.ClaimedAt)
	}
	if i.ClosedAt != "" {
		fmt.Printf("Closed At:    %s\n", i.ClosedAt)
	}
	fmt.Printf("Created:      %s\n", i.CreatedAt)
	fmt.Printf("Updated:      %s\n\n", i.UpdatedAt)
}

// ─── Issue Display Helpers ───────────────────────────────────────────────────

// statusSymbol returns a Unicode symbol representing the issue status.
func statusSymbol(status string) string {
	switch status {
	case "open":
		return "○"
	case "in_progress":
		return "●"
	case "done":
		return "✓"
	case "cancelled":
		return "✗"
	case "blocked":
		return "⊘"
	case "deferred":
		return "◌"
	default:
		return "?"
	}
}

// truncate shortens a string to n characters, adding "..." if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-3]) + "..."
}

// printIssueDetailed displays a single issue with basic details.
func printIssueDetailed(i core.Issue, l *core.IssueLease) {
	fmt.Printf("ID:         %s\n", i.ID)
	fmt.Printf("Short ID:   %s\n", i.ShortID)
	fmt.Printf("Status:     %s %s\n", statusSymbol(i.Status), i.Status)
	fmt.Printf("Type:       %s\n", i.IssueType)
	fmt.Printf("Title:      %s\n", i.Title)
	if i.ExternalKey != "" {
		fmt.Printf("External:   %s\n", i.ExternalKey)
	}
	fmt.Printf("Priority:   %d\n", i.Priority)
	if i.Assignee != "" {
		fmt.Printf("Assignee:   %s\n", i.Assignee)
	} else {
		fmt.Printf("Assignee:   (unassigned)\n")
	}
	fmt.Printf("Scope:      %s\n", i.ScopeKind)
	fmt.Printf("Version:    %d\n", i.Version)
	if l != nil {
		fmt.Printf("Claimed:    %s (expires %s)\n", l.Holder, l.ExpiresAt)
		fmt.Printf("Attempt ID: %s\n", l.AttemptID)
		if l.SessionID != "" {
			fmt.Printf("Session ID: %s\n", l.SessionID)
		}
	} else {
		fmt.Printf("Claimed:    (not claimed)\n")
	}
	fmt.Printf("Created:    %s\n", i.CreatedAt)
	fmt.Printf("Updated:    %s\n", i.UpdatedAt)
}

// printIssueFull displays a single issue with all fields and full history.
func printIssueFull(i core.Issue, l *core.IssueLease, events []core.Event, notes []core.Note, links []core.ArtifactRef) {
	fmt.Printf("ID:            %s\n", i.ID)
	fmt.Printf("Short ID:      %s\n", i.ShortID)
	fmt.Printf("Status:        %s %s\n", statusSymbol(i.Status), i.Status)
	fmt.Printf("Type:          %s\n", i.IssueType)
	fmt.Printf("Title:         %s\n", i.Title)
	if i.ExternalKey != "" {
		fmt.Printf("External Key:  %s\n", i.ExternalKey)
	}
	if i.Description != "" {
		fmt.Printf("Description:   %s\n", i.Description)
	}
	if i.AcceptanceCriteria != "" {
		fmt.Printf("Acceptance:    %s\n", i.AcceptanceCriteria)
	}
	fmt.Printf("Priority:      %d\n", i.Priority)
	if i.Assignee != "" {
		fmt.Printf("Assignee:      %s\n", i.Assignee)
	} else {
		fmt.Printf("Assignee:      (unassigned)\n")
	}
	fmt.Printf("Project ID:    %s\n", i.ProjectID)
	if i.RepositoryID != "" {
		fmt.Printf("Repository ID: %s\n", i.RepositoryID)
	}
	if i.WorktreeID != "" {
		fmt.Printf("Worktree ID:   %s\n", i.WorktreeID)
	}
	fmt.Printf("Scope:         %s\n", i.ScopeKind)
	fmt.Printf("Version:       %d\n", i.Version)
	if l != nil {
		fmt.Printf("Claimed:       %s (expires %s)\n", l.Holder, l.ExpiresAt)
		fmt.Printf("Attempt ID:    %s\n", l.AttemptID)
		if l.SessionID != "" {
			fmt.Printf("Session ID:    %s\n", l.SessionID)
		}
	} else {
		fmt.Printf("Claimed:       (not claimed)\n")
	}
	if i.ClaimedAt != "" {
		fmt.Printf("Claimed At:    %s\n", i.ClaimedAt)
	}
	if i.ClosedAt != "" {
		fmt.Printf("Closed At:     %s\n", i.ClosedAt)
	}
	fmt.Printf("Created:       %s\n", i.CreatedAt)
	fmt.Printf("Updated:       %s\n", i.UpdatedAt)

	// A "blocks" edge is rendered from the blocked side as "Blocked By" and from
	// the blocking side as "Blocks"; it is intentionally omitted from this raw
	// list so the direction is never shown ambiguously (e.g. "blocks aion-190"
	// for an edge that actually means "blocked by aion-190").
	nonBlockDeps := make([]core.Dependency, 0, len(i.Dependencies))
	for _, dep := range i.Dependencies {
		if dep.Kind == "blocks" {
			continue
		}
		nonBlockDeps = append(nonBlockDeps, dep)
	}
	if len(nonBlockDeps) > 0 {
		fmt.Printf("\nDependencies:\n")
		for _, dep := range nonBlockDeps {
			label := dep.DependsOnShortID
			if label == "" {
				label = dep.DependsOnID
			}
			fmt.Printf("  - %s %s [%s]\n", dep.Kind, label, dep.DependsOnID)
		}
	}
	if i.Blocked || len(i.BlockedBy) > 0 {
		if len(i.BlockedBy) > 0 {
			fmt.Printf("Blocked:       yes\n")
			fmt.Printf("Blocked By:    %s\n", strings.Join(i.BlockedBy, ", "))
		} else {
			// Blocked with no dependency edge: the issue's own status is "blocked".
			fmt.Printf("Blocked:       yes (status)\n")
		}
	}
	if len(i.Blocks) > 0 {
		fmt.Printf("Blocks:        %s\n", strings.Join(i.Blocks, ", "))
	}

	if len(links) > 0 {
		fmt.Printf("\nLinks:\n")
		for _, link := range links {
			fmt.Printf("  - %s (%s, %s)\n", link.RelativePath, link.Kind, link.Relation)
		}
	}

	if len(events) > 0 {
		fmt.Printf("\nHistory:\n")
		for _, e := range events {
			fmt.Printf("  [%s] %s by %s\n", e.CreatedAt, e.EventType, e.Actor)

			// If it's a note_added event, try to find and print the note body.
			if e.EventType == "note_added" {
				var noteBody string
				for _, n := range notes {
					// Correlate note with event by timestamp and actor
					if n.CreatedAt == e.CreatedAt && n.Author == e.Actor {
						noteBody = n.Body
						break
					}
				}
				if noteBody != "" {
					fmt.Printf("    Note: %s\n", noteBody)
				}
			} else if e.PayloadJSON != "" && e.PayloadJSON != "{}" {
				fmt.Printf("    %s\n", e.PayloadJSON)
			}
		}
	}
}

// printIssuesTable displays a list of issues in a fixed-width table format.
func printIssuesTable(issues []core.Issue) {
	const (
		idWidth        = 10
		shortWidth     = 10
		statusWidth    = 13
		typeWidth      = 8
		titleWidth     = 42
		assigneeWidth  = 10
		claimedWidth   = 10
		blockedByWidth = 18
		depsWidth      = 34
	)
	format := "%-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s\n"
	fmt.Printf(format, idWidth, "ID", shortWidth, "SHORT", statusWidth, "STATUS", typeWidth, "TYPE", titleWidth, "TITLE", assigneeWidth, "ASSIGNEE", claimedWidth, "CLAIMED", blockedByWidth, "BLOCKED BY", depsWidth, "DEPS")
	fmt.Printf(format, idWidth, "---", shortWidth, "-----", statusWidth, "------", typeWidth, "----", titleWidth, "-----", assigneeWidth, "-------", claimedWidth, "-------", blockedByWidth, "----------", depsWidth, "----")
	for _, i := range issues {
		id := truncate(i.ID, idWidth)
		title := truncate(i.Title, titleWidth)
		status := statusSymbol(i.Status) + " " + i.Status
		if i.Blocked && i.Status != "blocked" {
			status += " [B]"
		}
		shortID := truncate(i.ShortID, shortWidth)
		status = truncate(status, statusWidth)
		issueType := truncate(i.IssueType, typeWidth)
		assignee := truncate(i.Assignee, assigneeWidth)
		claimed := truncate(i.Holder, claimedWidth)
		blockedBy := truncate(strings.Join(i.BlockedBy, ","), blockedByWidth)
		dependencies := truncate(formatIssueDependencies(i.Dependencies), depsWidth)
		fmt.Printf(format, idWidth, id, shortWidth, shortID, statusWidth, status, typeWidth, issueType, titleWidth, title, assigneeWidth, assignee, claimedWidth, claimed, blockedByWidth, blockedBy, depsWidth, dependencies)
	}
}

func formatIssueDependencies(dependencies []core.Dependency) string {
	labels := make([]string, 0, len(dependencies))
	for _, dep := range dependencies {
		if dep.Kind == "blocks" {
			continue
		}
		label := dep.DependsOnShortID
		if label == "" {
			label = dep.DependsOnID
		}
		if dep.Kind != "" {
			label = dep.Kind + ":" + label
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, ",")
}

func printWorktree(wt core.Worktree) {
	mainStr := ""
	if wt.IsMain {
		mainStr = " (main)"
	}
	ephemeralStr := ""
	if wt.IsEphemeral {
		ephemeralStr = " [ephemeral]"
	}
	fmt.Printf("ID:           %s%s%s\n", wt.ID, mainStr, ephemeralStr)
	fmt.Printf("Repository ID:%s\n", wt.RepositoryID)
	fmt.Printf("Path:         %s\n", wt.AbsolutePath)
	fmt.Printf("Branch:       %s\n", wt.Branch)
	if wt.HeadCommit != "" {
		fmt.Printf("Head:         %s\n", wt.HeadCommit)
	}
	if wt.RemoteName != "" {
		fmt.Printf("Remote:       %s/%s\n", wt.RemoteName, wt.RemoteBranch)
	}
	fmt.Printf("Last Seen:    %s\n", wt.LastSeenAt)
	fmt.Printf("Created:      %s\n", wt.CreatedAt)
	fmt.Printf("Updated:      %s\n\n", wt.UpdatedAt)
}
