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

// RootList is the ECS mode's root screen (the cluster list). enter drills into
// the cluster's services; 'd' opens the cluster describe; 't' opens the
// region's task-definition families.
type RootList interface {
	core.Screen
	SetClient(*awsx.EcsClient)
	Refresh() tea.Cmd
	IsFiltering() bool
	OpenServices(*awsx.Config) core.Screen
	OpenDescribe(*awsx.Config) core.Screen
	OpenTaskDefs(*awsx.Config) core.Screen
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
func (s *listScreen) View() string     { return s.m.View() }
func (s *listScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *listScreen) Title() string    { return "clusters" }
func (s *listScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("enter", "services"), core.Hint("d", "describe"), core.Hint("t", "task-defs")}
}
func (s *listScreen) SetClient(c *awsx.EcsClient) { s.m.SetClient(c) }
func (s *listScreen) Refresh() tea.Cmd            { return s.m.Refresh() }
func (s *listScreen) IsFiltering() bool           { return s.m.IsFiltering() }
func (s *listScreen) CapturesInput() bool         { return s.m.IsFiltering() }
func (s *listScreen) WantsEsc() bool              { return s.m.IsFiltering() }

func (s *listScreen) OpenServices(cfg *awsx.Config) core.Screen {
	if name := s.m.Selected(); name != "" && cfg != nil {
		return &serviceListScreen{m: NewServiceList(awsx.NewEcsClient(cfg), name)}
	}
	return nil
}

func (s *listScreen) OpenDescribe(cfg *awsx.Config) core.Screen {
	if name := s.m.Selected(); name != "" && cfg != nil {
		return &clusterDescribeScreen{m: NewClusterDescribe(awsx.NewEcsClient(cfg), name)}
	}
	return nil
}

func (s *listScreen) OpenTaskDefs(cfg *awsx.Config) core.Screen {
	if cfg == nil {
		return nil
	}
	return &taskDefListScreen{m: NewTaskDefList(awsx.NewEcsClient(cfg))}
}

// TaskDefList is the region's task-definition families list. enter opens the
// task-def describe.
type TaskDefList interface {
	core.Screen
	IsFiltering() bool
	OpenDescribe(*awsx.Config) core.Screen
}

type taskDefListScreen struct{ m TaskDefListModel }

func (s *taskDefListScreen) Init() tea.Cmd { return s.m.Init() }
func (s *taskDefListScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *taskDefListScreen) View() string     { return s.m.View() }
func (s *taskDefListScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *taskDefListScreen) Title() string    { return "task-defs" }
func (s *taskDefListScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("enter", "describe")}
}
func (s *taskDefListScreen) IsFiltering() bool   { return s.m.IsFiltering() }
func (s *taskDefListScreen) CapturesInput() bool { return s.m.IsFiltering() }
func (s *taskDefListScreen) WantsEsc() bool      { return s.m.IsFiltering() }

func (s *taskDefListScreen) OpenDescribe(cfg *awsx.Config) core.Screen {
	if family := s.m.Selected(); family != "" && cfg != nil {
		return &taskDefDescribeScreen{m: NewTaskDefDescribe(awsx.NewEcsClient(cfg), family)}
	}
	return nil
}

// TaskDefDescriber is a task-def describe that can open its revision history.
type TaskDefDescriber interface {
	core.Screen
	OpenRevisions(*awsx.Config) core.Screen
}

type taskDefDescribeScreen struct{ m TaskDefDescribeModel }

func (s *taskDefDescribeScreen) Init() tea.Cmd { return s.m.Init() }
func (s *taskDefDescribeScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *taskDefDescribeScreen) View() string     { return s.m.View() }
func (s *taskDefDescribeScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *taskDefDescribeScreen) Title() string    { return "describe" }
func (s *taskDefDescribeScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("v", "revisions"), core.Hint("J", "json"), core.Hint("r", "refresh")}
}

func (s *taskDefDescribeScreen) OpenRevisions(cfg *awsx.Config) core.Screen {
	if cfg == nil {
		return nil
	}
	return &taskDefRevisionsScreen{m: NewTaskDefRevisions(awsx.NewEcsClient(cfg), s.m.Family())}
}

// RevisionList is a task-def family's revision history. enter opens the
// describe for the selected revision.
type RevisionList interface {
	core.Screen
	IsFiltering() bool
	OpenRevision(*awsx.Config) core.Screen
}

type taskDefRevisionsScreen struct{ m TaskDefRevisionsModel }

func (s *taskDefRevisionsScreen) Init() tea.Cmd { return s.m.Init() }
func (s *taskDefRevisionsScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *taskDefRevisionsScreen) View() string     { return s.m.View() }
func (s *taskDefRevisionsScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *taskDefRevisionsScreen) Title() string    { return "revisions" }
func (s *taskDefRevisionsScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("enter", "describe")}
}
func (s *taskDefRevisionsScreen) IsFiltering() bool   { return s.m.IsFiltering() }
func (s *taskDefRevisionsScreen) CapturesInput() bool { return s.m.IsFiltering() }
func (s *taskDefRevisionsScreen) WantsEsc() bool      { return s.m.IsFiltering() }

func (s *taskDefRevisionsScreen) OpenRevision(cfg *awsx.Config) core.Screen {
	if rev := s.m.Selected(); rev != "" && cfg != nil {
		ref := s.m.Family() + ":" + rev
		return &taskDefDescribeScreen{m: NewTaskDefDescribe(awsx.NewEcsClient(cfg), ref)}
	}
	return nil
}

type clusterDescribeScreen struct{ m ClusterDescribeModel }

func (s *clusterDescribeScreen) Init() tea.Cmd { return s.m.Init() }
func (s *clusterDescribeScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *clusterDescribeScreen) View() string     { return s.m.View() }
func (s *clusterDescribeScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *clusterDescribeScreen) Title() string    { return "describe" }
func (s *clusterDescribeScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("r", "refresh")}
}

// ServiceList is a per-cluster service list. enter drills into the service's
// tasks; 'd' opens the service describe.
type ServiceList interface {
	core.Screen
	IsFiltering() bool
	OpenTasks(*awsx.Config) core.Screen
	OpenDescribe(*awsx.Config) core.Screen
}

type serviceListScreen struct{ m ServiceListModel }

func (s *serviceListScreen) Init() tea.Cmd { return s.m.Init() }
func (s *serviceListScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *serviceListScreen) View() string     { return s.m.View() }
func (s *serviceListScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *serviceListScreen) Title() string    { return "services" }
func (s *serviceListScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("enter", "tasks"), core.Hint("d", "describe")}
}
func (s *serviceListScreen) IsFiltering() bool   { return s.m.IsFiltering() }
func (s *serviceListScreen) CapturesInput() bool { return s.m.IsFiltering() }
func (s *serviceListScreen) WantsEsc() bool      { return s.m.IsFiltering() }

func (s *serviceListScreen) OpenDescribe(cfg *awsx.Config) core.Screen {
	if name := s.m.Selected(); name != "" && cfg != nil {
		return &serviceDescribeScreen{m: NewServiceDescribe(awsx.NewEcsClient(cfg), s.m.Cluster(), name)}
	}
	return nil
}

func (s *serviceListScreen) OpenTasks(cfg *awsx.Config) core.Screen {
	if name := s.m.Selected(); name != "" && cfg != nil {
		return &taskListScreen{m: NewTaskList(awsx.NewEcsClient(cfg), s.m.Cluster(), name)}
	}
	return nil
}

// TaskList is a per-service task list. enter drills into a task's containers.
type TaskList interface {
	core.Screen
	IsFiltering() bool
	OpenContainers(*awsx.Config) core.Screen
}

type taskListScreen struct{ m TaskListModel }

func (s *taskListScreen) Init() tea.Cmd { return s.m.Init() }
func (s *taskListScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *taskListScreen) View() string     { return s.m.View() }
func (s *taskListScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *taskListScreen) Title() string    { return "tasks" }
func (s *taskListScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("enter", "containers")}
}
func (s *taskListScreen) IsFiltering() bool   { return s.m.IsFiltering() }
func (s *taskListScreen) CapturesInput() bool { return s.m.IsFiltering() }
func (s *taskListScreen) WantsEsc() bool      { return s.m.IsFiltering() }

func (s *taskListScreen) OpenContainers(cfg *awsx.Config) core.Screen {
	if id := s.m.Selected(); id != "" && cfg != nil {
		return &containerListScreen{m: NewContainerList(awsx.NewEcsClient(cfg), s.m.Cluster(), id)}
	}
	return nil
}

// ContainerList is a per-task container list. enter drills into the selected
// container's logs; 'x' execs into it.
type ContainerList interface {
	core.Screen
	IsFiltering() bool
	OpenLogs(*awsx.Config) core.Screen
	// ExecTarget returns the cluster/task/container for the selected row.
	// ok is false when nothing is selected.
	ExecTarget() (cluster, task, container string, ok bool)
}

type containerListScreen struct{ m ContainerListModel }

func (s *containerListScreen) Init() tea.Cmd { return s.m.Init() }
func (s *containerListScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *containerListScreen) View() string     { return s.m.View() }
func (s *containerListScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *containerListScreen) Title() string    { return "containers" }
func (s *containerListScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("enter", "logs"), core.Hint("x", "exec")}
}
func (s *containerListScreen) IsFiltering() bool   { return s.m.IsFiltering() }
func (s *containerListScreen) CapturesInput() bool { return s.m.IsFiltering() }
func (s *containerListScreen) WantsEsc() bool      { return s.m.IsFiltering() }

func (s *containerListScreen) OpenLogs(cfg *awsx.Config) core.Screen {
	if name := s.m.Selected(); name != "" && cfg != nil {
		return &containerLogsScreen{
			m: NewContainerLogs(awsx.NewEcsClient(cfg), awsx.NewLogsClient(cfg), s.m.cluster, s.m.task, name),
		}
	}
	return nil
}

func (s *containerListScreen) ExecTarget() (string, string, string, bool) {
	name := s.m.Selected()
	if name == "" {
		return "", "", "", false
	}
	return s.m.cluster, s.m.task, name, true
}

type containerLogsScreen struct{ m ContainerLogsModel }

func (s *containerLogsScreen) Init() tea.Cmd { return s.m.Init() }
func (s *containerLogsScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *containerLogsScreen) View() string     { return s.m.View() }
func (s *containerLogsScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *containerLogsScreen) Title() string    { return "logs" }
func (s *containerLogsScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("f", "follow"), core.Hint("/", "filter"), core.Hint("c", "clear")}
}
func (s *containerLogsScreen) CapturesInput() bool { return true }                // tailing view owns keys
func (s *containerLogsScreen) WantsEsc() bool      { return s.m.FilterFocused() } // esc clears filter, else pops

// ServiceDescriber is a service describe screen that can open its events panel.
type ServiceDescriber interface {
	core.Screen
	OpenEvents(*awsx.Config) core.Screen
}

type serviceDescribeScreen struct{ m ServiceDescribeModel }

func (s *serviceDescribeScreen) Init() tea.Cmd { return s.m.Init() }
func (s *serviceDescribeScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *serviceDescribeScreen) View() string     { return s.m.View() }
func (s *serviceDescribeScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *serviceDescribeScreen) Title() string    { return "describe" }
func (s *serviceDescribeScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("e", "events"), core.Hint("r", "refresh")}
}

func (s *serviceDescribeScreen) OpenEvents(cfg *awsx.Config) core.Screen {
	if cfg == nil || s.m.Name() == "" {
		return nil
	}
	return &serviceEventsScreen{m: NewServiceEvents(awsx.NewEcsClient(cfg), s.m.Cluster(), s.m.Name())}
}

type serviceEventsScreen struct{ m ServiceEventsModel }

func (s *serviceEventsScreen) Init() tea.Cmd { return s.m.Init() }
func (s *serviceEventsScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	nm, cmd := s.m.Update(msg)
	s.m = nm
	return s, cmd
}
func (s *serviceEventsScreen) View() string     { return s.m.View() }
func (s *serviceEventsScreen) SetSize(w, h int) { s.m.SetSize(w, h) }
func (s *serviceEventsScreen) Title() string    { return "events" }
func (s *serviceEventsScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("r", "refresh")}
}
