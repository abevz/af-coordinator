package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/abevz/dibs/internal/build"
	"github.com/abevz/dibs/internal/config"
	"github.com/abevz/dibs/internal/update"
	"github.com/charmbracelet/x/term"
)

func runUpdate(ctx context.Context, cfg config.Config, args []string) (int, error) {
	source := update.DefaultSource()
	command := update.Command{Version: build.Version, Source: source, Cache: update.DefaultCache(), Installer: update.Installer{Bindir: update.DefaultBindir(), Source: source}, Out: os.Stdout, Err: os.Stderr, In: os.Stdin, JSON: jsonOutput, Interactive: !jsonOutput && term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stderr.Fd()), Restart: func(ctx context.Context, version string) error {
		return update.RestartDaemon(ctx, cfg.SocketPath, version)
	}}
	return command.Run(ctx, args)
}

func updateNotice(footer bool) string {
	return update.DefaultCache().Notice(update.NoticeOptions{Current: build.Version, Args: os.Args[1:], TTY: term.IsTerminal(os.Stderr.Fd()), JSON: jsonOutput, Footer: footer, NoNotifier: os.Getenv("DIBS_NO_UPDATE_NOTIFIER") == "1", Child: os.Getenv("DIBS_ISSUE_ID") != "" || os.Getenv("DIBS_LEASE_TOKEN") != "" || os.Getenv("DIBS_LEASE_TOKEN_FILE") != "", Now: time.Now()})
}

func printUpdateNotice() {
	if line := updateNotice(false); line != "" {
		fmt.Fprintln(os.Stderr, line)
	}
}
