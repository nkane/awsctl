package ecs

import (
	"strings"
	"testing"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func TestTaskDefListRenders(t *testing.T) {
	m := NewTaskDefList(awsx.NewEcsClient(&awsx.Config{}))
	m.SetSize(100, 30)

	m, _ = m.Update(taskDefsLoadedMsg{families: []awsx.TaskDefFamilySummary{
		{Family: "api", Revision: "7"},
		{Family: "worker", Revision: "3"},
	}})

	v := m.View()
	for _, want := range []string{"api", "worker", "latest revision 7", "ACTIVE"} {
		if !strings.Contains(v, want) {
			t.Fatalf("task-def list view missing %q; got:\n%s", want, v)
		}
	}
	if got := m.Selected(); got != "api" {
		t.Fatalf("Selected() = %q, want api", got)
	}
}

func TestTaskDefListEmpty(t *testing.T) {
	m := NewTaskDefList(awsx.NewEcsClient(&awsx.Config{}))
	m.SetSize(100, 30)
	m, _ = m.Update(taskDefsLoadedMsg{families: nil})
	if v := m.View(); !strings.Contains(v, "no task definitions") {
		t.Fatalf("expected empty-state message; got:\n%s", v)
	}
}

// TestClusterDrillOpensTaskDefs verifies 't' on the cluster list builds the
// task-def list (#43 -> #50 drill). It does not require a selection.
func TestClusterDrillOpensTaskDefs(t *testing.T) {
	rl := NewListScreen()
	scr := rl.OpenTaskDefs(&awsx.Config{})
	if scr == nil || scr.Title() != "task-defs" {
		t.Fatalf("OpenTaskDefs should yield a 'task-defs' screen, got %v", scr)
	}
	if got := rl.OpenTaskDefs(nil); got != nil {
		t.Fatalf("OpenTaskDefs with nil cfg should be nil, got %v", got)
	}
}
