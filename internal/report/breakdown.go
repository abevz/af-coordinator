package report

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/abevz/dibs/internal/core"
)

var versionActor = regexp.MustCompile(`^v?[0-9]+\.[0-9]+(?:\.[0-9]+)?(?:[-+].*)?$`)
var taskActor = regexp.MustCompile(`^(?:afc|aion|utils|piac|jsb)-[0-9]+-(.+)$`)
var liveWorker = regexp.MustCompile(`^aion-worker-live-[0-9]+$`)

// NormalizeActor is a report-only convention; it never rewrites records or
// infers the engine of a task-role label or a bare version.
func NormalizeActor(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" || versionActor.MatchString(name) {
		return "unknown"
	}
	if at := strings.IndexByte(name, '@'); at >= 0 {
		name = name[:at]
	}
	if versionActor.MatchString(name) {
		return "unknown"
	}
	for _, family := range []string{"codex", "agy", "claude.exe", "claude-code", "claude", "opencode", "aion-forge-worker"} {
		if name == family || strings.HasPrefix(name, family+"-") {
			if family == "claude.exe" || family == "claude-code" {
				return "claude"
			}
			return family
		}
	}
	if liveWorker.MatchString(name) {
		return "aion-forge-worker"
	}
	if task := taskActor.FindStringSubmatch(name); task != nil {
		return "task/" + task[1]
	}
	if name == "" {
		return "unknown"
	}
	return name
}

type ActorStats struct {
	RawActors []string       `json:"raw_actors"`
	Claims    int            `json:"claims"`
	Completed int            `json:"completed"`
	Outcomes  map[string]int `json:"outcomes"`
	CloseRate Coverage       `json:"close_rate"`
	Duration  Percentiles    `json:"duration"`
}

type IssueCost struct {
	IssueID              string  `json:"issue_id"`
	ShortID              string  `json:"short_id"`
	Claims               int     `json:"claims"`
	TrackedClaims        int     `json:"tracked_claims"`
	Completed            int     `json:"completed"`
	AttemptSeconds       float64 `json:"attempt_seconds"`
	Released             int     `json:"released"`
	Handoffs             int     `json:"handoffs"`
	Expired              int     `json:"expired"`
	OperatorReleased     int     `json:"operator_released"`
	Notes                int     `json:"notes"`
	LeadTimeSeconds      float64 `json:"lead_time_seconds"`
	NoProgressReleases   int     `json:"no_progress_releases"`
	UnclassifiedReleases int     `json:"unclassified_releases"`
	ClaimWithoutProgress bool    `json:"claim_without_progress"`
}

type breakdown struct {
	report      *Report
	rows        map[string]*IssueCost
	issues      map[string]core.Issue
	active      map[string]string
	durations   map[string][]float64
	aliases     map[string]map[string]bool
	latestClose map[string]time.Time
	until       time.Time
	top         int
}

func newBreakdown(r *Report, issues []core.Issue, q parsedQuery, top int) *breakdown {
	b := &breakdown{report: r, rows: map[string]*IssueCost{}, issues: map[string]core.Issue{}, active: map[string]string{}, durations: map[string][]float64{}, aliases: map[string]map[string]bool{}, latestClose: map[string]time.Time{}, until: q.until, top: top}
	for _, i := range issues {
		b.issues[i.ID] = i
		b.rows[i.ID] = &IssueCost{IssueID: i.ID, ShortID: i.ShortID}
	}
	return b
}
func (b *breakdown) actor(e attempt) ActorStats {
	row, ok := b.report.ByActor[e.agent]
	if !ok {
		row = ActorStats{Outcomes: emptyOutcomeCounts()}
		b.aliases[e.agent] = map[string]bool{}
	}
	b.aliases[e.agent][e.rawActor] = true
	return row
}
func (b *breakdown) claim(e attempt, within bool) {
	if within && b.report.ByActor != nil {
		row := b.actor(e)
		row.Claims++
		b.report.ByActor[e.agent] = row
	}
	if within {
		b.rows[e.issueID].TrackedClaims++
	}
}
func (b *breakdown) observe(e core.Event, p map[string]any, at time.Time, within bool, attempts map[string]attempt) {
	if at.After(b.until) {
		return
	}
	row := b.rows[e.IssueID]
	if e.EventType == "issue_claimed" {
		b.active[e.IssueID] = payloadString(p, "attempt_id")
		if within {
			row.Claims++
		}
	}
	id := b.active[e.IssueID]
	if entry, ok := attempts[id]; ok && entry.issueID == e.IssueID && (payloadString(p, "attempt_id") == "" || payloadString(p, "attempt_id") == id) {
		// Wall clocks can move backwards between committed mutations. Exact
		// sequence order, rather than timestamps, associates progress with
		// the current attempt. Missing order evidence cannot prove its absence.
		if entry.exact && e.Sequence <= entry.sequence {
			entry.exact = false
		}
		if (e.EventType == "note_added" || hasSCM(p)) && (entry.exact || !at.Before(entry.started)) {
			entry.progress = true
		}
		attempts[id] = entry
	}
	switch e.EventType {
	case "note_added":
		if within {
			row.Notes++
		}
	case "issue_closed", "issue_operator_closed":
		if last := b.latestClose[e.IssueID]; last.IsZero() || at.After(last) {
			b.latestClose[e.IssueID] = at
		}
		if id == payloadString(p, "attempt_id") {
			delete(b.active, e.IssueID)
		}
	case "issue_released":
		if within {
			if payloadString(p, "end_reason") == "handoff" {
				row.Handoffs++
			} else if payloadString(p, "end_reason") == "operator_released" {
				row.OperatorReleased++
			} else {
				row.Released++
			}
		}
		if id == payloadString(p, "attempt_id") {
			delete(b.active, e.IssueID)
		}
	case "issue_operator_released":
		if within {
			row.OperatorReleased++
		}
		if id == payloadString(p, "attempt_id") {
			delete(b.active, e.IssueID)
		}
	case "lease_expired":
		if within {
			row.Expired++
		}
		if id == payloadString(p, "attempt_id") {
			delete(b.active, e.IssueID)
		}
	}
}
func hasSCM(p map[string]any) bool {
	return payloadString(p, "commit_sha") != "" || payloadString(p, "pr_url") != ""
}
func (b *breakdown) complete(e attempt, ended time.Time, outcome string, p map[string]any) {
	seconds := ended.Sub(e.started).Seconds()
	if b.report.ByActor != nil {
		row := b.actor(e)
		row.Completed++
		row.Outcomes[outcome]++
		b.report.ByActor[e.agent] = row
		b.durations[e.agent] = append(b.durations[e.agent], seconds)
	}
	row := b.rows[e.issueID]
	row.Completed++
	row.AttemptSeconds += seconds
	if outcome == "released" {
		if !e.exact {
			row.UnclassifiedReleases++
		} else if !e.progress && !hasSCM(p) {
			row.NoProgressReleases++
		}
	}
}
func (b *breakdown) finish() {
	for agent, row := range b.report.ByActor {
		for raw := range b.aliases[agent] {
			row.RawActors = append(row.RawActors, raw)
		}
		sort.Strings(row.RawActors)
		row.CloseRate = coverage(row.Outcomes["done"], row.Claims)
		row.Duration = percentiles(b.durations[agent])
		b.report.ByActor[agent] = row
	}
	if b.top <= 0 {
		return
	}
	b.report.TopIssues = []IssueCost{}
	for id, row := range b.rows {
		if row.Claims == 0 && row.Completed == 0 && row.Released+row.Handoffs+row.Expired+row.OperatorReleased == 0 {
			continue
		}
		end := b.latestClose[id]
		if end.IsZero() {
			end = b.until
		}
		start, err := parseTimestamp(b.issues[id].CreatedAt)
		if err == nil && !end.Before(start) {
			row.LeadTimeSeconds = end.Sub(start).Seconds()
		}
		row.ClaimWithoutProgress = row.NoProgressReleases >= 3
		b.report.TopIssues = append(b.report.TopIssues, *row)
	}
	sort.Slice(b.report.TopIssues, func(i, j int) bool {
		a, c := b.report.TopIssues[i], b.report.TopIssues[j]
		if a.Claims != c.Claims {
			return a.Claims > c.Claims
		}
		if a.AttemptSeconds != c.AttemptSeconds {
			return a.AttemptSeconds > c.AttemptSeconds
		}
		if a.ShortID != c.ShortID {
			return a.ShortID < c.ShortID
		}
		return a.IssueID < c.IssueID
	})
	if len(b.report.TopIssues) > b.top {
		b.report.TopIssues = b.report.TopIssues[:b.top]
	}
}
