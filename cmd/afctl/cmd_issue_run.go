package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/abevz/af-coordinator/internal/client"
	"github.com/abevz/af-coordinator/internal/core"
)

const issueRunUsage = "Usage: afctl issue run <issue-id> [--actor <name>] [--ttl <seconds>] [--close-resolution done|cancelled] [--branch <name>] [--pr-url <url>] [--commit-sha <sha>] [--note <text>] -- <command> [args...]\n" + lifecycleHint +
	"\nOwns claim -> heartbeat -> close/handoff around a single subprocess, so the lease token never leaves this process's memory: it cannot be lost the way a multi-step script can lose it before persisting it. On exit 0, closes with --close-resolution (default done). On any other exit, or on Ctrl-C, hands the lease off with an auto-generated HANDOFF: note instead of closing."

// runIssueRun claims issueID, execs the given command with the lease
// exported as environment variables, heartbeats in the background for the
// duration of the run, and closes or hands off the issue based on how the
// command exited. See issueRunUsage for the full contract.
func runIssueRun(ctx context.Context, c *client.Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println(issueRunUsage)
		return nil
	}

	sepIdx := -1
	for i, a := range args {
		if a == "--" {
			sepIdx = i
			break
		}
	}
	if sepIdx == -1 {
		return usageErr(issueRunUsage, "missing -- separator before the command to run")
	}
	flagArgs := args[:sepIdx]
	cmdArgs := args[sepIdx+1:]
	if len(cmdArgs) == 0 {
		return usageErr(issueRunUsage, "no command given after --")
	}
	if len(flagArgs) < 1 {
		return usageErr(issueRunUsage, "")
	}

	issueID := flagArgs[0]
	actor := ""
	ttl := 900
	closeResolution := "done"
	var branch, prURL, commitSHA, note string

	for i := 1; i < len(flagArgs); i++ {
		switch flagArgs[i] {
		case "--actor":
			if i+1 < len(flagArgs) {
				actor = flagArgs[i+1]
				i++
			}
		case "--ttl":
			if i+1 < len(flagArgs) {
				fmt.Sscanf(flagArgs[i+1], "%d", &ttl)
				i++
			}
		case "--close-resolution":
			if i+1 < len(flagArgs) {
				closeResolution = flagArgs[i+1]
				i++
			}
		case "--branch":
			if i+1 < len(flagArgs) {
				branch = flagArgs[i+1]
				i++
			}
		case "--pr-url":
			if i+1 < len(flagArgs) {
				prURL = flagArgs[i+1]
				i++
			}
		case "--commit-sha":
			if i+1 < len(flagArgs) {
				commitSHA = flagArgs[i+1]
				i++
			}
		case "--note":
			if i+1 < len(flagArgs) {
				note = flagArgs[i+1]
				i++
			}
		default:
			return usageErr(issueRunUsage, fmt.Sprintf("unknown flag: %s", flagArgs[i]))
		}
	}
	if closeResolution != "done" && closeResolution != "cancelled" {
		return usageErr(issueRunUsage, "--close-resolution must be done or cancelled")
	}
	if ttl <= 0 {
		return usageErr(issueRunUsage, "--ttl must be positive")
	}

	holder, err := resolveActor(actor)
	if err != nil {
		return usageErr(issueRunUsage, err.Error())
	}

	claim, err := c.ClaimIssueWithSession(ctx, issueID, holder, ttl, "")
	if err != nil {
		fail(err)
	}
	fmt.Printf("Claimed %s (version %d, expires %s)\n", issueID, claim.Version, claim.ExpiresAt)

	heartbeatInterval := time.Duration(ttl) * time.Second / 3
	if heartbeatInterval < 5*time.Second {
		heartbeatInterval = 5 * time.Second
	}
	hbCtx, hbCancel := context.WithCancel(context.Background())
	defer hbCancel()
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				if _, err := c.HeartbeatLease(context.Background(), issueID, claim.LeaseToken, ttl); err != nil {
					fmt.Fprintf(os.Stderr, "issue run: heartbeat failed: %v\n", err)
				}
			}
		}
	}()

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"AF_LEASE_TOKEN="+claim.LeaseToken,
		"AF_ATTEMPT_ID="+claim.AttemptID,
		"AF_ISSUE_ID="+issueID,
		fmt.Sprintf("AF_EXPECTED_VERSION=%d", claim.Version),
	)
	// exec.CommandContext's default cancellation is an immediate SIGKILL;
	// give the child a chance to clean up with SIGTERM first.
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second

	runErr := cmd.Run()
	hbCancel()

	background := context.Background()

	if runErr == nil {
		result, err := c.CloseIssue(background, issueID, core.CloseIssueRequest{
			Resolution:      closeResolution,
			Branch:          branch,
			PRURL:           prURL,
			CommitSHA:       commitSHA,
			ExpectedVersion: claim.Version,
			LeaseToken:      claim.LeaseToken,
			Actor:           holder,
			Note:            note,
		})
		if err != nil {
			return fmt.Errorf("command succeeded but close failed: %w", err)
		}
		if jsonOutput {
			json.NewEncoder(os.Stdout).Encode(result)
			return nil
		}
		fmt.Println("Issue closed.")
		return nil
	}

	exitCode := 1
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	handoffNote := fmt.Sprintf("HANDOFF: issue run command failed (exit %d)", exitCode)
	if ctx.Err() != nil {
		handoffNote = "HANDOFF: issue run cancelled"
	}
	if _, err := c.HandoffLease(background, issueID, claim.LeaseToken, handoffNote); err != nil {
		return fmt.Errorf("command failed (%v) and handoff also failed: %w", runErr, err)
	}
	fmt.Fprintf(os.Stderr, "issue run: command failed, lease handed off with note: %s\n", handoffNote)
	os.Exit(exitCode)
	return nil // unreachable
}
