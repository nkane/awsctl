package ecs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	awsx "github.com/nkane/awsctl/internal/aws"
	"github.com/nkane/awsctl/internal/ui/core"
)

// editMode identifies which inline write-input field, if any, owns input on the
// service-describe screen.
type editMode int

const (
	editNone editMode = iota
	editScale
	editTaskDef
)

// rolloutPollInterval controls how often the describe re-fetches while a
// deployment is in progress.
const rolloutPollInterval = 3 * time.Second

// serviceDescribeLoadedMsg carries the result of DescribeService.
type serviceDescribeLoadedMsg struct {
	name string
	svc  *ecstypes.Service
	err  error
}

// rolloutTickMsg fires while watching an in-progress deployment.
type rolloutTickMsg time.Time

func rolloutTickCmd() tea.Cmd {
	return tea.Tick(rolloutPollInterval, func(t time.Time) tea.Msg { return rolloutTickMsg(t) })
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
	client   *awsx.EcsClient
	cluster  string
	name     string
	svc      *ecstypes.Service
	vp       viewport.Model
	spinner  spinner.Model
	loading  bool
	watching bool // auto-polling an in-progress rollout
	err      string
	width    int
	height   int
	section  int // 0 = info, 1 = metrics
	metrics  metricsPanel

	// Inline write-input (scale / update-revision) state.
	editing    editMode
	editPrompt string
	input      textinput.Model
	notice     string // success line shown after a confirmed mutation
}

// service describe section indices.
const (
	sectionInfo = iota
	sectionMetrics
)

var serviceSectionNames = []string{"Info", "Metrics"}

// NewServiceDescribe constructs the describe screen; Init triggers the load. cfg
// is used to build the CloudWatch client for the Metrics section.
func NewServiceDescribe(client *awsx.EcsClient, cfg *awsx.Config, cluster, name string) ServiceDescribeModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return ServiceDescribeModel{
		client:  client,
		cluster: cluster,
		name:    name,
		vp:      viewport.New(0, 0),
		spinner: sp,
		loading: true,
		metrics: newMetricsPanel(cfg, ecsMetricSpecs(cluster, name)),
	}
}

// Init kicks off the first describe call.
func (m ServiceDescribeModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadServiceDescribeCmd(m.client, m.cluster, m.name))
}

// Name returns the service name.
func (m ServiceDescribeModel) Name() string { return m.name }

// Cluster returns the owning cluster name.
func (m ServiceDescribeModel) Cluster() string { return m.cluster }

// Editing reports whether an inline write-input field owns input. The App uses
// this to forward every key (and esc) to the screen while editing.
func (m ServiceDescribeModel) Editing() bool { return m.editing != editNone }

// SetSize sizes the viewport (1-line title + 1-line footer).
func (m *ServiceDescribeModel) SetSize(w, h int) {
	m.width, m.height = w, h
	// title + section strip + footer = 3 lines of chrome.
	body := h - 3
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
	// The metrics section renders its own selector line inside body.
	mh := body - 1
	if mh < 4 {
		mh = 4
	}
	m.metrics.SetSize(w, mh)
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
		// Watch while the primary deployment is still rolling out; start the
		// poll loop on the transition from not-watching to watching.
		wasWatching := m.watching
		m.watching = isRollingOut(msg.svc)
		if m.watching && !wasWatching {
			return m, rolloutTickCmd()
		}
		return m, nil

	case ecsWriteDoneMsg:
		// React only to writes that targeted this service (scale / force-deploy
		// / update-task-def all use the service name as the target).
		if msg.target != m.name {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err.Error()
			m.notice = ""
			return m, nil
		}
		m.err = ""
		m.notice = noticeFor(msg.action, msg.target)
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, loadServiceDescribeCmd(m.client, m.cluster, m.name))

	case rolloutTickMsg:
		if !m.watching {
			return m, nil // rollout finished; stop polling
		}
		return m, tea.Batch(loadServiceDescribeCmd(m.client, m.cluster, m.name), rolloutTickCmd())

	case metricsLoadedMsg:
		return m, m.metrics.Update(msg)

	case spinner.TickMsg:
		var cmds []tea.Cmd
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.metrics.Update(msg))
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		// While an inline write-input field is open it owns every key.
		if m.editing != editNone {
			return m.updateEditing(msg)
		}
		// In the metrics section the range keys drive the TimeRange selector.
		if m.section == sectionMetrics {
			switch msg.String() {
			case "left", "right", "h", "l":
				return m, m.metrics.Update(msg)
			}
		}
		switch msg.String() {
		case "tab", "shift+tab":
			m.section = (m.section + 1) % len(serviceSectionNames)
			if m.section == sectionMetrics {
				return m, m.metrics.startIfNeeded()
			}
			return m, nil
		case "r":
			if m.section == sectionMetrics {
				return m, m.metrics.start()
			}
			m.loading = true
			m.err = ""
			return m, tea.Batch(m.spinner.Tick, loadServiceDescribeCmd(m.client, m.cluster, m.name))
		case "S": // scale desired count (#57)
			cur := ""
			if m.svc != nil {
				cur = strconv.Itoa(int(m.svc.DesiredCount))
			}
			m.startEdit(editScale, "new desired count", cur)
			return m, textinput.Blink
		case "F": // force a new deployment (#58)
			run := forceDeployCmd(m.client, m.cluster, m.name)
			return m, core.ConfirmRequest(
				"Force new deployment",
				"Start a new rolling deployment of "+m.name+"?",
				actionForceDeploy, m.name, run,
			)
		case "U": // update task-def revision (#60)
			fam := m.currentFamily()
			if fam == "" {
				m.err = "current task-def family unknown — refresh first"
				return m, nil
			}
			m.startEdit(editTaskDef, "new revision for "+fam, m.currentRevision())
			return m, textinput.Blink
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// startEdit opens an inline numeric input seeded with initial.
func (m *ServiceDescribeModel) startEdit(mode editMode, prompt, initial string) {
	in := textinput.New()
	in.Prompt = "  "
	in.CharLimit = 16
	in.SetValue(initial)
	in.CursorEnd()
	in.Focus()
	m.input = in
	m.editing = mode
	m.editPrompt = prompt
	m.err = ""
	m.notice = ""
}

// updateEditing handles keys while an inline input owns the screen: esc cancels,
// enter submits, everything else edits the field.
func (m ServiceDescribeModel) updateEditing(msg tea.KeyMsg) (ServiceDescribeModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editing = editNone
		m.input.Blur()
		m.err = ""
		return m, nil
	case "enter":
		return m.submitEdit()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// submitEdit validates the field and emits a gated mutation, or sets an error.
func (m ServiceDescribeModel) submitEdit() (ServiceDescribeModel, tea.Cmd) {
	val := strings.TrimSpace(m.input.Value())
	switch m.editing {
	case editScale:
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			m.err = "desired count must be a non-negative integer"
			return m, nil
		}
		m.editing = editNone
		m.input.Blur()
		run := scaleCmd(m.client, m.cluster, m.name, int32(n))
		return m, core.ConfirmRequest(
			"Scale service",
			fmt.Sprintf("Set %s desired count to %d?", m.name, n),
			actionScale, m.name, run,
		)
	case editTaskDef:
		rev, err := strconv.Atoi(val)
		if err != nil || rev <= 0 {
			m.err = "revision must be a positive integer"
			return m, nil
		}
		fam := m.currentFamily()
		if fam == "" {
			m.err = "current task-def family unknown — refresh first"
			return m, nil
		}
		taskDef := fam + ":" + strconv.Itoa(rev)
		m.editing = editNone
		m.input.Blur()
		run := updateTaskDefCmd(m.client, m.cluster, m.name, taskDef)
		return m, core.ConfirmRequest(
			"Update task definition",
			fmt.Sprintf("Point %s at %s?", m.name, taskDef),
			actionUpdateTaskDef, m.name, run,
		)
	}
	return m, nil
}

// currentFamily returns the family of the service's current task definition.
func (m ServiceDescribeModel) currentFamily() string {
	if m.svc == nil || m.svc.TaskDefinition == nil {
		return ""
	}
	return familyOf(taskDefTail(*m.svc.TaskDefinition))
}

// currentRevision returns the revision of the service's current task definition.
func (m ServiceDescribeModel) currentRevision() string {
	if m.svc == nil || m.svc.TaskDefinition == nil {
		return ""
	}
	tail := taskDefTail(*m.svc.TaskDefinition)
	if i := strings.LastIndexByte(tail, ':'); i >= 0 {
		return tail[i+1:]
	}
	return ""
}

// editView renders the inline input form.
func (m ServiceDescribeModel) editView() string {
	title := lipgloss.NewStyle().Bold(true).Render(m.editPrompt + ":")
	body := title + "\n" + m.input.View()
	if m.err != "" {
		body += "\n\n" + errStyle.Render("error: "+m.err)
	}
	return body
}

// View renders the describe screen.
func (m ServiceDescribeModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("service: " + m.name)
	if rl := rolloutLine(m.svc); rl != "" {
		title += "  " + rl
	}

	var body string
	switch {
	case m.editing != editNone:
		body = m.editView()
	case m.section == sectionMetrics:
		body = m.metrics.View()
	default:
		body = m.vp.View()
		if m.loading {
			body = fmt.Sprintf("%s describing %s…", m.spinner.View(), m.name)
		} else if m.err != "" {
			body = errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
		}
	}

	var footer string
	switch {
	case m.editing != editNone:
		footer = faint("enter confirm · esc cancel")
	case m.watching:
		footer = faint("● watching rollout · tab section · S scale · F force-deploy · U update-rev · r refresh · esc back")
	default:
		footer = faint("tab section · S scale · F force-deploy · U update-rev · r refresh · esc back")
	}
	if m.notice != "" && m.editing == editNone {
		footer = noticeStyle.Render("✓ "+m.notice) + "  " + footer
	}
	return title + "\n" + m.sectionStrip() + "\n" + body + "\n" + footer
}

// sectionStrip renders the Info / Metrics section tabs.
func (m ServiceDescribeModel) sectionStrip() string {
	active := lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63")).Padding(0, 1)
	inactive := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	parts := make([]string, len(serviceSectionNames))
	for i, n := range serviceSectionNames {
		if i == m.section {
			parts[i] = active.Render(n)
		} else {
			parts[i] = inactive.Render(n)
		}
	}
	return strings.Join(parts, " ")
}

// primaryDeployment returns the service's PRIMARY deployment, or nil.
func primaryDeployment(s *ecstypes.Service) *ecstypes.Deployment {
	if s == nil {
		return nil
	}
	for i := range s.Deployments {
		if s.Deployments[i].Status != nil && *s.Deployments[i].Status == "PRIMARY" {
			return &s.Deployments[i]
		}
	}
	return nil
}

// isRollingOut reports whether the primary deployment is still in progress.
func isRollingOut(s *ecstypes.Service) bool {
	d := primaryDeployment(s)
	return d != nil && d.RolloutState == ecstypes.DeploymentRolloutStateInProgress
}

// rolloutLine renders the primary deployment's state + a progress bar, or "".
func rolloutLine(s *ecstypes.Service) string {
	d := primaryDeployment(s)
	if d == nil {
		return ""
	}
	state := string(d.RolloutState)
	col := lipgloss.Color("244")
	switch d.RolloutState {
	case ecstypes.DeploymentRolloutStateInProgress:
		col = lipgloss.Color("214")
	case ecstypes.DeploymentRolloutStateCompleted:
		col = lipgloss.Color("42")
	case ecstypes.DeploymentRolloutStateFailed:
		col = lipgloss.Color("203")
	}
	badge := lipgloss.NewStyle().Foreground(col).Bold(true).Render(state)
	return fmt.Sprintf("rollout %s %s %d/%d (pending %d)",
		badge, rolloutBar(d.RunningCount, d.DesiredCount), d.RunningCount, d.DesiredCount, d.PendingCount)
}

// rolloutBar renders a 10-cell ASCII progress bar for running/desired.
func rolloutBar(running, desired int32) string {
	const width = 10
	filled := width
	if desired > 0 {
		filled = int(float64(running) / float64(desired) * float64(width))
		if filled > width {
			filled = width
		}
		if filled < 0 {
			filled = 0
		}
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
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
