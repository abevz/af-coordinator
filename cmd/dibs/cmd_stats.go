package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/abevz/dibs/internal/client"
	"github.com/abevz/dibs/internal/report"
)

const statsUsage = `Usage: dibs stats [filters]

Filters:
  --project <key>                    Limit to one project
  --repo <repository-id-or-name>      Limit to one repository
  --since <RFC3339|duration>          Start of the flow window, e.g. 24h
  --by actor                        Group outcomes by normalized claim owner
  --top <N>                         Costliest issues by claims (N > 0)
  --until <RFC3339>                   End of the flow window (default: now)
`

func runStats(ctx context.Context, c *client.Client, args []string) error {
	query, help, err := parseStatsArgs(args)
	if err != nil {
		return err
	}
	if help {
		fmt.Fprint(os.Stdout, statsUsage)
		return nil
	}

	stats, err := c.GetStats(ctx, query)
	if err != nil {
		fail(err)
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(stats)
	}
	printStats(stats)
	return nil
}

func parseStatsArgs(args []string) (report.Query, bool, error) {
	var query report.Query
	for i := 0; i < len(args); i++ {
		flag := args[i]
		if flag == "--help" || flag == "-h" {
			return report.Query{}, true, nil
		}
		switch flag {
		case "--project", "--repo", "--since", "--until", "--by", "--top":
		default:
			return report.Query{}, false, fmt.Errorf("unknown flag: %s", flag)
		}
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return report.Query{}, false, fmt.Errorf("%s requires a value", flag)
		}
		value := args[i+1]
		switch flag {
		case "--project":
			query.Project = value
		case "--repo":
			query.Repo = value
		case "--since":
			query.Since = value
		case "--until":
			query.Until = value
		case "--by":
			if value != "actor" {
				return report.Query{}, false, fmt.Errorf("--by must be actor")
			}
			query.By = value
		case "--top":
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return report.Query{}, false, fmt.Errorf("--top must be a positive integer")
			}
			query.Top = n
		}
		i++
	}
	return query, false, nil
}

func printStats(stats report.Report) {
	writeStats(os.Stdout, stats)
}

func writeStats(w io.Writer, stats report.Report) {
	window := stats.Window.Until
	if stats.Window.Since != "" {
		window = stats.Window.Since + " to " + stats.Window.Until
	}
	fmt.Fprintf(w, "Execution statistics (%s)\n", stats.Version)
	fmt.Fprintf(w, "Window: %s\n", window)
	if stats.Scope.ProjectKey != "" {
		fmt.Fprintf(w, "Project: %s\n", stats.Scope.ProjectKey)
	}
	if stats.Scope.RepositoryName != "" {
		fmt.Fprintf(w, "Repository: %s (%s)\n", stats.Scope.RepositoryName, stats.Scope.RepositoryID)
	}
	fmt.Fprintf(w, "\nInventory\n  %d total, %d ready, %d in progress\n", stats.Inventory.Total, stats.Inventory.Ready, stats.Inventory.InProgress)
	statuses := make([]string, 0, len(stats.Inventory.ByStatus))
	for status := range stats.Inventory.ByStatus {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		fmt.Fprintf(w, "  %-12s %d\n", status+":", stats.Inventory.ByStatus[status])
	}
	if stats.ByProject != nil {
		writeProjectStats(w, stats.ByProject, stats.Inventory.Total)
	}

	fmt.Fprintf(w, "\nFlow\n  %d created, %d closed, %d cancelled, %d reopened\n", stats.Flow.Created, stats.Flow.Closed, stats.Flow.Cancelled, stats.Flow.Reopened)
	writePercentiles(w, "  Lead time", stats.Flow.LeadTime)
	fmt.Fprintln(w, "\nAttempts")
	writePercentiles(w, "  Duration", stats.Attempts.Duration)
	fmt.Fprintf(w, "  %d claims, %d completed, %d/%d multi-attempt issues (%.1f%%)\n",
		stats.Attempts.Claims,
		stats.Attempts.Completed,
		stats.Attempts.Churn.Numerator,
		stats.Attempts.Churn.Denominator,
		stats.Attempts.Churn.Ratio*100,
	)
	outcomes := make([]string, 0, len(stats.Attempts.Outcomes))
	for outcome := range stats.Attempts.Outcomes {
		outcomes = append(outcomes, outcome)
	}
	sort.Strings(outcomes)
	fmt.Fprint(w, "  Outcomes:")
	for _, outcome := range outcomes {
		fmt.Fprintf(w, " %s=%d", outcome, stats.Attempts.Outcomes[outcome])
	}
	fmt.Fprintln(w)
	writeActorStats(w, stats.ByActor)
	writeIssueCosts(w, stats.TopIssues)
	fmt.Fprintln(w, "\nQuality")
	fmt.Fprintf(w, "  Handoff: %d/%d releases (%.1f%%)\n", stats.Handoff.Numerator, stats.Handoff.Denominator, stats.Handoff.Ratio*100)
	fmt.Fprintf(w, "  Coverage: notes %d/%d (%.1f%%), spec links %d/%d (%.1f%%), SCM metadata %d/%d (%.1f%%)\n",
		stats.Coverage.Notes.Numerator, stats.Coverage.Notes.Denominator, stats.Coverage.Notes.Ratio*100,
		stats.Coverage.SpecLinks.Numerator, stats.Coverage.SpecLinks.Denominator, stats.Coverage.SpecLinks.Ratio*100,
		stats.Coverage.SCMCloseMetadata.Numerator, stats.Coverage.SCMCloseMetadata.Denominator, stats.Coverage.SCMCloseMetadata.Ratio*100,
	)
	if stats.DataQuality.LegacyEventsIncluded {
		fmt.Fprintf(w, "  Data quality: %d legacy events in scope; exact ordering starts at sequence %d\n", stats.DataQuality.LegacyEventCount, stats.DataQuality.ExactOrderingFromSequence)
	}
}

func writeProjectStats(w io.Writer, projects map[string]report.ProjectStats, total int) {
	fmt.Fprintf(w, "\nProjects: %d\n", len(projects))
	if len(projects) == 0 {
		return
	}
	keys := make([]string, 0, len(projects))
	for key := range projects {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := projects[keys[i]], projects[keys[j]]
		if a.Open != b.Open {
			return a.Open > b.Open
		}
		if a.Ready != b.Ready {
			return a.Ready > b.Ready
		}
		return keys[i] < keys[j]
	})
	table := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(table, "  PROJECT\tTOTAL\tOPEN\tREADY\tIN PROGRESS\tBLOCKED\tDONE\tCANCEL\tDEFER\t7D CREATE/CLOSE\tSHARE")
	for _, key := range keys {
		row := projects[key]
		share := 0.0
		if total > 0 {
			share = float64(row.Total) / float64(total) * 100
		}
		fmt.Fprintf(table, "  %s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d/%d\t%.1f%%\n",
			key, row.Total, row.Open, row.Ready, row.InProgress, row.Blocked, row.Done, row.Cancelled, row.Deferred,
			row.Created7d, row.Closed7d, share)
	}
	_ = table.Flush()
}

func writePercentiles(w io.Writer, label string, values report.Percentiles) {
	if values.SampleSize == 0 {
		fmt.Fprintf(w, "%s: no samples\n", label)
		return
	}
	fmt.Fprintf(w, "%s: n=%d p50=%s p75=%s p90=%s\n", label, values.SampleSize,
		humanDuration(values.P50Seconds), humanDuration(values.P75Seconds), humanDuration(values.P90Seconds))
}

func humanDuration(seconds float64) string {
	switch {
	case seconds < 60:
		return fmt.Sprintf("%.0fs", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%.0fm", seconds/60)
	case seconds < 86400:
		return fmt.Sprintf("%.1fh", seconds/3600)
	default:
		return fmt.Sprintf("%.1fd", seconds/86400)
	}
}

func writeActorStats(w io.Writer, actors map[string]report.ActorStats) {
	if actors == nil {
		return
	}
	keys := make([]string, 0, len(actors))
	for a := range actors {
		keys = append(keys, a)
	}
	sort.Slice(keys, func(i, j int) bool {
		if actors[keys[i]].Claims != actors[keys[j]].Claims {
			return actors[keys[i]].Claims > actors[keys[j]].Claims
		}
		return keys[i] < keys[j]
	})
	fmt.Fprintln(w, "\nAgents (claim owners)")
	t := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(t, "  AGENT\tCLAIMS\tDONE\tCANCEL\tRELEASE\tHANDOFF\tEXPIRED\tOP RELEASE\tDONE/CLAIMS\tMEDIAN")
	for _, a := range keys {
		r := actors[a]
		median := "no samples"
		if r.Duration.SampleSize > 0 {
			median = humanDuration(r.Duration.P50Seconds)
		}
		fmt.Fprintf(t, "  %s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%.1f%%\t%s\n", a, r.Claims, r.Outcomes["done"], r.Outcomes["cancelled"], r.Outcomes["released"], r.Outcomes["handoff"], r.Outcomes["expired"], r.Outcomes["operator_released"], r.CloseRate.Ratio*100, median)
	}
	t.Flush()
	fmt.Fprintln(w, "  Claims and outcomes use their own timestamps; DONE/CLAIMS is not a cohort rate.")
}
func writeIssueCosts(w io.Writer, rows []report.IssueCost) {
	if rows == nil {
		return
	}
	fmt.Fprintln(w, "\nCostliest issues (claims, then completed attempt time)")
	t := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(t, "  ISSUE\tCLAIMS\tATTEMPT TIME\tRELEASE\tHANDOFF\tEXPIRED\tOP RELEASE\tNOTES\tLEAD/AGE\tNO PROGRESS\tUNCLASSIFIED")
	for _, r := range rows {
		flag := ""
		if r.ClaimWithoutProgress {
			flag = "claim without progress"
		}
		fmt.Fprintf(t, "  %s\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%s\t%d %s\t%d\n", r.ShortID, r.Claims, humanDuration(r.AttemptSeconds), r.Released, r.Handoffs, r.Expired, r.OperatorReleased, r.Notes, humanDuration(r.LeadTimeSeconds), r.NoProgressReleases, flag, r.UnclassifiedReleases)
	}
	t.Flush()
}
