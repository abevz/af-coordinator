package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/core"
)

// ─── Worktree ───────────────────────────────────────────────────────────────

func runWorktree(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl worktree <register|list>")
		os.Exit(1)
	}

	switch args[0] {
	case "register":
		runWorktreeRegister(ctx, c, args[1:])
	case "list":
		runWorktreeList(ctx, c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown worktree subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runWorktreeRegister(ctx context.Context, c *client.Client, args []string) {
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

	wt, err := c.RegisterWorktree(ctx, req)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(wt)
		return
	}
	printWorktree(wt)
}

func runWorktreeList(ctx context.Context, c *client.Client, args []string) {
	repo := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--repo" && i+1 < len(args) {
			repo = args[i+1]
		}
	}

	worktrees, err := c.ListWorktrees(ctx, repo)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(worktrees)
		return
	}
	if len(worktrees) == 0 {
		fmt.Println("No worktrees found.")
		return
	}
	for _, wt := range worktrees {
		printWorktree(wt)
	}
}
