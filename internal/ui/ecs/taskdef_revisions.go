package ecs

import (
	"context"
	"fmt"
	"time"

	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
)

// revisionItem adapts a TaskDefRevision to bubbles/list.Item.
type revisionItem struct{ r awsx.TaskDefRevision }

func (i revisionItem) FilterValue() string { return i.r.Revision }
func (i revisionItem) Title() string       { return "revision " + i.r.Revision }
func (i revisionItem) Description() string { return "ACTIVE · " + i.r.Arn }

// revisionsLoadedMsg carries fetched revisions for a family.
type revisionsLoadedMsg struct {
	family    string
	revisions []awsx.TaskDefRevision
	err       error
}

func loadRevisionsCmd(client *awsx.EcsClient, family string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return revisionsLoadedMsg{family: family, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		revs, err := client.ListTaskDefRevisions(ctx, family)
		return revisionsLoadedMsg{family: family, revisions: revs, err: err}
	}
}

// diffLoadedMsg carries a computed revision-to-revision diff.
type diffLoadedMsg struct {
	family string
	lines  []diffLine
	err    error
}

// loadDiffCmd fetches two revisions' task-def JSON and diffs them line-by-line.
func loadDiffCmd(client *awsx.EcsClient, family, baseRev, targetRev string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return diffLoadedMsg{family: family, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		base, err := client.DescribeTaskDef(ctx, family+":"+baseRev)
		if err != nil {
			return diffLoadedMsg{family: family, err: fmt.Errorf("revision %s: %w", baseRev, err)}
		}
		target, err := client.DescribeTaskDef(ctx, family+":"+targetRev)
		if err != nil {
			return diffLoadedMsg{family: family, err: fmt.Errorf("revision %s: %w", targetRev, err)}
		}
		baseLines := strings.Split(renderTaskDefJSON(base), "\n")
		targetLines := strings.Split(renderTaskDefJSON(target), "\n")
		return diffLoadedMsg{family: family, lines: lineDiff(baseLines, targetLines)}
	}
}

// TaskDefRevisionsModel is the revision-history list for one family. 'd' marks a
// base revision; pressing 'd' on a different revision opens an inline two-way
// diff of the two revisions' task-def JSON.
type TaskDefRevisionsModel struct {
	client  *awsx.EcsClient
	family  string
	list    list.Model
	spinner spinner.Model
	loading bool
	loaded  bool
	err     string
	width   int
	height  int

	baseRev     string // revision marked as the diff base ("" = none)
	diffing     bool   // diff overlay is open
	diffLoading bool
	diffErr     string
	diffTitle   string
	diffVP      viewport.Model
}

// NewTaskDefRevisions constructs the revisions list scoped to a family.
func NewTaskDefRevisions(client *awsx.EcsClient, family string) TaskDefRevisionsModel {
	d := list.NewDefaultDelegate()
	l := list.New(nil, d, 0, 0)
	l.Title = "Revisions · " + family
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return TaskDefRevisionsModel{
		client:  client,
		family:  family,
		list:    l,
		spinner: sp,
		diffVP:  viewport.New(0, 0),
	}
}

// Init triggers the first fetch.
func (m TaskDefRevisionsModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadRevisionsCmd(m.client, m.family))
}

// Family returns the owning family name.
func (m TaskDefRevisionsModel) Family() string { return m.family }

// SetSize sets visible dimensions.
func (m *TaskDefRevisionsModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.list.SetSize(w, h)
	body := h - 2 // 1-line title + 1-line footer
	if body < 4 {
		body = 4
	}
	m.diffVP.Width = w
	m.diffVP.Height = body
}

// Diffing reports whether the inline diff overlay is open. The App uses this to
// suppress the enter→describe drill while the overlay owns the screen.
func (m TaskDefRevisionsModel) Diffing() bool { return m.diffing }

// updateTitle reflects the marked diff base in the list title.
func (m *TaskDefRevisionsModel) updateTitle() {
	t := "Revisions · " + m.family
	if m.baseRev != "" {
		t += "  [diff base: " + m.baseRev + "]"
	}
	m.list.Title = t
}

// Selected returns the highlighted revision number, or "".
func (m TaskDefRevisionsModel) Selected() string {
	if it, ok := m.list.SelectedItem().(revisionItem); ok {
		return it.r.Revision
	}
	return ""
}

// IsFiltering reports whether the inner list is in filter-input mode.
func (m TaskDefRevisionsModel) IsFiltering() bool {
	return m.list.SettingFilter() || m.list.FilterState() == list.Filtering
}

// Update handles tea messages.
func (m TaskDefRevisionsModel) Update(msg tea.Msg) (TaskDefRevisionsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case revisionsLoadedMsg:
		if msg.family != m.family {
			return m, nil
		}
		m.loading = false
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.revisions))
		for _, r := range msg.revisions {
			items = append(items, revisionItem{r: r})
		}
		cmd := m.list.SetItems(items)
		return m, cmd

	case diffLoadedMsg:
		if msg.family != m.family {
			return m, nil
		}
		m.diffLoading = false
		if msg.err != nil {
			m.diffErr = msg.err.Error()
			return m, nil
		}
		m.diffErr = ""
		added, removed := diffStat(msg.lines)
		m.diffTitle += fmt.Sprintf("  (+%d -%d)", added, removed)
		m.diffVP.SetContent(renderDiff(msg.lines))
		m.diffVP.GotoTop()
		return m, nil

	case spinner.TickMsg:
		if !m.loading && !m.diffLoading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// While the diff overlay is open it owns input: esc closes it, everything
		// else scrolls the diff viewport.
		if m.diffing {
			if msg.String() == "esc" {
				m.diffing = false
				return m, nil
			}
			var cmd tea.Cmd
			m.diffVP, cmd = m.diffVP.Update(msg)
			return m, cmd
		}
		if m.list.SettingFilter() {
			break
		}
		switch msg.String() {
		case "r":
			m.loading = true
			m.err = ""
			return m, tea.Batch(m.spinner.Tick, loadRevisionsCmd(m.client, m.family))
		case "d":
			return m.handleDiffKey()
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// handleDiffKey implements the two-press diff gesture: the first 'd' marks the
// highlighted revision as the base, a second 'd' on the same revision clears it,
// and 'd' on a different revision opens the base→target diff.
func (m TaskDefRevisionsModel) handleDiffKey() (TaskDefRevisionsModel, tea.Cmd) {
	cur := m.Selected()
	if cur == "" {
		return m, nil
	}
	switch {
	case m.baseRev == "":
		m.baseRev = cur
		m.updateTitle()
		return m, nil
	case m.baseRev == cur:
		m.baseRev = ""
		m.updateTitle()
		return m, nil
	default:
		m.diffing = true
		m.diffLoading = true
		m.diffErr = ""
		m.diffTitle = fmt.Sprintf("diff %s: r%s → r%s", m.family, m.baseRev, cur)
		return m, tea.Batch(m.spinner.Tick, loadDiffCmd(m.client, m.family, m.baseRev, cur))
	}
}

// View renders the list, the diff overlay, or a loading / error / empty state.
func (m TaskDefRevisionsModel) View() string {
	if m.diffing {
		return m.diffView()
	}
	if m.loading && !m.loaded {
		return fmt.Sprintf("%s loading revisions…", m.spinner.View())
	}
	if m.err != "" {
		return errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	if m.loaded && len(m.list.Items()) == 0 {
		return faint("no active revisions for this family.\npress r to refresh")
	}
	return m.list.View()
}

// diffView renders the inline two-way diff overlay.
func (m TaskDefRevisionsModel) diffView() string {
	title := lipgloss.NewStyle().Bold(true).Render(m.diffTitle)
	switch {
	case m.diffLoading:
		return title + "\n" + fmt.Sprintf("%s computing diff…", m.spinner.View()) + "\n" + faint("esc back")
	case m.diffErr != "":
		return title + "\n" + errStyle.Render("error: "+m.diffErr) + "\n\n" + faint("esc back")
	default:
		return title + "\n" + m.diffVP.View() + "\n" + faint("↑/↓ scroll · esc back")
	}
}
