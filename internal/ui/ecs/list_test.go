package ecs

import (
	"strings"
	"testing"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func TestListRendersClusters(t *testing.T) {
	m := NewList()
	m.SetClient(awsx.NewEcsClient(&awsx.Config{}))
	m.SetSize(100, 30)

	m, _ = m.Update(loadedMsg{clusters: []awsx.ClusterSummary{
		{Name: "demo-cluster", Status: "ACTIVE", RunningTasks: 3, PendingTasks: 1, ActiveServices: 2, ContainerInstances: 4},
		{Name: "prod-cluster", Status: "ACTIVE"},
	}})

	if !m.Loaded() {
		t.Fatal("model should be loaded after loadedMsg")
	}
	v := m.View()
	for _, want := range []string{"demo-cluster", "prod-cluster", "ACTIVE", "3 running", "2 services"} {
		if !strings.Contains(v, want) {
			t.Fatalf("view missing %q; got:\n%s", want, v)
		}
	}
	if got := m.Selected(); got != "demo-cluster" {
		t.Fatalf("Selected() = %q, want demo-cluster", got)
	}
}

func TestListEmptyState(t *testing.T) {
	m := NewList()
	m.SetClient(awsx.NewEcsClient(&awsx.Config{}))
	m.SetSize(100, 30)
	m, _ = m.Update(loadedMsg{clusters: nil})
	if v := m.View(); !strings.Contains(v, "no ECS clusters") {
		t.Fatalf("expected empty-state message; got:\n%s", v)
	}
}
