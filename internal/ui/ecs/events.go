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

	awsx "github.com/nkane/awsctl/internal/aws"
)

// eventsLoadedMsg carries fetched service events.
type eventsLoadedMsg struct {
	service string
	events  []awsx.EventSummary
	err     error
}

func loadEventsCmd(client *awsx.EcsClient, cluster, service string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return eventsLoadedMsg{service: service, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		evs, err := client.ServiceEvents(ctx, cluster, service)
		return eventsLoadedMsg{service: service, events: evs, err: err}
	}
}

// ServiceEventsModel is a scrollable, newest-first service events panel.
type ServiceEventsModel struct {
	client  *awsx.EcsClient
	cluster string
	service string
	events  []awsx.EventSummary
	vp      viewport.Model
	spinner spinner.Model
	loading bool
	loaded  bool
	err     string
	width   int
	height  int
}

// NewServiceEvents constructs the events panel; Init triggers the load.
func NewServiceEvents(client *awsx.EcsClient, cluster, service string) ServiceEventsModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return ServiceEventsModel{
		client:  client,
		cluster: cluster,
		service: service,
		vp:      viewport.New(0, 0),
		spinner: sp,
		loading: true,
	}
}

// Init kicks off the first fetch.
func (m ServiceEventsModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadEventsCmd(m.client, m.cluster, m.service))
}

// SetSize sizes the viewport (1-line title + 1-line footer).
func (m *ServiceEventsModel) SetSize(w, h int) {
	m.width, m.height = w, h
	body := h - 2
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
	if m.loaded {
		m.vp.SetContent(m.render())
	}
}

// Update handles fetch results + key input.
func (m ServiceEventsModel) Update(msg tea.Msg) (ServiceEventsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case eventsLoadedMsg:
		if msg.service != m.service {
			return m, nil
		}
		m.loading = false
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.events = msg.events
		m.vp.SetContent(m.render())
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
			return m, tea.Batch(m.spinner.Tick, loadEventsCmd(m.client, m.cluster, m.service))
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the events panel.
func (m ServiceEventsModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("events: " + m.service)
	body := m.vp.View()
	if m.loading && !m.loaded {
		body = fmt.Sprintf("%s loading events…", m.spinner.View())
	} else if m.err != "" {
		body = errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	} else if m.loaded && len(m.events) == 0 {
		body = faint("no events for this service.")
	}
	footer := faint("g/G top/bottom · r refresh · esc back")
	return title + "\n" + body + "\n" + footer
}

// render formats every event, newest-first.
func (m ServiceEventsModel) render() string {
	if len(m.events) == 0 {
		return faint("(no events)")
	}
	tsSty := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	var b strings.Builder
	for _, e := range m.events {
		b.WriteString(tsSty.Render(e.CreatedAt) + "  " + strings.TrimRight(e.Message, "\n") + "\n")
	}
	return b.String()
}
