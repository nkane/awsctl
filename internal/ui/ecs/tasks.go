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

// taskItem adapts a TaskSummary to bubbles/list.Item.
type taskItem struct{ ts awsx.TaskSummary }

func (i taskItem) FilterValue() string { return i.ts.ID }
func (i taskItem) Title() string       { return i.ts.ID }
func (i taskItem) Description() string {
	health := i.ts.Health
	if health == "" || health == "UNKNOWN" {
		health = "—"
	}
	started := i.ts.StartedAt
	if started == "" {
		started = "not started"
	}
	return fmt.Sprintf("%s → %s · health %s · %s · %s · %s",
		orUnknown(i.ts.LastStatus), orUnknown(i.ts.Desired), health,
		orUnknown(i.ts.LaunchType), orUnknown(i.ts.TaskDef), started)
}

// tasksLoadedMsg carries fetched tasks for a service.
type tasksLoadedMsg struct {
	cluster string
	service string
	tasks   []awsx.TaskSummary
	err     error
}

func loadTasksCmd(client *awsx.EcsClient, cluster, service string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return tasksLoadedMsg{cluster: cluster, service: service, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		tasks, err := client.ListTasks(ctx, cluster, service)
		return tasksLoadedMsg{cluster: cluster, service: service, tasks: tasks, err: err}
	}
}

// TaskListModel is the per-service task list screen.
type TaskListModel struct {
	client  *awsx.EcsClient
	cluster string
	service string
	list    list.Model
	spinner spinner.Model
	loading bool
	loaded  bool
	err     string
	width   int
	height  int
}

// NewTaskList constructs a task list scoped to a cluster + service.
func NewTaskList(client *awsx.EcsClient, cluster, service string) TaskListModel {
	d := list.NewDefaultDelegate()
	l := list.New(nil, d, 0, 0)
	l.Title = "Tasks · " + service
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return TaskListModel{client: client, cluster: cluster, service: service, list: l, spinner: sp}
}

// Init triggers the first fetch.
func (m TaskListModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadTasksCmd(m.client, m.cluster, m.service))
}

// Refresh re-fetches tasks.
func (m *TaskListModel) Refresh() tea.Cmd {
	m.loading = true
	m.err = ""
	return tea.Batch(m.spinner.Tick, loadTasksCmd(m.client, m.cluster, m.service))
}

// SetSize sets visible dimensions.
func (m *TaskListModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.list.SetSize(w, h)
}

// Cluster returns the owning cluster name.
func (m TaskListModel) Cluster() string { return m.cluster }

// Selected returns the highlighted task id, or "".
func (m TaskListModel) Selected() string {
	if it, ok := m.list.SelectedItem().(taskItem); ok {
		return it.ts.ID
	}
	return ""
}

// IsFiltering reports whether the inner list is in filter-input mode.
func (m TaskListModel) IsFiltering() bool {
	return m.list.SettingFilter() || m.list.FilterState() == list.Filtering
}

// Update handles tea messages.
func (m TaskListModel) Update(msg tea.Msg) (TaskListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tasksLoadedMsg:
		if msg.cluster != m.cluster || msg.service != m.service {
			return m, nil // stale load from another service
		}
		m.loading = false
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.tasks))
		for _, tk := range msg.tasks {
			items = append(items, taskItem{ts: tk})
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

// View renders the task list (or loading / error / empty state).
func (m TaskListModel) View() string {
	if m.loading && !m.loaded {
		return fmt.Sprintf("%s loading tasks…", m.spinner.View())
	}
	if m.err != "" {
		return errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	if m.loaded && len(m.list.Items()) == 0 {
		return faint("no tasks for this service.\npress r to refresh")
	}
	return m.list.View()
}
