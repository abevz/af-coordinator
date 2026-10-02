package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/abevz/dibs/internal/client"
	"github.com/abevz/dibs/internal/watch"
)

func runWatch(ctx context.Context, c *client.Client, args []string) error {
	var project string
	once := jsonOutput
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return argumentError("--project requires a key")
			}
			project = args[i+1]
			i++
		case "--once":
			once = true
		default:
			return argumentError("unknown watch flag: " + args[i])
		}
	}
	svc := watch.New(c, project)
	if once {
		snapshot, err := svc.Refresh(ctx, time.Now())
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(snapshot)
		}
		fmt.Fprintln(os.Stdout, watch.RenderOnce(snapshot, time.Now()))
		if line := updateNotice(true); line != "" {
			fmt.Fprintln(os.Stderr, line)
		}
		return nil
	}
	if !terminalDevice(os.Stdin) || !terminalDevice(os.Stdout) {
		return argumentError("watch requires a terminal; use --once or --json for one snapshot")
	}
	model := watchModel{
		ctx:         ctx,
		rootProject: project,
		service:     svc,
		snapshot:    watch.Snapshot{Project: project},
		width:       100,
		height:      28,
		loading:     true,
		footer:      updateNotice(true),
		now:         time.Now(),
	}
	_, err := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
		return nil
	}
	return err
}

func terminalDevice(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type watchSnapshotMsg struct {
	snapshot watch.Snapshot
	err      error
}

type watchDetailMsg struct {
	id     string
	detail watch.Detail
	err    error
}

type watchRefreshMsg struct{}
type watchClockMsg time.Time

type watchModel struct {
	footer        string
	ctx           context.Context
	service       *watch.Service
	rootProject   string
	selected      int
	detailID      string
	detail        watch.Detail
	detailErr     error
	detailLoading bool
	detailOffset  int
	snapshot      watch.Snapshot
	lastErr       error
	width         int
	height        int
	loading       bool
	now           time.Time
}

func (m watchModel) Init() tea.Cmd {
	return tea.Batch(m.fetch(), watchClock())
}

func (m watchModel) fetch() tea.Cmd {
	return func() tea.Msg {
		snapshot, err := m.service.Refresh(m.ctx, time.Now())
		return watchSnapshotMsg{snapshot: snapshot, err: err}
	}
}

func watchClock() tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg { return watchClockMsg(now) })
}

func watchRefresh() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return watchRefreshMsg{} })
}

func (m watchModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "backspace":
			if m.detailID != "" {
				m.detailID = ""
				m.detailOffset = 0
				return m, nil
			}
			if m.snapshot.Project != m.rootProject && !m.loading {
				m.service = m.service.Scoped(m.rootProject)
				m.snapshot = watch.Snapshot{Project: m.rootProject}
				m.selected = 0
				m.loading = true
				return m, m.fetch()
			}
		case "up", "k":
			if m.detailID != "" {
				if m.detailOffset > 0 {
					m.detailOffset--
				}
			} else if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.detailID != "" {
				m.detailOffset++
			} else if m.selected+1 < m.rowCount() {
				m.selected++
			}
		case "enter":
			if m.detailID != "" || m.loading {
				return m, nil
			}
			if m.snapshot.Project == "" {
				if m.selected < len(m.snapshot.Projects) {
					project := m.snapshot.Projects[m.selected].Key
					m.service = m.service.Scoped(project)
					m.snapshot = watch.Snapshot{Project: project}
					m.selected = 0
					m.loading = true
					return m, m.fetch()
				}
			} else if m.selected < len(m.snapshot.Issues) {
				issue := m.snapshot.Issues[m.selected]
				m.detailID = issue.ID
				m.detail = watch.Detail{Issue: issue}
				m.detailErr = nil
				m.detailOffset = 0
				m.detailLoading = true
				return m, m.fetchDetail()
			}
		case "r":
			if m.detailID != "" {
				if !m.detailLoading {
					m.detailLoading = true
					return m, m.fetchDetail()
				}
				return m, nil
			}
			if !m.loading {
				m.loading = true
				return m, m.fetch()
			}
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case watchClockMsg:
		m.now = time.Time(msg)
		return m, watchClock()
	case watchRefreshMsg:
		if m.footer == "" {
			m.footer = updateNotice(true)
		}
		if !m.loading {
			m.loading = true
			return m, m.fetch()
		}
	case watchDetailMsg:
		if msg.id == m.detailID {
			m.detailLoading = false
			m.detailErr = msg.err
			if msg.err == nil {
				m.detail = msg.detail
			}
		}
	case watchSnapshotMsg:
		m.loading = false
		m.lastErr = msg.err
		if msg.err == nil {
			selectedID := ""
			if m.selected < len(m.snapshot.Issues) {
				selectedID = m.snapshot.Issues[m.selected].ID
			}
			selectedKey := ""
			if m.selected < len(m.snapshot.Projects) {
				selectedKey = m.snapshot.Projects[m.selected].Key
			}
			m.snapshot = msg.snapshot
			for i, issue := range m.snapshot.Issues {
				if issue.ID == selectedID {
					m.selected = i
				}
			}
			for i, project := range m.snapshot.Projects {
				if project.Key == selectedKey {
					m.selected = i
				}
			}
			if m.selected >= m.rowCount() {
				m.selected = m.rowCount() - 1
			}
			if m.selected < 0 {
				m.selected = 0
			}
		}
		return m, watchRefresh()
	}
	return m, nil
}

func (m watchModel) View() string {
	height := m.height
	if m.footer != "" && height > 1 {
		height--
	}
	foot := func(s string) string {
		if m.footer != "" {
			return s + "\n" + ansi.Truncate(m.footer, m.width, "…")
		}
		return s
	}
	if m.detailID != "" {
		return foot(watch.RenderDetail(m.detail, m.detailErr, m.detailLoading, m.width, height, m.detailOffset))
	}
	snapshot := m.snapshot
	if snapshot.Issues != nil {
		snapshot = snapshot.At(m.now)
	}
	return foot(watch.RenderNavigation(snapshot, m.lastErr, m.now, m.width, height, m.selected))
}

func (m watchModel) rowCount() int {
	if m.snapshot.Project == "" {
		return len(m.snapshot.Projects)
	}
	return len(m.snapshot.Issues)
}
func (m watchModel) fetchDetail() tea.Cmd {
	id, service := m.detailID, m.service
	return func() tea.Msg {
		detail, err := service.Detail(m.ctx, id)
		return watchDetailMsg{id: id, detail: detail, err: err}
	}
}
