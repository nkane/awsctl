// Package ecs renders the ECS cluster list and (later) service/task drill-down.
package ecs

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
)

// clusterItem adapts a ClusterSummary to bubbles/list.Item.
type clusterItem struct{ cs awsx.ClusterSummary }

func (i clusterItem) FilterValue() string { return i.cs.Name }
func (i clusterItem) Title() string       { return i.cs.Name }
func (i clusterItem) Description() string {
	return fmt.Sprintf("%s · %d running · %d pending · %d services · %d instances",
		orUnknown(i.cs.Status), i.cs.RunningTasks, i.cs.PendingTasks,
		i.cs.ActiveServices, i.cs.ContainerInstances)
}

// loadedMsg carries fetched clusters.
type loadedMsg struct {
	clusters []awsx.ClusterSummary
	err      error
}

// LoadCmd kicks off ListClusters in the background.
func LoadCmd(client *awsx.EcsClient) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return loadedMsg{err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		clusters, err := client.ListClusters(ctx)
		return loadedMsg{clusters: clusters, err: err}
	}
}

// ListModel is the ECS cluster-list screen.
type ListModel struct {
	client  *awsx.EcsClient
	list    list.Model
	spinner spinner.Model
	loading bool
	loaded  bool
	err     string
	width   int
	height  int
}

// NewList constructs an empty list model. Call SetClient + Refresh once an AWS
// config is resolved.
func NewList() ListModel {
	d := list.NewDefaultDelegate()
	l := list.New(nil, d, 0, 0)
	l.Title = "ECS Clusters"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return ListModel{list: l, spinner: sp}
}

// SetClient swaps in the AWS client (called when config loads or changes).
func (m *ListModel) SetClient(c *awsx.EcsClient) { m.client = c }

// Refresh triggers a fresh ListClusters call.
func (m *ListModel) Refresh() tea.Cmd {
	m.loading = true
	m.err = ""
	return tea.Batch(m.spinner.Tick, LoadCmd(m.client))
}

// SetSize sets visible dimensions.
func (m *ListModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.list.SetSize(w, h)
}

// Selected returns the highlighted cluster name, or "".
func (m ListModel) Selected() string {
	if it, ok := m.list.SelectedItem().(clusterItem); ok {
		return it.cs.Name
	}
	return ""
}

// Loaded reports whether the first fetch completed.
func (m ListModel) Loaded() bool { return m.loaded }

// IsFiltering reports whether the inner list is in filter-input mode.
func (m ListModel) IsFiltering() bool {
	return m.list.SettingFilter() || m.list.FilterState() == list.Filtering
}

// Update handles tea messages.
func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.loading = false
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.clusters))
		for _, c := range msg.clusters {
			items = append(items, clusterItem{cs: c})
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
func (m ListModel) View() string {
	if m.client == nil {
		return faint("waiting for AWS config…")
	}
	if m.loading && !m.loaded {
		return fmt.Sprintf("%s loading clusters…", m.spinner.View())
	}
	if m.err != "" {
		return errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	if m.loaded && len(m.list.Items()) == 0 {
		return faint("no ECS clusters found in this region.\npress r to refresh")
	}
	return m.list.View()
}

var (
	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	faintSty = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func faint(s string) string { return faintSty.Render(s) }

func orUnknown(s string) string {
	if s == "" {
		return "?"
	}
	return s
}
