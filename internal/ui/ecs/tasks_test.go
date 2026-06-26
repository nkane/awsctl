package ecs

import (
	"strings"
	"testing"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func TestTaskListRenders(t *testing.T) {
	m := NewTaskList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")
	m.SetSize(100, 30)

	m, _ = m.Update(tasksLoadedMsg{cluster: "demo-cluster", service: "api", tasks: []awsx.TaskSummary{
		{ID: "abc123", LastStatus: "RUNNING", Desired: "RUNNING", Health: "HEALTHY", LaunchType: "FARGATE", TaskDef: "api:7", StartedAt: "2026-06-26 09:00:00"},
		{ID: "def456", LastStatus: "PENDING", Desired: "RUNNING", LaunchType: "FARGATE", TaskDef: "api:7"},
	}})

	v := m.View()
	for _, want := range []string{"abc123", "def456", "RUNNING", "HEALTHY", "api:7", "not started"} {
		if !strings.Contains(v, want) {
			t.Fatalf("task view missing %q; got:\n%s", want, v)
		}
	}
	if got := m.Selected(); got != "abc123" {
		t.Fatalf("Selected() = %q, want abc123", got)
	}
}

func TestTaskListIgnoresStaleLoad(t *testing.T) {
	m := NewTaskList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")
	m.SetSize(100, 30)
	// Wrong service -> must not populate.
	m, _ = m.Update(tasksLoadedMsg{cluster: "demo-cluster", service: "worker", tasks: []awsx.TaskSummary{{ID: "ghost"}}})
	if got := m.Selected(); got != "" {
		t.Fatalf("stale load leaked; Selected() = %q, want empty", got)
	}
}

// TestServiceDrillKeys verifies enter -> tasks and 'd' -> describe from the
// service list (#45 -> #48 / #46).
func TestServiceDrillKeys(t *testing.T) {
	newSL := func() *serviceListScreen {
		sl := &serviceListScreen{m: NewServiceList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster")}
		sl.m, _ = sl.m.Update(servicesLoadedMsg{cluster: "demo-cluster", services: []awsx.ServiceSummary{{Name: "api"}}})
		return sl
	}

	if scr := newSL().OpenTasks(&awsx.Config{}); scr == nil || scr.Title() != "tasks" {
		t.Fatalf("OpenTasks should yield a 'tasks' screen, got %v", scr)
	}
	if scr := newSL().OpenDescribe(&awsx.Config{}); scr == nil || scr.Title() != "describe" {
		t.Fatalf("OpenDescribe should yield a 'describe' screen, got %v", scr)
	}

	empty := &serviceListScreen{m: NewServiceList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster")}
	if got := empty.OpenTasks(&awsx.Config{}); got != nil {
		t.Fatalf("OpenTasks with no selection should be nil, got %v", got)
	}
}
