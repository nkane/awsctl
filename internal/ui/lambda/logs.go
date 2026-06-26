package lambda

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
)

// pollInterval controls how often the tail re-queries CloudWatch.
const pollInterval = 2 * time.Second

// tailPageMsg carries one page of fetched events.
type tailPageMsg struct {
	page *awsx.FilterPage
	err  error
}

// tailTickMsg fires every pollInterval to trigger a new fetch in follow mode.
type tailTickMsg time.Time

// tailFetchCmd runs Filter once.
func tailFetchCmd(client *awsx.LogsClient, in awsx.FilterInput) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return tailPageMsg{err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		p, err := client.Filter(ctx, in)
		return tailPageMsg{page: p, err: err}
	}
}

// tailTickCmd schedules the next poll.
func tailTickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg { return tailTickMsg(t) })
}

// LogsModel is a CloudWatch log tail screen. It is used for Lambda functions
// (NewLogs) and, via NewLogsFor, for any log group + stream prefix — e.g. ECS
// container logs. (Shared here for now; a neutral logsview package would be a
// cleaner home if a third consumer appears.)
type LogsModel struct {
	client       *awsx.LogsClient
	logGroup     string
	streamPrefix string
	label        string

	vp      viewport.Model
	spinner spinner.Model
	filter  textinput.Model

	// All events seen so far, sorted by timestamp.
	events []awsx.LogEvent
	// Highest timestamp seen; next poll asks for events strictly after this.
	lastSeen int64

	follow         bool
	loading        bool
	err            string
	width, height  int
	filteringInput bool   // true when '/' input has focus
	pattern        string // applied server-side filter pattern
}

// NewLogs constructs the tail screen for one function. Default 5-minute window.
func NewLogs(client *awsx.LogsClient, fnName string) LogsModel {
	return newLogsModel(client, fnName, awsx.LambdaLogGroup(fnName), "")
}

// NewLogsFor constructs a tail screen for an arbitrary log group, optionally
// scoped to a log-stream prefix. Used for ECS container logs. Default 5-minute
// window.
func NewLogsFor(client *awsx.LogsClient, label, logGroup, streamPrefix string) LogsModel {
	return newLogsModel(client, label, logGroup, streamPrefix)
}

func newLogsModel(client *awsx.LogsClient, label, logGroup, streamPrefix string) LogsModel {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	ti := textinput.New()
	ti.Placeholder = "filter pattern (e.g. ERROR)"
	ti.CharLimit = 256

	return LogsModel{
		client:       client,
		logGroup:     logGroup,
		streamPrefix: streamPrefix,
		label:        label,
		vp:           vp,
		spinner:      sp,
		filter:       ti,
		follow:       true,
		// 5 min window
		lastSeen: time.Now().Add(-5*time.Minute).UnixMilli() - 1,
	}
}

// Init kicks off the first fetch + poll tick.
func (m LogsModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetch(), tailTickCmd())
}

// FilterFocused reports whether the filter input owns key input.
func (m LogsModel) FilterFocused() bool { return m.filteringInput }

// SetSize sizes the viewport (header 2 lines, footer 1 line).
func (m *LogsModel) SetSize(w, h int) {
	m.width, m.height = w, h
	body := h - 3
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
	m.filter.Width = w - 12
}

// fetch issues a Filter call from m.lastSeen forward.
func (m LogsModel) fetch() tea.Cmd {
	in := awsx.FilterInput{
		LogGroup:        m.logGroup,
		StartMillis:     m.lastSeen + 1,
		Limit:           1000,
		FilterPattern:   m.pattern,
		LogStreamPrefix: m.streamPrefix,
	}
	return tailFetchCmd(m.client, in)
}

// Update handles ticks, key input, and fetch results.
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tailPageMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		if msg.page != nil && len(msg.page.Events) > 0 {
			m.appendEvents(msg.page.Events)
			m.vp.SetContent(m.renderEvents())
			if m.follow {
				m.vp.GotoBottom()
			}
		}
		return m, nil

	case tailTickMsg:
		if !m.follow || m.filteringInput {
			return m, tailTickCmd()
		}
		m.loading = true
		return m, tea.Batch(m.fetch(), tailTickCmd())

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// Filter input owns most keys when active.
		if m.filteringInput {
			switch msg.String() {
			case "enter":
				m.pattern = strings.TrimSpace(m.filter.Value())
				m.filteringInput = false
				m.filter.Blur()
				// Reset window so the new filter applies to the same 5-min lookback.
				m.lastSeen = time.Now().Add(-5*time.Minute).UnixMilli() - 1
				m.events = nil
				m.vp.SetContent("")
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.fetch())
			case "esc":
				m.filteringInput = false
				m.filter.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "f":
			m.follow = !m.follow
			return m, nil
		case "g":
			m.vp.GotoTop()
			return m, nil
		case "G":
			m.vp.GotoBottom()
			return m, nil
		case "r":
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.fetch())
		case "c":
			// Clear buffer; keep lastSeen so we don't re-fetch old events.
			m.events = nil
			m.vp.SetContent("")
			return m, nil
		case "/":
			m.filteringInput = true
			m.filter.SetValue(m.pattern)
			m.filter.Focus()
			return m, textinput.Blink
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// appendEvents merges new events into m.events, dedup by (ts,stream,msg)
// and updates lastSeen.
func (m *LogsModel) appendEvents(in []awsx.LogEvent) {
	seen := make(map[string]struct{}, len(m.events))
	for _, e := range m.events {
		seen[eventKey(e)] = struct{}{}
	}
	for _, e := range in {
		if _, dup := seen[eventKey(e)]; dup {
			continue
		}
		m.events = append(m.events, e)
		if e.Timestamp > m.lastSeen {
			m.lastSeen = e.Timestamp
		}
	}
}

func eventKey(e awsx.LogEvent) string {
	return fmt.Sprintf("%d|%s|%s", e.Timestamp, e.Stream, e.Message)
}

// renderEvents formats every buffered event as a single string.
func (m LogsModel) renderEvents() string {
	if len(m.events) == 0 {
		return faint("(no events yet)")
	}
	tsSty := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	streamSty := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	var b strings.Builder
	for _, e := range m.events {
		ts := time.UnixMilli(e.Timestamp).Format("15:04:05.000")
		stream := e.Stream
		if len(stream) > 20 {
			stream = stream[:20]
		}
		msg := strings.TrimRight(e.Message, "\n")
		b.WriteString(tsSty.Render(ts))
		b.WriteString(" ")
		b.WriteString(streamSty.Render(stream))
		b.WriteString(" ")
		b.WriteString(msg)
		b.WriteString("\n")
	}
	return b.String()
}

// View renders title bar + viewport + footer.
func (m LogsModel) View() string {
	titleSty := lipgloss.NewStyle().Bold(true)
	statusSty := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	followLbl := "PAUSED"
	followCol := lipgloss.Color("203")
	if m.follow {
		followLbl = "FOLLOWING"
		followCol = lipgloss.Color("42")
	}
	followBadge := lipgloss.NewStyle().Foreground(followCol).Bold(true).Render(followLbl)

	patternBit := ""
	if m.pattern != "" {
		patternBit = "  filter=" + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(m.pattern)
	}
	loadingBit := ""
	if m.loading {
		loadingBit = "  " + m.spinner.View()
	}
	header := titleSty.Render("logs: "+m.label) + "  " +
		followBadge + statusSty.Render("  group="+m.logGroup) + patternBit + loadingBit

	body := m.vp.View()
	if m.err != "" {
		body = errStyle.Render("error: "+m.err) + "\n\n" + body
	}

	footer := faint("f follow · g/G top/bottom · r refresh · c clear · / filter · esc back")
	if m.filteringInput {
		footer = "filter: " + m.filter.View() + "  " + faint("(enter apply · esc cancel)")
	}

	return header + "\n" + body + "\n" + footer
}
