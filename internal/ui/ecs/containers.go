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

// containerItem adapts a ContainerSummary to bubbles/list.Item.
type containerItem struct{ cs awsx.ContainerSummary }

func (i containerItem) FilterValue() string { return i.cs.Name }
func (i containerItem) Title() string       { return i.cs.Name }
func (i containerItem) Description() string {
	health := i.cs.Health
	if health == "" || health == "UNKNOWN" {
		health = "—"
	}
	exit := i.cs.ExitCode
	if exit == "" {
		exit = "—"
	}
	return fmt.Sprintf("%s · health %s · exit %s · %s",
		orUnknown(i.cs.LastStatus), health, exit, orUnknown(i.cs.Image))
}

// containersLoadedMsg carries fetched containers for a task.
type containersLoadedMsg struct {
	task       string
	containers []awsx.ContainerSummary
	err        error
}

func loadContainersCmd(client *awsx.EcsClient, cluster, task string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return containersLoadedMsg{task: task, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cs, err := client.DescribeTaskContainers(ctx, cluster, task)
		return containersLoadedMsg{task: task, containers: cs, err: err}
	}
}

// ContainerListModel is the per-task container list screen.
type ContainerListModel struct {
	client  *awsx.EcsClient
	cluster string
	task    string
	list    list.Model
	spinner spinner.Model
	loading bool
	loaded  bool
	err     string
	width   int
	height  int
}

// NewContainerList constructs a container list scoped to a cluster + task.
func NewContainerList(client *awsx.EcsClient, cluster, task string) ContainerListModel {
	d := list.NewDefaultDelegate()
	l := list.New(nil, d, 0, 0)
	l.Title = "Containers · " + task
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return ContainerListModel{client: client, cluster: cluster, task: task, list: l, spinner: sp}
}

// Init triggers the first fetch.
func (m ContainerListModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadContainersCmd(m.client, m.cluster, m.task))
}

// Refresh re-fetches containers.
func (m *ContainerListModel) Refresh() tea.Cmd {
	m.loading = true
	m.err = ""
	return tea.Batch(m.spinner.Tick, loadContainersCmd(m.client, m.cluster, m.task))
}

// SetSize sets visible dimensions.
func (m *ContainerListModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.list.SetSize(w, h)
}

// Selected returns the highlighted container name, or "".
func (m ContainerListModel) Selected() string {
	if it, ok := m.list.SelectedItem().(containerItem); ok {
		return it.cs.Name
	}
	return ""
}

// IsFiltering reports whether the inner list is in filter-input mode.
func (m ContainerListModel) IsFiltering() bool {
	return m.list.SettingFilter() || m.list.FilterState() == list.Filtering
}

// Update handles tea messages.
func (m ContainerListModel) Update(msg tea.Msg) (ContainerListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case containersLoadedMsg:
		if msg.task != m.task {
			return m, nil // stale load from another task
		}
		m.loading = false
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.containers))
		for _, c := range msg.containers {
			items = append(items, containerItem{cs: c})
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

// View renders the container list (or loading / error / empty state).
func (m ContainerListModel) View() string {
	if m.loading && !m.loaded {
		return fmt.Sprintf("%s loading containers…", m.spinner.View())
	}
	if m.err != "" {
		return errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	if m.loaded && len(m.list.Items()) == 0 {
		return faint("no containers in this task.\npress r to refresh")
	}
	return m.list.View()
}
