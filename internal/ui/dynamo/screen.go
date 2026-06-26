package dynamo

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkane/awsctl/internal/ui/core"
)

// Adapters wrapping the Dynamo models as core.Screen. See lambda/screen.go for
// the pointer-adapter rationale. KeyHints return nil until the menu/help
// overlay (#19) is wired.

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
func (s *listScreen) Title() string           { return "tables" }
func (s *listScreen) KeyHints() []key.Binding { return nil }
func (s *listScreen) Model() *ListModel       { return &s.m }

// describeScreen adapts DescribeModel.
type describeScreen struct{ m DescribeModel }

// DescribeScreen wraps a DescribeModel as a core.Screen.
func DescribeScreen(m DescribeModel) core.Screen { return &describeScreen{m: m} }

func (s *describeScreen) Init() tea.Cmd { return s.m.Init() }
func (s *describeScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *describeScreen) View() string            { return s.m.View() }
func (s *describeScreen) SetSize(w, h int)        { s.m.SetSize(w, h) }
func (s *describeScreen) Title() string           { return "describe" }
func (s *describeScreen) KeyHints() []key.Binding { return nil }
func (s *describeScreen) Model() *DescribeModel   { return &s.m }

// scanScreen adapts ScanModel.
type scanScreen struct{ m ScanModel }

// ScanScreen wraps a ScanModel as a core.Screen.
func ScanScreen(m ScanModel) core.Screen { return &scanScreen{m: m} }

func (s *scanScreen) Init() tea.Cmd { return s.m.Init() }
func (s *scanScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *scanScreen) View() string            { return s.m.View() }
func (s *scanScreen) SetSize(w, h int)        { s.m.SetSize(w, h) }
func (s *scanScreen) Title() string           { return "scan" }
func (s *scanScreen) KeyHints() []key.Binding { return nil }
func (s *scanScreen) Model() *ScanModel       { return &s.m }

// queryScreen adapts QueryModel.
type queryScreen struct{ m QueryModel }

// QueryScreen wraps a QueryModel as a core.Screen.
func QueryScreen(m QueryModel) core.Screen { return &queryScreen{m: m} }

func (s *queryScreen) Init() tea.Cmd { return s.m.Init() }
func (s *queryScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *queryScreen) View() string            { return s.m.View() }
func (s *queryScreen) SetSize(w, h int)        { s.m.SetSize(w, h) }
func (s *queryScreen) Title() string           { return "query" }
func (s *queryScreen) KeyHints() []key.Binding { return nil }
func (s *queryScreen) Model() *QueryModel      { return &s.m }

// itemScreen adapts ItemModel.
type itemScreen struct{ m ItemModel }

// ItemScreen wraps an ItemModel as a core.Screen.
func ItemScreen(m ItemModel) core.Screen { return &itemScreen{m: m} }

func (s *itemScreen) Init() tea.Cmd { return s.m.Init() }
func (s *itemScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *itemScreen) View() string            { return s.m.View() }
func (s *itemScreen) SetSize(w, h int)        { s.m.SetSize(w, h) }
func (s *itemScreen) Title() string           { return "item" }
func (s *itemScreen) KeyHints() []key.Binding { return nil }
