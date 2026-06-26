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

// taskDefItem adapts a TaskDefFamilySummary to bubbles/list.Item.
type taskDefItem struct{ td awsx.TaskDefFamilySummary }

func (i taskDefItem) FilterValue() string { return i.td.Family }
func (i taskDefItem) Title() string       { return i.td.Family }
func (i taskDefItem) Description() string {
	return fmt.Sprintf("latest revision %s · ACTIVE", orUnknown(i.td.Revision))
}

// taskDefsLoadedMsg carries fetched task-definition families.
type taskDefsLoadedMsg struct {
	families []awsx.TaskDefFamilySummary
	err      error
}

func loadTaskDefsCmd(client *awsx.EcsClient) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return taskDefsLoadedMsg{err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fams, err := client.ListTaskDefFamilies(ctx)
		return taskDefsLoadedMsg{families: fams, err: err}
	}
}

// TaskDefListModel is the task-definition families list screen.
type TaskDefListModel struct {
	client  *awsx.EcsClient
	list    list.Model
	spinner spinner.Model
	loading bool
	loaded  bool
	err     string
	width   int
	height  int
}

// NewTaskDefList constructs the task-def list.
func NewTaskDefList(client *awsx.EcsClient) TaskDefListModel {
	d := list.NewDefaultDelegate()
	l := list.New(nil, d, 0, 0)
	l.Title = "Task Definitions"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return TaskDefListModel{client: client, list: l, spinner: sp}
}

// Init triggers the first fetch.
func (m TaskDefListModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadTaskDefsCmd(m.client))
}

// Refresh re-fetches families.
func (m *TaskDefListModel) Refresh() tea.Cmd {
	m.loading = true
	m.err = ""
	return tea.Batch(m.spinner.Tick, loadTaskDefsCmd(m.client))
}

// SetSize sets visible dimensions.
func (m *TaskDefListModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.list.SetSize(w, h)
}

// Selected returns the highlighted family name, or "".
func (m TaskDefListModel) Selected() string {
	if it, ok := m.list.SelectedItem().(taskDefItem); ok {
		return it.td.Family
	}
	return ""
}

// IsFiltering reports whether the inner list is in filter-input mode.
func (m TaskDefListModel) IsFiltering() bool {
	return m.list.SettingFilter() || m.list.FilterState() == list.Filtering
}

// Update handles tea messages.
func (m TaskDefListModel) Update(msg tea.Msg) (TaskDefListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case taskDefsLoadedMsg:
		m.loading = false
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.families))
		for _, f := range msg.families {
			items = append(items, taskDefItem{td: f})
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
			return m, m.Refresh()
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the list (or loading / error / empty state).
func (m TaskDefListModel) View() string {
	if m.loading && !m.loaded {
		return fmt.Sprintf("%s loading task definitions…", m.spinner.View())
	}
	if m.err != "" {
		return errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	if m.loaded && len(m.list.Items()) == 0 {
		return faint("no task definitions in this region.\npress r to refresh")
	}
	return m.list.View()
}
