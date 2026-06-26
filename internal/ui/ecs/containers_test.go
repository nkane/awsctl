package ecs

import (
	"strings"
	"testing"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func TestContainerListRenders(t *testing.T) {
	m := NewContainerList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "abc123")
	m.SetSize(100, 30)

	m, _ = m.Update(containersLoadedMsg{task: "abc123", containers: []awsx.ContainerSummary{
		{Name: "web", Image: "nginx:1.27", LastStatus: "RUNNING", Health: "HEALTHY", ExitCode: ""},
		{Name: "sidecar", Image: "envoy:1.31", LastStatus: "STOPPED", ExitCode: "1"},
	}})

	v := m.View()
	for _, want := range []string{"web", "sidecar", "nginx:1.27", "RUNNING", "HEALTHY", "exit 1"} {
		if !strings.Contains(v, want) {
			t.Fatalf("container view missing %q; got:\n%s", want, v)
		}
	}
	if got := m.Selected(); got != "web" {
		t.Fatalf("Selected() = %q, want web", got)
	}
}

func TestContainerListIgnoresStaleLoad(t *testing.T) {
	m := NewContainerList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "abc123")
	m.SetSize(100, 30)
	m, _ = m.Update(containersLoadedMsg{task: "other-task", containers: []awsx.ContainerSummary{{Name: "ghost"}}})
	if got := m.Selected(); got != "" {
		t.Fatalf("stale load leaked; Selected() = %q, want empty", got)
	}
}

// TestTaskDrillOpensContainers verifies enter on a selected task builds the
// container list (#48 -> #49 drill).
func TestTaskDrillOpensContainers(t *testing.T) {
	tl := &taskListScreen{m: NewTaskList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")}
	tl.m, _ = tl.m.Update(tasksLoadedMsg{cluster: "demo-cluster", service: "api", tasks: []awsx.TaskSummary{{ID: "abc123", LastStatus: "RUNNING"}}})

	scr := tl.OpenContainers(&awsx.Config{})
	if scr == nil || scr.Title() != "containers" {
		t.Fatalf("OpenContainers should yield a 'containers' screen, got %v", scr)
	}

	empty := &taskListScreen{m: NewTaskList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")}
	if got := empty.OpenContainers(&awsx.Config{}); got != nil {
		t.Fatalf("OpenContainers with no selection should be nil, got %v", got)
	}
}
