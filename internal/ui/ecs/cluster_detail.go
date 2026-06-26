package ecs

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	awsx "github.com/nkane/awsctl/internal/aws"
)

// clusterDescribeLoadedMsg carries the result of DescribeCluster.
type clusterDescribeLoadedMsg struct {
	name    string
	cluster *ecstypes.Cluster
	err     error
}

func loadClusterDescribeCmd(client *awsx.EcsClient, name string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return clusterDescribeLoadedMsg{name: name, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cl, err := client.DescribeCluster(ctx, name)
		return clusterDescribeLoadedMsg{name: name, cluster: cl, err: err}
	}
}

// ClusterDescribeModel renders one cluster's full description.
type ClusterDescribeModel struct {
	client  *awsx.EcsClient
	name    string
	cluster *ecstypes.Cluster
	vp      viewport.Model
	spinner spinner.Model
	loading bool
	err     string
	width   int
	height  int
}

// NewClusterDescribe constructs the describe screen; Init triggers the load.
func NewClusterDescribe(client *awsx.EcsClient, name string) ClusterDescribeModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return ClusterDescribeModel{
		client:  client,
		name:    name,
		vp:      viewport.New(0, 0),
		spinner: sp,
		loading: true,
	}
}

// Init kicks off the first describe call.
func (m ClusterDescribeModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadClusterDescribeCmd(m.client, m.name))
}

// Name returns the cluster name.
func (m ClusterDescribeModel) Name() string { return m.name }

// SetSize sizes the viewport (1-line title + 1-line footer).
func (m *ClusterDescribeModel) SetSize(w, h int) {
	m.width, m.height = w, h
	body := h - 2
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
	if m.cluster != nil {
		m.vp.SetContent(renderCluster(m.cluster))
	}
}

// Update handles describe results + key input.
func (m ClusterDescribeModel) Update(msg tea.Msg) (ClusterDescribeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case clusterDescribeLoadedMsg:
		if msg.name != m.name {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.cluster = msg.cluster
		m.vp.SetContent(renderCluster(msg.cluster))
		m.vp.GotoTop()
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if msg.String() == "r" {
			m.loading = true
			m.err = ""
			return m, tea.Batch(m.spinner.Tick, loadClusterDescribeCmd(m.client, m.name))
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the describe screen.
func (m ClusterDescribeModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("cluster: " + m.name)
	body := m.vp.View()
	if m.loading {
		body = fmt.Sprintf("%s describing %s…", m.spinner.View(), m.name)
	} else if m.err != "" {
		body = errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	footer := faint("r refresh · esc back")
	return title + "\n" + body + "\n" + footer
}

// renderCluster formats a Cluster as a multi-section string for the viewport.
func renderCluster(c *ecstypes.Cluster) string {
	if c == nil {
		return faint("(no description)")
	}
	var b strings.Builder
	hSty := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	kSty := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	row := func(k, v string) {
		if v == "" {
			v = "—"
		}
		b.WriteString("  " + kSty.Render(k) + "  " + v + "\n")
	}
	hdr := func(s string) { b.WriteString("\n" + hSty.Render(s) + "\n") }

	// Overview
	hdr("Overview")
	row("status", deref(c.Status))
	row("running-tasks", fmt.Sprintf("%d", c.RunningTasksCount))
	row("pending-tasks", fmt.Sprintf("%d", c.PendingTasksCount))
	row("active-services", fmt.Sprintf("%d", c.ActiveServicesCount))
	row("container-instances", fmt.Sprintf("%d", c.RegisteredContainerInstancesCount))
	if c.ClusterArn != nil {
		row("arn", *c.ClusterArn)
	}

	// Capacity providers
	if len(c.CapacityProviders) > 0 {
		hdr("Capacity Providers")
		row("providers", strings.Join(c.CapacityProviders, ", "))
		for _, s := range c.DefaultCapacityProviderStrategy {
			line := deref(s.CapacityProvider) + fmt.Sprintf(" (weight %d, base %d)", s.Weight, s.Base)
			row("  default", line)
		}
	}

	// Settings
	if len(c.Settings) > 0 {
		hdr("Settings")
		for _, st := range c.Settings {
			row(string(st.Name), deref(st.Value))
		}
	}

	// Statistics (sorted for stable rendering)
	if len(c.Statistics) > 0 {
		hdr("Statistics")
		stats := append([]ecstypes.KeyValuePair(nil), c.Statistics...)
		sort.Slice(stats, func(i, j int) bool { return deref(stats[i].Name) < deref(stats[j].Name) })
		for _, kv := range stats {
			row(deref(kv.Name), deref(kv.Value))
		}
	}

	// Tags
	if len(c.Tags) > 0 {
		hdr(fmt.Sprintf("Tags (%d)", len(c.Tags)))
		for _, tg := range c.Tags {
			row(deref(tg.Key), deref(tg.Value))
		}
	}

	return b.String()
}
