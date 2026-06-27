package lambda

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	awsx "github.com/nkane/awsctl/internal/aws"
	"github.com/nkane/awsctl/internal/ui/core"
)

// This file adapts the Lambda models to core.Screen and owns the Lambda
// drill-in graph. Adapters are pointer types: the wrapped model is a value
// (Bubble Tea idiom) but SetSize has a pointer receiver and Update returns a new
// model value, so the adapter stores the model and reassigns it in place.
//
// Drill-ins are exposed as OpenX(cfg) builders. Selection state stays inside the
// package; the App only supplies the AWS config and pushes the returned screen.
// A builder returns nil when there is nothing selected, which the App treats as
// a no-op. KeyHints return nil until the menu/help overlay (#19) lands.

// RootList is the Lambda mode's root screen (the function list).
type RootList interface {
	core.Screen
	SetClient(*awsx.LambdaClient)
	Refresh() tea.Cmd
	IsFiltering() bool
	OpenDetail(*awsx.Config) core.Screen
	OpenInvoke(*awsx.Config) core.Screen
	OpenLogs(*awsx.Config) core.Screen
}

// Detailer is a detail screen that can drill into invoke / logs.
type Detailer interface {
	core.Screen
	OpenInvoke(*awsx.Config) core.Screen
	OpenLogs(*awsx.Config) core.Screen
}

// ---- list ----

type listScreen struct{ m ListModel }

// NewListScreen builds the Lambda root list screen.
func NewListScreen() RootList { return &listScreen{m: NewList()} }

func (s *listScreen) Init() tea.Cmd { return nil } // first fetch driven by App on config load
func (s *listScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *listScreen) View() string     { return s.m.View() }
func (s *listScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *listScreen) Title() string    { return "functions" }
func (s *listScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("enter", "detail"), core.Hint("i", "invoke"), core.Hint("l", "logs")}
}
func (s *listScreen) SetClient(c *awsx.LambdaClient) { s.m.SetClient(c) }
func (s *listScreen) Refresh() tea.Cmd               { return s.m.Refresh() }
func (s *listScreen) IsFiltering() bool              { return s.m.IsFiltering() }
func (s *listScreen) CapturesInput() bool            { return s.m.IsFiltering() }
func (s *listScreen) WantsEsc() bool                 { return s.m.IsFiltering() }

func (s *listScreen) OpenDetail(cfg *awsx.Config) core.Screen {
	if sel := s.m.Selected(); sel.Name != "" && cfg != nil {
		return &detailScreen{m: NewDetail(awsx.NewLambdaClient(cfg), sel.Name)}
	}
	return nil
}
func (s *listScreen) OpenInvoke(cfg *awsx.Config) core.Screen {
	if sel := s.m.Selected(); sel.Name != "" && cfg != nil {
		return &invokeScreen{m: NewInvoke(awsx.NewLambdaClient(cfg), sel.Name)}
	}
	return nil
}
func (s *listScreen) OpenLogs(cfg *awsx.Config) core.Screen {
	if sel := s.m.Selected(); sel.Name != "" && cfg != nil {
		return &logsScreen{m: NewLogs(awsx.NewLogsClient(cfg), sel.Name)}
	}
	return nil
}

// ---- detail ----

type detailScreen struct{ m DetailModel }

func (s *detailScreen) Init() tea.Cmd { return s.m.Init() }
func (s *detailScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *detailScreen) View() string     { return s.m.View() }
func (s *detailScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *detailScreen) Title() string    { return "detail" }
func (s *detailScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("i", "invoke"), core.Hint("l", "logs")}
}

func (s *detailScreen) OpenInvoke(cfg *awsx.Config) core.Screen {
	if s.m.Name() != "" && cfg != nil {
		return &invokeScreen{m: NewInvoke(awsx.NewLambdaClient(cfg), s.m.Name())}
	}
	return nil
}
func (s *detailScreen) OpenLogs(cfg *awsx.Config) core.Screen {
	if s.m.Name() != "" && cfg != nil {
		return &logsScreen{m: NewLogs(awsx.NewLogsClient(cfg), s.m.Name())}
	}
	return nil
}

// ---- invoke ----

type invokeScreen struct{ m InvokeModel }

func (s *invokeScreen) Init() tea.Cmd { return nil }
func (s *invokeScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *invokeScreen) View() string     { return s.m.View() }
func (s *invokeScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *invokeScreen) Title() string    { return "invoke" }
func (s *invokeScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("ctrl+r", "run"), core.Hint("esc", "back")}
}
func (s *invokeScreen) CapturesInput() bool { return true } // payload editor

// ---- logs ----

type logsScreen struct{ m LogsModel }

func (s *logsScreen) Init() tea.Cmd { return s.m.Init() }
func (s *logsScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *logsScreen) View() string     { return s.m.View() }
func (s *logsScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *logsScreen) Title() string    { return "logs" }
func (s *logsScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("f", "follow"), core.Hint("/", "filter"), core.Hint("c", "clear")}
}
func (s *logsScreen) CapturesInput() bool { return true }                // tailing view owns keys
func (s *logsScreen) WantsEsc() bool      { return s.m.FilterFocused() } // esc clears filter, else pops
