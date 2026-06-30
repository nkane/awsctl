package lambda

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
	"github.com/nkane/awsctl/internal/ui/core"
)

// This file holds the Lambda write actions (M6 #32–#35). Every mutation is
// emitted through core.ConfirmRequest so the App gates it behind the confirm
// modal + audit log; the result message is handled here AND, for refresh, by the
// underlying detail/list screen (messages are broadcast to every screen in the
// stack). Input-requiring writes (env, config, publish) are their own pushed
// screens that capture input, mirroring the invoke / dynamo-query idiom.

// ---- result messages -------------------------------------------------------

type envUpdateDoneMsg struct {
	name string
	err  error
}

type configUpdateDoneMsg struct {
	name string
	err  error
}

type publishDoneMsg struct {
	name    string
	version string
	alias   string
	err     error
}

type deleteDoneMsg struct {
	name string
	err  error
}

const writeTimeout = 30 * time.Second

// ---- run commands (executed only after the user confirms) ------------------

func updateEnvCmd(client *awsx.LambdaClient, name string, env map[string]string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return envUpdateDoneMsg{name: name, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		return envUpdateDoneMsg{name: name, err: client.UpdateFunctionEnv(ctx, name, env)}
	}
}

func updateConfigCmd(client *awsx.LambdaClient, name string, memoryMB, timeoutSec int32) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return configUpdateDoneMsg{name: name, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		return configUpdateDoneMsg{name: name, err: client.UpdateFunctionConfig(ctx, name, memoryMB, timeoutSec)}
	}
}

func publishCmd(client *awsx.LambdaClient, name, description, alias string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return publishDoneMsg{name: name, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		ver, err := client.PublishVersion(ctx, name, description)
		if err != nil {
			return publishDoneMsg{name: name, err: err}
		}
		if alias != "" {
			if err := client.CreateOrUpdateAlias(ctx, name, alias, ver); err != nil {
				return publishDoneMsg{name: name, version: ver, alias: alias, err: err}
			}
		}
		return publishDoneMsg{name: name, version: ver, alias: alias}
	}
}

func deleteFunctionCmd(client *awsx.LambdaClient, name string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return deleteDoneMsg{name: name, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		return deleteDoneMsg{name: name, err: client.DeleteFunction(ctx, name)}
	}
}

// ---- input parsing (unit-tested) -------------------------------------------

// parseEnvLines parses a textarea body of `KEY=VALUE` lines into a map. Blank
// lines are skipped; a line without '=' or with an empty key is an error.
func parseEnvLines(body string) (map[string]string, error) {
	out := map[string]string{}
	for i, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		eq := strings.IndexByte(t, '=')
		if eq < 0 {
			return nil, fmt.Errorf("line %d: missing '=' (use KEY=VALUE)", i+1)
		}
		k := strings.TrimSpace(t[:eq])
		if k == "" {
			return nil, fmt.Errorf("line %d: empty key", i+1)
		}
		out[k] = strings.TrimSpace(t[eq+1:])
	}
	return out, nil
}

// parseMemTimeout validates a memory (MB) and timeout (s) string pair against
// Lambda's accepted ranges and returns them as int32.
func parseMemTimeout(memStr, timeoutStr string) (int32, int32, error) {
	mem, err := strconv.ParseInt(strings.TrimSpace(memStr), 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("memory must be a number")
	}
	if mem < 128 || mem > 10240 {
		return 0, 0, fmt.Errorf("memory must be 128–10240 MB")
	}
	to, err := strconv.ParseInt(strings.TrimSpace(timeoutStr), 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("timeout must be a number")
	}
	if to < 1 || to > 900 {
		return 0, 0, fmt.Errorf("timeout must be 1–900 s")
	}
	return int32(mem), int32(to), nil
}

// ---- env editor screen (#32) -----------------------------------------------

type envEditScreen struct {
	client        *awsx.LambdaClient
	name          string
	ta            textarea.Model
	width, height int
	status        string
	err           string
}

func newEnvEditScreen(client *awsx.LambdaClient, name string, env map[string]string) *envEditScreen {
	ta := textarea.New()
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}
	ta.SetValue(strings.TrimRight(b.String(), "\n"))
	ta.Focus()
	return &envEditScreen{client: client, name: name, ta: ta}
}

func (s *envEditScreen) Init() tea.Cmd { return textarea.Blink }

func (s *envEditScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case envUpdateDoneMsg:
		if msg.name != s.name {
			return s, nil
		}
		if msg.err != nil {
			s.err = msg.err.Error()
			return s, nil
		}
		s.status = "environment updated"
		return s, core.Pop()
	case tea.KeyMsg:
		if msg.String() == "ctrl+s" {
			env, err := parseEnvLines(s.ta.Value())
			if err != nil {
				s.err = err.Error()
				return s, nil
			}
			s.err = ""
			body := fmt.Sprintf("Replace environment on %s with %d variable(s)?", s.name, len(env))
			return s, core.ConfirmRequest("Update environment", body,
				"lambda.updateEnv", s.name, updateEnvCmd(s.client, s.name, env))
		}
	}
	var cmd tea.Cmd
	s.ta, cmd = s.ta.Update(msg)
	return s, cmd
}

func (s *envEditScreen) View() string {
	header := titleStyle.Render("Edit env · "+s.name) + "  " +
		faintSty.Render("(one KEY=VALUE per line · ctrl+s save · esc cancel)")
	return header + "\n\n" + s.ta.View() + "\n\n" + writeFooter(s.status, s.err)
}

func (s *envEditScreen) Title() string { return "edit env" }
func (s *envEditScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("ctrl+s", "save"), core.Hint("esc", "cancel")}
}
func (s *envEditScreen) SetSize(w, h int) {
	s.width, s.height = w, h
	bh := h - 4
	if bh < 3 {
		bh = 3
	}
	s.ta.SetWidth(w)
	s.ta.SetHeight(bh)
}
func (s *envEditScreen) CapturesInput() bool { return true }

// ---- memory/timeout editor screen (#33) ------------------------------------

type configEditScreen struct {
	client        *awsx.LambdaClient
	name          string
	mem           textinput.Model
	timeout       textinput.Model
	focus         int // 0 mem, 1 timeout
	width, height int
	status        string
	err           string
}

func newConfigEditScreen(client *awsx.LambdaClient, name string, curMem, curTimeout int32) *configEditScreen {
	mem := textinput.New()
	mem.Prompt = "  "
	mem.CharLimit = 6
	mem.SetValue(strconv.Itoa(int(curMem)))
	mem.Focus()

	to := textinput.New()
	to.Prompt = "  "
	to.CharLimit = 6
	to.SetValue(strconv.Itoa(int(curTimeout)))

	return &configEditScreen{client: client, name: name, mem: mem, timeout: to}
}

func (s *configEditScreen) Init() tea.Cmd { return textinput.Blink }

func (s *configEditScreen) setFocus(i int) {
	s.focus = i
	if i == 0 {
		s.mem.Focus()
		s.timeout.Blur()
	} else {
		s.mem.Blur()
		s.timeout.Focus()
	}
}

func (s *configEditScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case configUpdateDoneMsg:
		if msg.name != s.name {
			return s, nil
		}
		if msg.err != nil {
			s.err = msg.err.Error()
			return s, nil
		}
		s.status = "configuration updated"
		return s, core.Pop()
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down", "shift+tab", "up":
			s.setFocus((s.focus + 1) % 2)
			return s, nil
		case "ctrl+s":
			mem, to, err := parseMemTimeout(s.mem.Value(), s.timeout.Value())
			if err != nil {
				s.err = err.Error()
				return s, nil
			}
			s.err = ""
			body := fmt.Sprintf("Set %s to %d MB memory and %d s timeout?", s.name, mem, to)
			return s, core.ConfirmRequest("Update configuration", body,
				"lambda.updateConfig", s.name, updateConfigCmd(s.client, s.name, mem, to))
		}
	}
	var cmd tea.Cmd
	if s.focus == 0 {
		s.mem, cmd = s.mem.Update(msg)
	} else {
		s.timeout, cmd = s.timeout.Update(msg)
	}
	return s, cmd
}

func (s *configEditScreen) View() string {
	header := titleStyle.Render("Edit config · "+s.name) + "  " +
		faintSty.Render("(tab switch · ctrl+s save · esc cancel)")
	body := keyStyle.Render("Memory (MB)") + "\n" + s.mem.View() + "\n\n" +
		keyStyle.Render("Timeout (s)") + "\n" + s.timeout.View()
	return header + "\n\n" + body + "\n\n" + writeFooter(s.status, s.err)
}

func (s *configEditScreen) Title() string { return "edit config" }
func (s *configEditScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("tab", "switch"), core.Hint("ctrl+s", "save"), core.Hint("esc", "cancel")}
}
func (s *configEditScreen) SetSize(w, h int) {
	s.width, s.height = w, h
	s.mem.Width = w - 4
	s.timeout.Width = w - 4
}
func (s *configEditScreen) CapturesInput() bool { return true }

// ---- publish + alias screen (#34) ------------------------------------------

type publishScreen struct {
	client        *awsx.LambdaClient
	name          string
	desc          textinput.Model
	alias         textinput.Model
	focus         int // 0 desc, 1 alias
	width, height int
	status        string
	err           string
}

func newPublishScreen(client *awsx.LambdaClient, name string) *publishScreen {
	desc := textinput.New()
	desc.Prompt = "  "
	desc.Placeholder = "version description (optional)"
	desc.CharLimit = 256
	desc.Focus()

	alias := textinput.New()
	alias.Prompt = "  "
	alias.Placeholder = "alias to point at the new version (optional)"
	alias.CharLimit = 128

	return &publishScreen{client: client, name: name, desc: desc, alias: alias}
}

func (s *publishScreen) Init() tea.Cmd { return textinput.Blink }

func (s *publishScreen) setFocus(i int) {
	s.focus = i
	if i == 0 {
		s.desc.Focus()
		s.alias.Blur()
	} else {
		s.desc.Blur()
		s.alias.Focus()
	}
}

func (s *publishScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case publishDoneMsg:
		if msg.name != s.name {
			return s, nil
		}
		if msg.err != nil {
			s.err = msg.err.Error()
			return s, nil
		}
		if msg.alias != "" {
			s.status = fmt.Sprintf("published version %s (alias %s)", msg.version, msg.alias)
		} else {
			s.status = "published version " + msg.version
		}
		return s, core.Pop()
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down", "shift+tab", "up":
			s.setFocus((s.focus + 1) % 2)
			return s, nil
		case "ctrl+s":
			desc := strings.TrimSpace(s.desc.Value())
			alias := strings.TrimSpace(s.alias.Value())
			s.err = ""
			body := "Publish a new version of " + s.name + "?"
			if alias != "" {
				body = fmt.Sprintf("Publish a new version of %s and point alias %q at it?", s.name, alias)
			}
			return s, core.ConfirmRequest("Publish version", body,
				"lambda.publish", s.name, publishCmd(s.client, s.name, desc, alias))
		}
	}
	var cmd tea.Cmd
	if s.focus == 0 {
		s.desc, cmd = s.desc.Update(msg)
	} else {
		s.alias, cmd = s.alias.Update(msg)
	}
	return s, cmd
}

func (s *publishScreen) View() string {
	header := titleStyle.Render("Publish · "+s.name) + "  " +
		faintSty.Render("(tab switch · ctrl+s publish · esc cancel)")
	body := keyStyle.Render("Description") + "\n" + s.desc.View() + "\n\n" +
		keyStyle.Render("Alias") + "\n" + s.alias.View()
	return header + "\n\n" + body + "\n\n" + writeFooter(s.status, s.err)
}

func (s *publishScreen) Title() string { return "publish" }
func (s *publishScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("tab", "switch"), core.Hint("ctrl+s", "publish"), core.Hint("esc", "cancel")}
}
func (s *publishScreen) SetSize(w, h int) {
	s.width, s.height = w, h
	s.desc.Width = w - 4
	s.alias.Width = w - 4
}
func (s *publishScreen) CapturesInput() bool { return true }

// ---- shared rendering ------------------------------------------------------

var okStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)

func writeFooter(status, errMsg string) string {
	if errMsg != "" {
		return errStyle.Render("error: " + errMsg)
	}
	if status != "" {
		return okStyle.Render(status)
	}
	return ""
}
