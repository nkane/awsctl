// Package lambda renders the Lambda function list and detail screens.
package lambda

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
)

// fnItem adapts a FunctionSummary to bubbles/list.Item.
type fnItem struct{ s awsx.FunctionSummary }

func (i fnItem) FilterValue() string { return i.s.Name }
func (i fnItem) Title() string       { return i.s.Name }
func (i fnItem) Description() string {
	rt := i.s.Runtime
	if rt == "" {
		rt = "-"
	}
	return fmt.Sprintf("%s · %dMB · %ds · %s", rt, i.s.Memory, i.s.Timeout, i.s.Handler)
}

// loadedMsg carries fetched function summaries, or an error.
type loadedMsg struct {
	fns []awsx.FunctionSummary
	err error
}

// LoadCmd kicks off a background ListFunctions call.
func LoadCmd(client *awsx.LambdaClient) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return loadedMsg{err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30_000_000_000) // 30s
		defer cancel()
		fns, err := client.ListFunctions(ctx)
		return loadedMsg{fns: fns, err: err}
	}
}

// ListModel is the Lambda list screen.
type ListModel struct {
	client  *awsx.LambdaClient
	list    list.Model
	spinner spinner.Model
	loading bool
	err     string
	width   int
	height  int
	loaded  bool
}

// NewList constructs an empty list model. Call SetClient + Refresh once an
// AWS config has been resolved by the root app.
func NewList() ListModel {
	d := list.NewDefaultDelegate()
	l := list.New(nil, d, 0, 0)
	l.Title = "Lambda Functions"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return ListModel{list: l, spinner: sp}
}

// SetClient swaps in the AWS client (called when AWS config loads / changes).
func (m *ListModel) SetClient(c *awsx.LambdaClient) { m.client = c }

// Refresh triggers a fresh ListFunctions call.
func (m *ListModel) Refresh() tea.Cmd {
	m.loading = true
	m.err = ""
	return tea.Batch(m.spinner.Tick, LoadCmd(m.client))
}

// SetSize sets the visible dimensions for the inner list.
func (m *ListModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.list.SetSize(w, h)
}

// Selected returns the currently highlighted function summary, or zero value.
func (m ListModel) Selected() awsx.FunctionSummary {
	if it, ok := m.list.SelectedItem().(fnItem); ok {
		return it.s
	}
	return awsx.FunctionSummary{}
}

// Loaded reports whether the first fetch has completed.
func (m ListModel) Loaded() bool { return m.loaded }

// IsFiltering reports whether the inner list is currently in filter-input
// mode. Used by the root model to suppress global keybindings (like 'q').
func (m ListModel) IsFiltering() bool {
	return m.list.SettingFilter() || m.list.FilterState() == list.Filtering
}

// Update implements tea.Model conventions but returns the concrete type so the
// root model can keep its zero-allocation message routing.
func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.loading = false
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.fns))
		for _, fn := range msg.fns {
			items = append(items, fnItem{s: fn})
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
		// 'r' refreshes regardless of filter focus state.
		if msg.String() == "r" && !m.list.SettingFilter() {
			return m, m.Refresh()
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the list (or a status line while loading / on error).
func (m ListModel) View() string {
	if m.client == nil {
		return faint("waiting for AWS config…")
	}
	if m.loading && !m.loaded {
		return fmt.Sprintf("%s loading lambdas…", m.spinner.View())
	}
	if m.err != "" {
		return errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	if m.loaded && len(m.list.Items()) == 0 {
		return faint("no Lambda functions found in this region.\npress r to refresh")
	}
	return m.list.View()
}

var (
	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	faintSty = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func faint(s string) string { return faintSty.Render(s) }
