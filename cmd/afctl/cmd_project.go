package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/core"
)

// ─── Project ────────────────────────────────────────────────────────────────

func runProject(ctx context.Context, c *client.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: afctl project <add|list>")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		runProjectAdd(ctx, c, args[1:])
	case "list":
		runProjectList(ctx, c)
	default:
		fmt.Fprintf(os.Stderr, "unknown project subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func runProjectAdd(ctx context.Context, c *client.Client, args []string) {
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

	project, err := c.CreateProject(ctx, key, name, description)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(project)
		return
	}
	printProject(project)
}

func runProjectList(ctx context.Context, c *client.Client) {
	projects, err := c.ListProjects(ctx)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(projects)
		return
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
