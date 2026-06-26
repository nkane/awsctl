package ecs

import (
	"strings"
	"testing"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func TestServiceListRenders(t *testing.T) {
	m := NewServiceList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster")
	m.SetSize(100, 30)

	m, _ = m.Update(servicesLoadedMsg{cluster: "demo-cluster", services: []awsx.ServiceSummary{
		{Name: "api", Status: "ACTIVE", Desired: 3, Running: 2, Pending: 1, LaunchType: "FARGATE", TaskDef: "api:7", Rollout: "IN_PROGRESS"},
		{Name: "worker", Status: "ACTIVE", Desired: 1, Running: 1, TaskDef: "worker:3"},
	}})

	v := m.View()
	for _, want := range []string{"api", "worker", "2/3 running", "FARGATE", "api:7", "IN_PROGRESS"} {
		if !strings.Contains(v, want) {
			t.Fatalf("view missing %q; got:\n%s", want, v)
		}
	}
	if got := m.Selected(); got != "api" {
		t.Fatalf("Selected() = %q, want api", got)
	}
	if got := m.Cluster(); got != "demo-cluster" {
		t.Fatalf("Cluster() = %q, want demo-cluster", got)
	}
}

func TestServiceListIgnoresStaleLoad(t *testing.T) {
	m := NewServiceList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster")
	m.SetSize(100, 30)
	// A load that resolved for a different cluster must not populate this one.
	m, _ = m.Update(servicesLoadedMsg{cluster: "other-cluster", services: []awsx.ServiceSummary{{Name: "ghost"}}})
	if got := m.Selected(); got != "" {
		t.Fatalf("stale load leaked; Selected() = %q, want empty", got)
	}
}

// TestClusterDrillOpensServices verifies enter on a selected cluster builds the
// service list screen (the #43 -> #45 drill).
func TestClusterDrillOpensServices(t *testing.T) {
	rl := NewListScreen()
	rl.SetClient(awsx.NewEcsClient(&awsx.Config{}))
	// Populate the cluster list (loadedMsg is a valid tea.Msg) so a row is
	// selected, then drill.
	rl.Update(loadedMsg{clusters: []awsx.ClusterSummary{{Name: "demo-cluster", Status: "ACTIVE"}}})

	scr := rl.OpenServices(&awsx.Config{})
	if scr == nil {
		t.Fatal("OpenServices returned nil for a selected cluster")
	}
	if scr.Title() != "services" {
		t.Fatalf("drilled screen Title = %q, want services", scr.Title())
	}

	// No selection -> no drill.
	empty := NewListScreen()
	if got := empty.OpenServices(&awsx.Config{}); got != nil {
		t.Fatalf("OpenServices with no selection should be nil, got %v", got)
	}
}
