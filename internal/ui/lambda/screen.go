package lambda

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkane/awsctl/internal/ui/core"
)

// This file adapts the Lambda models to core.Screen so they can live on the
// navigation stack. Adapters are pointer types: the wrapped model is a value
// (Bubble Tea idiom) but SetSize has a pointer receiver and Update returns a new
// model value, so the adapter stores the model and reassigns it in place.
//
// KeyHints return nil for now; the menu/help overlay (#19) populates them later.

// listScreen adapts ListModel.
type listScreen struct{ m ListModel }

// ListScreen wraps a ListModel as a core.Screen.
func ListScreen(m ListModel) core.Screen { return &listScreen{m: m} }

func (s *listScreen) Init() tea.Cmd { return nil } // first fetch driven by App on config load
func (s *listScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *listScreen) View() string            { return s.m.View() }
func (s *listScreen) SetSize(w, h int)        { s.m.SetSize(w, h) }
func (s *listScreen) Title() string           { return "functions" }
func (s *listScreen) KeyHints() []key.Binding { return nil }

// Model exposes the wrapped model so the App can seed clients / read selection.
func (s *listScreen) Model() *ListModel { return &s.m }

// detailScreen adapts DetailModel.
type detailScreen struct{ m DetailModel }

// DetailScreen wraps a DetailModel as a core.Screen.
func DetailScreen(m DetailModel) core.Screen { return &detailScreen{m: m} }

func (s *detailScreen) Init() tea.Cmd { return s.m.Init() }
func (s *detailScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *detailScreen) View() string            { return s.m.View() }
func (s *detailScreen) SetSize(w, h int)        { s.m.SetSize(w, h) }
func (s *detailScreen) Title() string           { return "detail" }
func (s *detailScreen) KeyHints() []key.Binding { return nil }
func (s *detailScreen) Model() *DetailModel     { return &s.m }

// invokeScreen adapts InvokeModel.
type invokeScreen struct{ m InvokeModel }

// InvokeScreen wraps an InvokeModel as a core.Screen.
func InvokeScreen(m InvokeModel) core.Screen { return &invokeScreen{m: m} }

func (s *invokeScreen) Init() tea.Cmd { return nil }
func (s *invokeScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *invokeScreen) View() string            { return s.m.View() }
func (s *invokeScreen) SetSize(w, h int)        { s.m.SetSize(w, h) }
func (s *invokeScreen) Title() string           { return "invoke" }
func (s *invokeScreen) KeyHints() []key.Binding { return nil }

// logsScreen adapts LogsModel.
type logsScreen struct{ m LogsModel }

// LogsScreen wraps a LogsModel as a core.Screen.
func LogsScreen(m LogsModel) core.Screen { return &logsScreen{m: m} }

func (s *logsScreen) Init() tea.Cmd { return s.m.Init() }
func (s *logsScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *logsScreen) View() string            { return s.m.View() }
func (s *logsScreen) SetSize(w, h int)        { s.m.SetSize(w, h) }
func (s *logsScreen) Title() string           { return "logs" }
func (s *logsScreen) KeyHints() []key.Binding { return nil }
