package ecs

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

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

// TaskDefRevisionsModel is the revision-history list for one family.
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

	return TaskDefRevisionsModel{client: client, family: family, list: l, spinner: sp}
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

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if msg.String() == "r" && !m.list.SettingFilter() {
			m.loading = true
			m.err = ""
			return m, tea.Batch(m.spinner.Tick, loadRevisionsCmd(m.client, m.family))
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the list (or loading / error / empty state).
func (m TaskDefRevisionsModel) View() string {
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
