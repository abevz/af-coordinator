package update

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const ExitAvailable = 10

type Command struct {
	Version           string
	Source            Source
	Cache             Cache
	Installer         Installer
	Out, Err          io.Writer
	In                io.Reader
	Interactive, JSON bool
	Restart           func(context.Context, string) error
}

func (c Command) Run(ctx context.Context, args []string) (int, error) {
	var check, rollback, pre, yes, restart bool
	for _, a := range args {
		switch a {
		case "--check":
			check = true
		case "--rollback":
			rollback = true
		case "--prerelease":
			pre = true
		case "--yes":
			yes = true
		case "--restart":
			restart = true
		default:
			return 1, fmt.Errorf("unknown update flag %s", a)
		}
	}
	if check && (rollback || yes || restart) || rollback && pre {
		return 1, fmt.Errorf("--check cannot combine with upgrade/rollback options; --rollback cannot combine with --prerelease")
	}
	reader := bufio.NewReader(c.In)
	confirm := func(question string) bool {
		if !c.Interactive {
			return false
		}
		fmt.Fprint(c.Err, question+" [y/N] ")
		s, e := reader.ReadString('\n')
		return e == nil && (strings.EqualFold(strings.TrimSpace(s), "y") || strings.EqualFold(strings.TrimSpace(s), "yes"))
	}
	if rollback {
		if e := c.Installer.Rollback(); e != nil {
			return 1, e
		}
		v, e := c.Installer.CurrentVersion()
		if e != nil {
			return 1, e
		}
		if !c.JSON {
			fmt.Fprintf(c.Out, "Restored previous binaries (%s).\n", v)
		}
		if restart || confirm("Restart dibsd with the restored binaries?") {
			if c.Restart == nil {
				return 1, fmt.Errorf("daemon restart unavailable")
			}
			if e = c.Restart(ctx, v); e != nil {
				return 1, fmt.Errorf("rollback restored binaries but daemon restart failed: %w", e)
			}
		}
		if c.JSON {
			e = json.NewEncoder(c.Out).Encode(map[string]string{"status": "rolled_back", "version": v})
		}
		return 0, e
	}
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	r, e := c.Cache.Refresh(checkCtx, c.Source, c.Version, pre, true, time.Now())
	cancel()
	if e != nil {
		return 1, e
	}
	printResult := func() error {
		if c.JSON {
			return json.NewEncoder(c.Out).Encode(r)
		}
		fmt.Fprintf(c.Out, "Installed: %s\nLatest:    %s\n", r.Installed, r.Latest)
		if r.Available {
			fmt.Fprintln(c.Out, "Update available.")
			if r.Breaking {
				fmt.Fprintln(c.Out, "Breaking changes — confirmation required before upgrading.")
			}
			fmt.Fprintln(c.Out, r.Changelog)
		} else {
			fmt.Fprintln(c.Out, "Up to date (no newer release).")
		}
		return nil
	}
	if check || !r.Available {
		if e = printResult(); e != nil {
			return 1, e
		}
		if r.Available {
			return ExitAvailable, nil
		}
		return 0, nil
	}
	if !c.JSON {
		if e = printResult(); e != nil {
			return 1, e
		}
	}
	if r.Breaking && !yes && !confirm("This release has breaking changes. Have you migrated consumers and want to upgrade?") {
		return 1, fmt.Errorf("breaking changes require confirmation or --yes")
	}
	if e = c.Installer.Upgrade(ctx, r); e != nil {
		return 1, e
	}
	if !c.JSON {
		fmt.Fprintf(c.Out, "Installed %s into %s; previous binaries retained.\n", r.Latest, c.Installer.Bindir)
	}
	restarted := restart || confirm("Restart dibsd now?")
	if restarted {
		if c.Restart == nil {
			return 1, fmt.Errorf("daemon restart unavailable")
		}
		if e = c.Restart(ctx, r.Latest); e != nil {
			return 1, fmt.Errorf("binaries upgraded; daemon restart failed (dibs update --rollback is available): %w", e)
		}
	} else if !c.JSON {
		fmt.Fprintln(c.Out, "Restart dibsd explicitly when ready; the running daemon keeps its previous version.")
	}
	if c.JSON {
		e = json.NewEncoder(c.Out).Encode(map[string]any{"status": "updated", "version": r.Latest, "restarted": restarted})
	}
	return 0, e
}
