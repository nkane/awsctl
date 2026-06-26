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

// serviceItem adapts a ServiceSummary to bubbles/list.Item.
type serviceItem struct{ ss awsx.ServiceSummary }

func (i serviceItem) FilterValue() string { return i.ss.Name }
func (i serviceItem) Title() string       { return i.ss.Name }
func (i serviceItem) Description() string {
	rollout := i.ss.Rollout
	if rollout == "" {
		rollout = "—"
	}
	return fmt.Sprintf("%s · %d/%d running (%d pending) · %s · %s · %s",
		orUnknown(i.ss.Status), i.ss.Running, i.ss.Desired, i.ss.Pending,
		orUnknown(i.ss.LaunchType), orUnknown(i.ss.TaskDef), rollout)
}

// servicesLoadedMsg carries fetched services for a cluster.
type servicesLoadedMsg struct {
	cluster  string
	services []awsx.ServiceSummary
	err      error
}

// loadServicesCmd fetches services for a cluster in the background.
func loadServicesCmd(client *awsx.EcsClient, cluster string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return servicesLoadedMsg{cluster: cluster, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		svcs, err := client.ListServices(ctx, cluster)
		return servicesLoadedMsg{cluster: cluster, services: svcs, err: err}
	}
}

// ServiceListModel is the per-cluster service list screen.
type ServiceListModel struct {
	client  *awsx.EcsClient
	cluster string
	list    list.Model
	spinner spinner.Model
	loading bool
	loaded  bool
	err     string
	width   int
	height  int
}

// NewServiceList constructs a service list scoped to a cluster.
func NewServiceList(client *awsx.EcsClient, cluster string) ServiceListModel {
	d := list.NewDefaultDelegate()
	l := list.New(nil, d, 0, 0)
	l.Title = "Services · " + cluster
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return ServiceListModel{client: client, cluster: cluster, list: l, spinner: sp}
}

// Init triggers the first fetch.
func (m ServiceListModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadServicesCmd(m.client, m.cluster))
}

// Refresh re-fetches services.
func (m *ServiceListModel) Refresh() tea.Cmd {
	m.loading = true
	m.err = ""
	return tea.Batch(m.spinner.Tick, loadServicesCmd(m.client, m.cluster))
}

// SetSize sets visible dimensions.
func (m *ServiceListModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.list.SetSize(w, h)
}

// Cluster returns the owning cluster name.
func (m ServiceListModel) Cluster() string { return m.cluster }

// Selected returns the highlighted service name, or "".
func (m ServiceListModel) Selected() string {
	if it, ok := m.list.SelectedItem().(serviceItem); ok {
		return it.ss.Name
	}
	return ""
}

// IsFiltering reports whether the inner list is in filter-input mode.
func (m ServiceListModel) IsFiltering() bool {
	return m.list.SettingFilter() || m.list.FilterState() == list.Filtering
}

// Update handles tea messages.
func (m ServiceListModel) Update(msg tea.Msg) (ServiceListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case servicesLoadedMsg:
		if msg.cluster != m.cluster {
			return m, nil // stale load from another cluster
		}
		m.loading = false
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.services))
		for _, s := range msg.services {
			items = append(items, serviceItem{ss: s})
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

// View renders the service list (or loading / error / empty state).
func (m ServiceListModel) View() string {
	if m.loading && !m.loaded {
		return fmt.Sprintf("%s loading services…", m.spinner.View())
	}
	if m.err != "" {
		return errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	if m.loaded && len(m.list.Items()) == 0 {
		return faint("no services in this cluster.\npress r to refresh")
	}
	return m.list.View()
}
