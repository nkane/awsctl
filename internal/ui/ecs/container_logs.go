package ecs

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	awsx "github.com/nkane/awsctl/internal/aws"
	lambdaui "github.com/nkane/awsctl/internal/ui/lambda"
)

// containerLogTargetMsg carries the resolved awslogs target for a container.
type containerLogTargetMsg struct {
	container string
	target    *awsx.ContainerLogTarget
	err       error
}

func resolveContainerLogCmd(client *awsx.EcsClient, cluster, task, container string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return containerLogTargetMsg{container: container, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		tgt, err := client.ResolveContainerLog(ctx, cluster, task, container)
		return containerLogTargetMsg{container: container, target: tgt, err: err}
	}
}

// ContainerLogsModel tails a container's CloudWatch logs. It first resolves the
// awslogs group + stream prefix from the task definition, then delegates to the
// shared log viewer (lambda.LogsModel via NewLogsFor).
type ContainerLogsModel struct {
	ecs       *awsx.EcsClient
	logs      *awsx.LogsClient
	cluster   string
	task      string
	container string

	inner     *lambdaui.LogsModel // nil until the target resolves
	resolving bool
	err       string
	spinner   spinner.Model
	width     int
	height    int
}

// NewContainerLogs constructs the container log screen.
func NewContainerLogs(ecsClient *awsx.EcsClient, logsClient *awsx.LogsClient, cluster, task, container string) ContainerLogsModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return ContainerLogsModel{
		ecs:       ecsClient,
		logs:      logsClient,
		cluster:   cluster,
		task:      task,
		container: container,
		resolving: true,
		spinner:   sp,
	}
}

// Init kicks off log-target resolution.
func (m ContainerLogsModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, resolveContainerLogCmd(m.ecs, m.cluster, m.task, m.container))
}

// FilterFocused reports whether the inner viewer's filter owns key input.
func (m ContainerLogsModel) FilterFocused() bool {
	return m.inner != nil && m.inner.FilterFocused()
}

// SetSize sizes the screen (and the inner viewer once resolved).
func (m *ContainerLogsModel) SetSize(w, h int) {
	m.width, m.height = w, h
	if m.inner != nil {
		m.inner.SetSize(w, h)
	}
}

// Update resolves the target, then forwards to the inner viewer.
func (m ContainerLogsModel) Update(msg tea.Msg) (ContainerLogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case containerLogTargetMsg:
		if msg.container != m.container {
			return m, nil
		}
		m.resolving = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		lv := lambdaui.NewLogsFor(m.logs, m.container, msg.target.LogGroup, msg.target.StreamPrefix)
		lv.SetSize(m.width, m.height)
		m.inner = &lv
		return m, m.inner.Init()

	case spinner.TickMsg:
		if !m.resolving {
			if m.inner != nil {
				lv, cmd := m.inner.Update(msg)
				m.inner = &lv
				return m, cmd
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if m.inner != nil {
		lv, cmd := m.inner.Update(msg)
		m.inner = &lv
		return m, cmd
	}
	return m, nil
}

// View renders the resolving state, an error, or the inner log viewer.
func (m ContainerLogsModel) View() string {
	if m.resolving {
		return fmt.Sprintf("%s resolving log config for %s…", m.spinner.View(), m.container)
	}
	if m.err != "" {
		return errStyle.Render("error: "+m.err) + "\n\n" + faint("esc back")
	}
	if m.inner != nil {
		return m.inner.View()
	}
	return faint("(no logs)")
}
