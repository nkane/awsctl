package ecs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	awsx "github.com/nkane/awsctl/internal/aws"
)

// serviceDescribeLoadedMsg carries the result of DescribeService.
type serviceDescribeLoadedMsg struct {
	name string
	svc  *ecstypes.Service
	err  error
}

func loadServiceDescribeCmd(client *awsx.EcsClient, cluster, name string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return serviceDescribeLoadedMsg{name: name, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		svc, err := client.DescribeService(ctx, cluster, name)
		return serviceDescribeLoadedMsg{name: name, svc: svc, err: err}
	}
}

// ServiceDescribeModel renders one service's full description.
type ServiceDescribeModel struct {
	client  *awsx.EcsClient
	cluster string
	name    string
	svc     *ecstypes.Service
	vp      viewport.Model
	spinner spinner.Model
	loading bool
	err     string
	width   int
	height  int
}

// NewServiceDescribe constructs the describe screen; Init triggers the load.
func NewServiceDescribe(client *awsx.EcsClient, cluster, name string) ServiceDescribeModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return ServiceDescribeModel{
		client:  client,
		cluster: cluster,
		name:    name,
		vp:      viewport.New(0, 0),
		spinner: sp,
		loading: true,
	}
}

// Init kicks off the first describe call.
func (m ServiceDescribeModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadServiceDescribeCmd(m.client, m.cluster, m.name))
}

// Name returns the service name.
func (m ServiceDescribeModel) Name() string { return m.name }

// SetSize sizes the viewport (1-line title + 1-line footer).
func (m *ServiceDescribeModel) SetSize(w, h int) {
	m.width, m.height = w, h
	body := h - 2
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
	if m.svc != nil {
		m.vp.SetContent(renderService(m.svc))
	}
}

// Update handles describe results + key input.
func (m ServiceDescribeModel) Update(msg tea.Msg) (ServiceDescribeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case serviceDescribeLoadedMsg:
		if msg.name != m.name {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.svc = msg.svc
		m.vp.SetContent(renderService(msg.svc))
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
			return m, tea.Batch(m.spinner.Tick, loadServiceDescribeCmd(m.client, m.cluster, m.name))
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the describe screen.
func (m ServiceDescribeModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("service: " + m.name)
	body := m.vp.View()
	if m.loading {
		body = fmt.Sprintf("%s describing %s…", m.spinner.View(), m.name)
	} else if m.err != "" {
		body = errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	footer := faint("r refresh · esc back")
	return title + "\n" + body + "\n" + footer
}

// renderService formats a Service as a multi-section string for the viewport.
func renderService(s *ecstypes.Service) string {
	if s == nil {
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
	row("status", deref(s.Status))
	row("desired", fmt.Sprintf("%d", s.DesiredCount))
	row("running", fmt.Sprintf("%d", s.RunningCount))
	row("pending", fmt.Sprintf("%d", s.PendingCount))
	row("launch-type", string(s.LaunchType))
	row("scheduling", string(s.SchedulingStrategy))
	if s.TaskDefinition != nil {
		row("task-def", taskDefTail(*s.TaskDefinition))
	}
	if s.RoleArn != nil {
		row("role", *s.RoleArn)
	}
	if s.CreatedAt != nil {
		row("created", s.CreatedAt.Format(time.RFC3339))
	}
	if s.ServiceArn != nil {
		row("arn", *s.ServiceArn)
	}

	// Deployments
	if len(s.Deployments) > 0 {
		hdr(fmt.Sprintf("Deployments (%d)", len(s.Deployments)))
		for _, d := range s.Deployments {
			b.WriteString("  • " + deref(d.Status))
			if d.RolloutState != "" {
				b.WriteString(" [" + string(d.RolloutState) + "]")
			}
			b.WriteString("\n")
			row("  running/desired", fmt.Sprintf("%d/%d (pending %d)", d.RunningCount, d.DesiredCount, d.PendingCount))
			if d.TaskDefinition != nil {
				row("  task-def", taskDefTail(*d.TaskDefinition))
			}
			if d.RolloutStateReason != nil {
				row("  reason", *d.RolloutStateReason)
			}
			if d.CreatedAt != nil {
				row("  created", d.CreatedAt.Format(time.RFC3339))
			}
		}
	}

	// Network
	if s.NetworkConfiguration != nil && s.NetworkConfiguration.AwsvpcConfiguration != nil {
		v := s.NetworkConfiguration.AwsvpcConfiguration
		hdr("Network (awsvpc)")
		row("subnets", strings.Join(v.Subnets, ", "))
		if len(v.SecurityGroups) > 0 {
			row("security-groups", strings.Join(v.SecurityGroups, ", "))
		}
		row("public-ip", string(v.AssignPublicIp))
	}

	// Load balancers
	if len(s.LoadBalancers) > 0 {
		hdr(fmt.Sprintf("Load Balancers (%d)", len(s.LoadBalancers)))
		for _, lb := range s.LoadBalancers {
			target := deref(lb.ContainerName)
			if lb.ContainerPort != nil {
				target += fmt.Sprintf(":%d", *lb.ContainerPort)
			}
			row("container", target)
			if lb.TargetGroupArn != nil {
				row("  target-group", *lb.TargetGroupArn)
			}
		}
	}

	// Recent events
	if len(s.Events) > 0 {
		hdr("Recent Events")
		max := 10
		for i, e := range s.Events {
			if i >= max {
				break
			}
			ts := ""
			if e.CreatedAt != nil {
				ts = e.CreatedAt.Format("15:04:05")
			}
			b.WriteString("  " + kSty.Render(ts) + "  " + deref(e.Message) + "\n")
		}
	}

	return b.String()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// taskDefTail trims a task-definition ARN to its family:revision tail.
func taskDefTail(arn string) string {
	if i := strings.LastIndexByte(arn, '/'); i >= 0 {
		return arn[i+1:]
	}
	return arn
}
