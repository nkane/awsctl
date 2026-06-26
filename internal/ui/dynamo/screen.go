package dynamo

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	awsx "github.com/nkane/awsctl/internal/aws"
	"github.com/nkane/awsctl/internal/ui/core"
)

// Adapters wrapping the Dynamo models as core.Screen, plus the Dynamo drill-in
// graph. See lambda/screen.go for the pointer-adapter and OpenX(cfg) builder
// rationale. KeyHints return nil until the menu/help overlay (#19) is wired.

// RootList is the Dynamo mode's root screen (the table list).
type RootList interface {
	core.Screen
	SetClient(*awsx.DynamoClient)
	Refresh() tea.Cmd
	IsFiltering() bool
	OpenDescribe(*awsx.Config) core.Screen
}

// Describer is a table-describe screen that can drill into scan / query.
type Describer interface {
	core.Screen
	OpenScan(*awsx.Config) core.Screen
	OpenQuery(*awsx.Config) core.Screen
}

// Scanner is a scan-results screen that can open the selected item.
type Scanner interface {
	core.Screen
	OpenItem(*awsx.Config) core.Screen
}

// Querier is a query-results screen that can open the selected result.
type Querier interface {
	core.Screen
	OpenResult(*awsx.Config) core.Screen
	InputFocused() bool
}

// ---- list ----

type listScreen struct{ m ListModel }

// NewListScreen builds the Dynamo root table-list screen.
func NewListScreen() RootList { return &listScreen{m: NewList()} }

func (s *listScreen) Init() tea.Cmd { return nil } // first fetch driven by App on config load
func (s *listScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *listScreen) View() string                   { return s.m.View() }
func (s *listScreen) SetSize(w, h int)               { s.m.SetSize(w, h) }
func (s *listScreen) Title() string                  { return "tables" }
func (s *listScreen) KeyHints() []key.Binding        { return nil }
func (s *listScreen) SetClient(c *awsx.DynamoClient) { s.m.SetClient(c) }
func (s *listScreen) Refresh() tea.Cmd               { return s.m.Refresh() }
func (s *listScreen) IsFiltering() bool              { return s.m.IsFiltering() }
func (s *listScreen) CapturesInput() bool            { return s.m.IsFiltering() }
func (s *listScreen) WantsEsc() bool                 { return s.m.IsFiltering() }

func (s *listScreen) OpenDescribe(cfg *awsx.Config) core.Screen {
	if name := s.m.Selected(); name != "" && cfg != nil {
		return &describeScreen{m: NewDescribe(awsx.NewDynamoClient(cfg), name)}
	}
	return nil
}

// ---- describe ----

type describeScreen struct{ m DescribeModel }

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

func (s *describeScreen) OpenScan(cfg *awsx.Config) core.Screen {
	if s.m.Name() != "" && cfg != nil {
		return &scanScreen{m: NewScan(awsx.NewDynamoClient(cfg), s.m.Name(), s.m.Keys())}
	}
	return nil
}
func (s *describeScreen) OpenQuery(cfg *awsx.Config) core.Screen {
	if s.m.Name() != "" && cfg != nil {
		return &queryScreen{m: NewQuery(awsx.NewDynamoClient(cfg), s.m.Name())}
	}
	return nil
}

// ---- scan ----

type scanScreen struct{ m ScanModel }

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

func (s *scanScreen) OpenItem(cfg *awsx.Config) core.Screen {
	it := s.m.Selected()
	if it == nil || cfg == nil {
		return nil
	}
	return &itemScreen{m: NewItem(awsx.NewDynamoClient(cfg), s.m.Name(), it, s.m.SelectedKey())}
}

// ---- query ----

type queryScreen struct{ m QueryModel }

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
func (s *queryScreen) InputFocused() bool      { return s.m.InputFocused() }
func (s *queryScreen) CapturesInput() bool     { return s.m.InputFocused() } // key-condition editor

func (s *queryScreen) OpenResult(cfg *awsx.Config) core.Screen {
	it := s.m.Selected()
	if it == nil || cfg == nil {
		return nil
	}
	return &itemScreen{m: NewItem(awsx.NewDynamoClient(cfg), s.m.Name(), it, s.m.SelectedKey())}
}

// ---- item ----

type itemScreen struct{ m ItemModel }

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
func (s *itemScreen) CapturesInput() bool     { return true } // scrollable detail owns keys
