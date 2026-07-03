package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/config"
	"github.com/abevz/af-coordinator/internal/core"
)

func main() {
	cfg := config.Default()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	c := client.New(cfg.SocketPath)

	switch os.Args[1] {
	case "health":
		runHealth(c)
	case "project":
		runProject(c, os.Args[2:])
	case "repo":
		runRepo(c, os.Args[2:])
	case "worktree":
		runWorktree(c, os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: afctl <command>

Commands:
  health                Check daemon health
  project               Manage projects
    add                 Create a new project
    list                List all projects
  repo                  Manage repositories
    add                 Register a new repository
    list                List repositories
  worktree              Manage worktrees
    register            Register or update a worktree
    list                List worktrees
`)
}

// ─── Health ──────────────────────────────────────────────────────────────────

func runHealth(c *client.Client) {
	health, err := c.Health()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Name:       %s\n", health.Name)
	fmt.Printf("Status:     %s\n", health.Status)
	fmt.Printf("DBPath:     %s\n", health.DBPath)
	fmt.Printf("SocketPath: %s\n", health.SocketPath)
	fmt.Printf("Time:       %s\n", health.Time.UTC().Format(time.RFC3339))
}

// ─── Project ────────────────────────────────────────────────────────────────

func runProject(c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl project <add|list>")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runProjectAdd(c, args[1:])
	case "list":
		runProjectList(c)
	default:
		fmt.Fprintf(os.Stderr, "unknown project subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runProjectAdd(c *client.Client, args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: afctl project add --key <key> --name <name> [--description <desc>]")
		os.Exit(1)
	}

	var key, name, description string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--key":
			if i+1 < len(args) {
				key = args[i+1]
				i++
			}
		case "--name":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--description":
			if i+1 < len(args) {
				description = args[i+1]
				i++
			}
		}
	}

	project, err := c.CreateProject(key, name, description)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printProject(project)
}

func runProjectList(c *client.Client) {
	projects, err := c.ListProjects()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return
	}
	for _, p := range projects {
		printProject(p)
	}
}

func printProject(p core.Project) {
	fmt.Printf("ID:          %s\n", p.ID)
	fmt.Printf("Key:         %s\n", p.Key)
	fmt.Printf("Name:        %s\n", p.Name)
	fmt.Printf("Description: %s\n", p.Description)
	fmt.Printf("Created:     %s\n", p.CreatedAt)
	fmt.Printf("Updated:     %s\n\n", p.UpdatedAt)
}

// ─── Repo ───────────────────────────────────────────────────────────────────

func runRepo(c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl repo <add|list>")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runRepoAdd(c, args[1:])
	case "list":
		runRepoList(c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown repo subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runRepoAdd(c *client.Client, args []string) {
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: afctl repo add --project <key> --logical-name <name> --canonical-git-dir <path> [--default-branch <branch>] [--remotes '<json>']")
		os.Exit(1)
	}

	var req core.CreateRepoRequest
	req.DefaultBranch = "main"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				req.Project = args[i+1]
				i++
			}
		case "--logical-name":
			if i+1 < len(args) {
				req.LogicalName = args[i+1]
				i++
			}
		case "--canonical-git-dir":
			if i+1 < len(args) {
				req.CanonicalGitDir = args[i+1]
				i++
			}
		case "--default-branch":
			if i+1 < len(args) {
				req.DefaultBranch = args[i+1]
				i++
			}
		case "--remotes":
			if i+1 < len(args) {
				if err := json.Unmarshal([]byte(args[i+1]), &req.Remotes); err != nil {
					fmt.Fprintf(os.Stderr, "error: invalid --remotes JSON: %v\n", err)
					os.Exit(1)
				}
				i++
			}
		}
	}

	repo, remotes, err := c.CreateRepo(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Repository ID: %s\n", repo.ID)
	fmt.Printf("Project ID:    %s\n", repo.ProjectID)
	fmt.Printf("Logical Name:  %s\n", repo.LogicalName)
	fmt.Printf("Git Dir:       %s\n", repo.CanonicalGitDir)
	fmt.Printf("Default Branch:%s\n", repo.DefaultBranch)
	if len(remotes) > 0 {
		fmt.Println("Remotes:")
		for _, r := range remotes {
			primary := ""
			if r.IsPrimary {
				primary = " (primary)"
			}
			fmt.Printf("  - %s -> %s%s\n", r.RemoteName, r.FetchURL, primary)
		}
	}
	fmt.Println()
}

func runRepoList(c *client.Client, args []string) {
	project := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--project" && i+1 < len(args) {
			project = args[i+1]
		}
	}

	repos, err := c.ListRepos(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(repos) == 0 {
		fmt.Println("No repositories found.")
		return
	}
	for _, r := range repos {
		fmt.Printf("ID:           %s\n", r.ID)
		fmt.Printf("Project ID:   %s\n", r.ProjectID)
		fmt.Printf("Logical Name: %s\n", r.LogicalName)
		fmt.Printf("Git Dir:      %s\n", r.CanonicalGitDir)
		fmt.Printf("Branch:       %s\n", r.DefaultBranch)
		fmt.Printf("Created:      %s\n\n", r.CreatedAt)
	}
}

// ─── Worktree ───────────────────────────────────────────────────────────────

func runWorktree(c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl worktree <register|list>")
		os.Exit(1)
	}

	switch args[0] {
	case "register":
		runWorktreeRegister(c, args[1:])
	case "list":
		runWorktreeList(c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown worktree subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runWorktreeRegister(c *client.Client, args []string) {
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: afctl worktree register --repo <repo-id> --absolute-path <path> [--branch <branch>] [--head-commit <sha>] [--remote-name <name>] [--remote-branch <branch>] [--main] [--ephemeral]")
		os.Exit(1)
	}

	var req core.CreateWorktreeRequest
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 < len(args) {
				req.Repo = args[i+1]
				i++
			}
		case "--absolute-path":
			if i+1 < len(args) {
				req.AbsolutePath = args[i+1]
				i++
			}
		case "--branch":
			if i+1 < len(args) {
				req.Branch = args[i+1]
				i++
			}
		case "--head-commit":
			if i+1 < len(args) {
				req.HeadCommit = args[i+1]
				i++
			}
		case "--remote-name":
			if i+1 < len(args) {
				req.RemoteName = args[i+1]
				i++
			}
		case "--remote-branch":
			if i+1 < len(args) {
				req.RemoteBranch = args[i+1]
				i++
			}
		case "--main":
			req.IsMain = true
		case "--ephemeral":
			req.IsEphemeral = true
		}
	}

	wt, err := c.RegisterWorktree(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printWorktree(wt)
}

func runWorktreeList(c *client.Client, args []string) {
	repo := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--repo" && i+1 < len(args) {
			repo = args[i+1]
		}
	}

	worktrees, err := c.ListWorktrees(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(worktrees) == 0 {
		fmt.Println("No worktrees found.")
		return
	}
	for _, wt := range worktrees {
		printWorktree(wt)
	}
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
