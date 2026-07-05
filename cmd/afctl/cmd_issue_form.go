package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/core"
	"github.com/charmbracelet/huh"
)

func runIssueCreateForm(ctx context.Context, c *client.Client, args []string) error {
	actor, err := resolveActor("")
	if err != nil {
		return fmt.Errorf("error: %v\n", err)
	}

	projects, err := c.ListProjects(ctx)
	if err != nil {
		return fmt.Errorf("fetch projects: %w", err)
	}
	if len(projects) == 0 {
		return fmt.Errorf("no projects found, please create a project first")
	}

	projectOpts := make([]huh.Option[string], len(projects))
	for i, p := range projects {
		projectOpts[i] = huh.NewOption(p.Key, p.Key)
	}

	var project, scope, repo, worktree, title, priorityStr string
	var description, assignee string
	var dependsOnStr, artifactLink string
	var confirm bool

	// Screen 1: Project & Scope & Title & Priority
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Project").Options(projectOpts...).Value(&project),
			huh.NewSelect[string]().Title("Scope").Options(
				huh.NewOption("Project", "project"),
				huh.NewOption("Repository", "repository"),
				huh.NewOption("Worktree", "worktree"),
			).Value(&scope),
			huh.NewInput().Title("Title").Value(&title).Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("title is required")
				}
				return nil
			}),
			huh.NewSelect[string]().Title("Priority").Options(
				huh.NewOption("1 (High)", "1"),
				huh.NewOption("2 (Normal)", "2"),
				huh.NewOption("3 (Low)", "3"),
				huh.NewOption("4 (Lowest)", "4"),
			).Value(&priorityStr),
		).Title("Screen 1: Context & Basics"),
	).Run()
	
	if err != nil {
		return err // User cancelled
	}

	// Dynamic fetching for repo/worktree
	if scope == "repository" || scope == "worktree" {
		repos, err := c.ListRepos(ctx, project)
		if err != nil {
			return fmt.Errorf("fetch repos: %w", err)
		}
		if len(repos) == 0 {
			return fmt.Errorf("no repos found in project %s", project)
		}
		repoOpts := make([]huh.Option[string], len(repos))
		for i, r := range repos {
			repoOpts[i] = huh.NewOption(r.LogicalName, r.LogicalName)
		}

		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().Title("Repository").Options(repoOpts...).Value(&repo),
			).Title("Screen 1a: Select Repository"),
		).Run()
		if err != nil {
			return err
		}

		if scope == "worktree" {
			wts, err := c.ListWorktrees(ctx, repo)
			if err != nil {
				return fmt.Errorf("fetch worktrees: %w", err)
			}
			if len(wts) == 0 {
				return fmt.Errorf("no worktrees found in repo %s", repo)
			}
			wtOpts := make([]huh.Option[string], len(wts))
			for i, w := range wts {
				label := fmt.Sprintf("%s (%s)", w.Branch, w.ID[:8])
				wtOpts[i] = huh.NewOption(label, w.ID)
			}
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().Title("Worktree").Options(wtOpts...).Value(&worktree),
				).Title("Screen 1b: Select Worktree"),
			).Run()
			if err != nil {
				return err
			}
		}
	}

	// Screen 2: Details
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewText().Title("Description").Value(&description).Lines(5),
			huh.NewInput().Title("Assignee (optional)").Value(&assignee),
		).Title("Screen 2: Details"),
	).Run()
	if err != nil {
		return err
	}

	// Screen 3: Dependencies & Confirmation
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Depends On (comma separated short_ids, e.g. afc-1, utils-2)").Value(&dependsOnStr),
			huh.NewInput().Title("Artifact Link (relative path or UUID)").Value(&artifactLink),
		).Title("Screen 3: Dependencies & Links"),
		huh.NewGroup(
			huh.NewConfirm().Title("Create Issue?").Value(&confirm),
		),
	).Run()
	if err != nil {
		return err
	}

	if !confirm {
		fmt.Println("Cancelled.")
		return nil
	}

	priority, _ := strconv.Atoi(priorityStr)

	req := core.CreateIssueRequest{
		Project:     project,
		ScopeKind:   scope,
		Repo:        repo,
		Worktree:    worktree,
		Title:       title,
		Description: description,
		Priority:    priority,
		Actor:       actor,
	}

	issue, err := c.CreateIssue(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create issue: %w", err)
	}

	fmt.Printf("Created issue: %s (%s)\n", issue.ShortID, issue.ID)

	// Post-create steps
	if assignee != "" {
		// To safely update assignee without conflict, use the version from creation
		_, err := c.UpdateIssue(ctx, issue.ID, core.UpdateIssueRequest{
			Assignee:        assignee,
			ExpectedVersion: issue.Version,
			Actor:           actor,
		})
		if err != nil {
			fmt.Printf("Warning: failed to set assignee: %v\n", err)
		} else {
			fmt.Printf("Set assignee to: %s\n", assignee)
		}
	}

	if dependsOnStr != "" {
		deps := strings.Split(dependsOnStr, ",")
		for _, dep := range deps {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			err := c.AddDependency(ctx, issue.ID, core.AddDependencyRequest{
				DependsOn: dep,
				Kind:      "blocks", // default for form
				Actor:     actor,
			})
			if err != nil {
				fmt.Printf("Warning: failed to add dependency %s: %v\n", dep, err)
			} else {
				fmt.Printf("Added dependency: %s\n", dep)
			}
		}
	}

	if artifactLink != "" {
		err := c.LinkArtifact(ctx, issue.ID, core.LinkArtifactRequest{
			Artifact: artifactLink,
			Relation: "implements",
		})
		if err != nil {
			fmt.Printf("Warning: failed to link artifact %s: %v\n", artifactLink, err)
		} else {
			fmt.Printf("Linked artifact: %s\n", artifactLink)
		}
	}

	return nil
}
