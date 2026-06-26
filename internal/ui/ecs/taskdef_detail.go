package ecs

import (
	"context"
	"encoding/json"
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

// taskDefDescribeLoadedMsg carries the result of DescribeTaskDef.
type taskDefDescribeLoadedMsg struct {
	family string
	td     *ecstypes.TaskDefinition
	err    error
}

func loadTaskDefDescribeCmd(client *awsx.EcsClient, family string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return taskDefDescribeLoadedMsg{family: family, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		td, err := client.DescribeTaskDef(ctx, family)
		return taskDefDescribeLoadedMsg{family: family, td: td, err: err}
	}
}

// TaskDefDescribeModel renders one task definition, with a raw-JSON toggle.
type TaskDefDescribeModel struct {
	client  *awsx.EcsClient
	family  string
	td      *ecstypes.TaskDefinition
	showRaw bool
	vp      viewport.Model
	spinner spinner.Model
	loading bool
	err     string
	width   int
	height  int
}

// NewTaskDefDescribe constructs the describe screen; Init triggers the load.
func NewTaskDefDescribe(client *awsx.EcsClient, family string) TaskDefDescribeModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return TaskDefDescribeModel{
		client:  client,
		family:  family,
		vp:      viewport.New(0, 0),
		spinner: sp,
		loading: true,
	}
}

// Init kicks off the first describe call.
func (m TaskDefDescribeModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadTaskDefDescribeCmd(m.client, m.family))
}

// SetSize sizes the viewport (1-line title + 1-line footer).
func (m *TaskDefDescribeModel) SetSize(w, h int) {
	m.width, m.height = w, h
	body := h - 2
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
	if m.td != nil {
		m.vp.SetContent(m.content())
	}
}

// content renders either the formatted sections or the raw JSON.
func (m TaskDefDescribeModel) content() string {
	if m.showRaw {
		return renderTaskDefJSON(m.td)
	}
	return renderTaskDef(m.td)
}

// Update handles describe results + key input.
func (m TaskDefDescribeModel) Update(msg tea.Msg) (TaskDefDescribeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case taskDefDescribeLoadedMsg:
		if msg.family != m.family {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.td = msg.td
		m.vp.SetContent(m.content())
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
		switch msg.String() {
		case "r":
			m.loading = true
			m.err = ""
			return m, tea.Batch(m.spinner.Tick, loadTaskDefDescribeCmd(m.client, m.family))
		case "J":
			m.showRaw = !m.showRaw
			if m.td != nil {
				m.vp.SetContent(m.content())
				m.vp.GotoTop()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the describe screen.
func (m TaskDefDescribeModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("task-def: " + m.family)
	if m.showRaw {
		title += "  " + faint("[raw json]")
	}
	body := m.vp.View()
	if m.loading {
		body = fmt.Sprintf("%s describing %s…", m.spinner.View(), m.family)
	} else if m.err != "" {
		body = errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	footer := faint("J json · r refresh · esc back")
	return title + "\n" + body + "\n" + footer
}

func renderTaskDefJSON(td *ecstypes.TaskDefinition) string {
	if td == nil {
		return faint("(no task definition)")
	}
	out, err := json.MarshalIndent(td, "", "  ")
	if err != nil {
		return errStyle.Render("json error: " + err.Error())
	}
	return string(out)
}

// renderTaskDef formats a TaskDefinition as sectioned text for the viewport.
func renderTaskDef(td *ecstypes.TaskDefinition) string {
	if td == nil {
		return faint("(no task definition)")
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
	row("family", deref(td.Family))
	row("revision", fmt.Sprintf("%d", td.Revision))
	row("status", string(td.Status))
	row("cpu", deref(td.Cpu))
	row("memory", deref(td.Memory))
	row("network-mode", string(td.NetworkMode))
	if len(td.RequiresCompatibilities) > 0 {
		compat := make([]string, 0, len(td.RequiresCompatibilities))
		for _, c := range td.RequiresCompatibilities {
			compat = append(compat, string(c))
		}
		row("requires-compat", strings.Join(compat, ", "))
	}
	if td.TaskRoleArn != nil {
		row("task-role", *td.TaskRoleArn)
	}
	if td.ExecutionRoleArn != nil {
		row("execution-role", *td.ExecutionRoleArn)
	}

	// Container definitions
	if len(td.ContainerDefinitions) > 0 {
		hdr(fmt.Sprintf("Containers (%d)", len(td.ContainerDefinitions)))
		for _, cd := range td.ContainerDefinitions {
			b.WriteString("  • " + deref(cd.Name) + "\n")
			row("  image", deref(cd.Image))
			essential := "false"
			if cd.Essential != nil && *cd.Essential {
				essential = "true"
			}
			row("  essential", essential)
			if cd.Cpu > 0 {
				row("  cpu", fmt.Sprintf("%d", cd.Cpu))
			}
			if cd.Memory != nil {
				row("  memory", fmt.Sprintf("%d", *cd.Memory))
			}
			for _, pm := range cd.PortMappings {
				cp, hp := int32(0), int32(0)
				if pm.ContainerPort != nil {
					cp = *pm.ContainerPort
				}
				if pm.HostPort != nil {
					hp = *pm.HostPort
				}
				row("  port", fmt.Sprintf("%d:%d/%s", hp, cp, string(pm.Protocol)))
			}
			if len(cd.Environment) > 0 {
				row("  env", fmt.Sprintf("%d vars", len(cd.Environment)))
				for _, kv := range cd.Environment {
					row("    "+deref(kv.Name), deref(kv.Value))
				}
			}
			if len(cd.Secrets) > 0 {
				row("  secrets", fmt.Sprintf("%d", len(cd.Secrets)))
				for _, sc := range cd.Secrets {
					row("    "+deref(sc.Name), deref(sc.ValueFrom))
				}
			}
			for _, mp := range cd.MountPoints {
				ro := ""
				if mp.ReadOnly != nil && *mp.ReadOnly {
					ro = " (ro)"
				}
				row("  mount", deref(mp.SourceVolume)+" -> "+deref(mp.ContainerPath)+ro)
			}
		}
	}

	// Volumes
	if len(td.Volumes) > 0 {
		hdr(fmt.Sprintf("Volumes (%d)", len(td.Volumes)))
		for _, v := range td.Volumes {
			row("name", deref(v.Name))
		}
	}

	return b.String()
}
