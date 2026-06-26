package ecs

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	awsx "github.com/nkane/awsctl/internal/aws"
	"github.com/nkane/awsctl/internal/ui/core"
)

// Adapter wrapping the ECS cluster list as a core.Screen. See lambda/screen.go
// for the pointer-adapter rationale. Drill-ins (cluster -> services -> tasks)
// land in follow-up tickets (#43+); for now this is the mode's root list.

// RootList is the ECS mode's root screen (the cluster list).
type RootList interface {
	core.Screen
	SetClient(*awsx.EcsClient)
	Refresh() tea.Cmd
	IsFiltering() bool
}

type listScreen struct{ m ListModel }

// NewListScreen builds the ECS root cluster-list screen.
func NewListScreen() RootList { return &listScreen{m: NewList()} }

func (s *listScreen) Init() tea.Cmd { return nil } // first fetch driven by App on config load
func (s *listScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *listScreen) View() string                { return s.m.View() }
func (s *listScreen) SetSize(w, h int)            { s.m.SetSize(w, h) }
func (s *listScreen) Title() string               { return "clusters" }
func (s *listScreen) KeyHints() []key.Binding     { return nil }
func (s *listScreen) SetClient(c *awsx.EcsClient) { s.m.SetClient(c) }
func (s *listScreen) Refresh() tea.Cmd            { return s.m.Refresh() }
func (s *listScreen) IsFiltering() bool           { return s.m.IsFiltering() }
func (s *listScreen) CapturesInput() bool         { return s.m.IsFiltering() }
func (s *listScreen) WantsEsc() bool              { return s.m.IsFiltering() }
