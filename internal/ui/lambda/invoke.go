package lambda

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
)

// invokeDoneMsg carries the InvokeResult or an error.
type invokeDoneMsg struct {
	r       *awsx.InvokeResult
	err     error
	elapsed time.Duration
}

// InvokeCmd runs Invoke in the background.
func InvokeCmd(client *awsx.LambdaClient, name string, payload []byte) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return invokeDoneMsg{err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		start := time.Now()
		r, err := client.Invoke(ctx, name, payload)
		return invokeDoneMsg{r: r, err: err, elapsed: time.Since(start)}
	}
}

// InvokeModel is the Lambda invoke screen: editor on top, response below.
type InvokeModel struct {
	client  *awsx.LambdaClient
	name    string
	editor  textarea.Model
	resp    viewport.Model
	spinner spinner.Model

	width, height int
	running       bool
	err           string

	lastResult  *awsx.InvokeResult
	lastElapsed time.Duration
	focusEditor bool
}

// NewInvoke constructs the invoke screen for one function. Loads the last
// payload from disk if one exists.
func NewInvoke(client *awsx.LambdaClient, name string) InvokeModel {
	ed := textarea.New()
	ed.Placeholder = `{}`
	ed.ShowLineNumbers = true
	ed.CharLimit = 0 // unlimited
	ed.Focus()

	if cached := loadCachedPayload(name); cached != "" {
		ed.SetValue(cached)
	} else {
		ed.SetValue("{}")
	}

	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return InvokeModel{
		client:      client,
		name:        name,
		editor:      ed,
		resp:        vp,
		spinner:     sp,
		focusEditor: true,
	}
}

// SetSize splits available height: 60% editor, 40% response.
func (m *InvokeModel) SetSize(w, h int) {
	m.width, m.height = w, h
	// 1 line title, 1 line divider, 1 line help footer.
	body := h - 3
	if body < 6 {
		body = 6
	}
	editorH := body * 60 / 100
	respH := body - editorH
	m.editor.SetWidth(w)
	m.editor.SetHeight(editorH)
	m.resp.Width = w
	m.resp.Height = respH
}

// Update handles keys + invoke results.
func (m InvokeModel) Update(msg tea.Msg) (InvokeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case invokeDoneMsg:
		m.running = false
		m.lastElapsed = msg.elapsed
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.lastResult = msg.r
		m.resp.SetContent(formatResponse(msg.r, msg.elapsed))
		m.resp.GotoTop()
		return m, nil

	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.focusEditor = !m.focusEditor
			if m.focusEditor {
				m.editor.Focus()
			} else {
				m.editor.Blur()
			}
			return m, nil
		case "ctrl+r":
			// Run invoke. Validate JSON first.
			payload := strings.TrimSpace(m.editor.Value())
			if payload == "" {
				payload = "{}"
			}
			var any interface{}
			if err := json.Unmarshal([]byte(payload), &any); err != nil {
				m.err = "invalid JSON: " + err.Error()
				return m, nil
			}
			savePayloadCache(m.name, payload)
			m.running = true
			m.err = ""
			return m, tea.Batch(m.spinner.Tick, InvokeCmd(m.client, m.name, []byte(payload)))
		}
	}

	var cmd tea.Cmd
	if m.focusEditor {
		m.editor, cmd = m.editor.Update(msg)
	} else {
		m.resp, cmd = m.resp.Update(msg)
	}
	return m, cmd
}

// View renders title + editor + response + footer.
func (m InvokeModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("invoke: " + m.name)

	editorLabel := "payload (focused)"
	respLabel := "response"
	if !m.focusEditor {
		editorLabel = "payload"
		respLabel = "response (focused)"
	}
	labelSty := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)

	respBody := m.resp.View()
	if m.running {
		respBody = fmt.Sprintf("%s invoking…", m.spinner.View())
	} else if m.err != "" {
		respBody = errStyle.Render("error: "+m.err) + "\n\n" + m.resp.View()
	} else if m.lastResult == nil {
		respBody = faint("press ctrl+r to invoke")
	}

	footer := faint("ctrl+r run · tab switch focus · esc back")

	return strings.Join([]string{
		title,
		labelSty.Render(editorLabel),
		m.editor.View(),
		labelSty.Render(respLabel),
		respBody,
		footer,
	}, "\n")
}

// formatResponse renders an InvokeResult into the response viewport.
func formatResponse(r *awsx.InvokeResult, elapsed time.Duration) string {
	var b strings.Builder
	statusSty := lipgloss.NewStyle().Bold(true)
	if r.FunctionError != "" {
		statusSty = statusSty.Foreground(lipgloss.Color("203"))
	} else {
		statusSty = statusSty.Foreground(lipgloss.Color("42"))
	}
	fmt.Fprintf(&b, "%s  status=%d  duration=%s",
		statusSty.Render(statusLabel(r)),
		r.StatusCode, elapsed.Round(time.Millisecond))
	if r.ExecutedVersion != "" {
		fmt.Fprintf(&b, "  version=%s", r.ExecutedVersion)
	}
	b.WriteString("\n\n")

	// Pretty-print JSON payload when possible.
	if len(r.Payload) > 0 {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("payload:") + "\n")
		var pretty interface{}
		if err := json.Unmarshal(r.Payload, &pretty); err == nil {
			out, _ := json.MarshalIndent(pretty, "", "  ")
			b.Write(out)
		} else {
			b.Write(r.Payload)
		}
		b.WriteString("\n\n")
	}

	if r.LogResult != "" {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("log tail:") + "\n")
		if dec, err := base64.StdEncoding.DecodeString(r.LogResult); err == nil {
			b.Write(dec)
		} else {
			b.WriteString(r.LogResult)
		}
	}
	return b.String()
}

func statusLabel(r *awsx.InvokeResult) string {
	if r.FunctionError != "" {
		return "FUNCTION ERROR (" + r.FunctionError + ")"
	}
	return "OK"
}

// payloadCacheDir returns the directory used to persist last-used payloads.
// Honors XDG_CACHE_HOME, falls back to ~/.cache/awsctl/payloads.
func payloadCacheDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "awsctl", "payloads")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "awsctl", "payloads")
}

// payloadCachePath returns the cache file for one function name. Names may
// contain ':' (qualified ARN) so we replace path separators.
func payloadCachePath(name string) string {
	safe := strings.NewReplacer("/", "_", ":", "_").Replace(name)
	return filepath.Join(payloadCacheDir(), safe+".json")
}

func loadCachedPayload(name string) string {
	b, err := os.ReadFile(payloadCachePath(name))
	if err != nil {
		return ""
	}
	return string(b)
}

func savePayloadCache(name, payload string) {
	dir := payloadCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(payloadCachePath(name), []byte(payload), 0o644)
}
