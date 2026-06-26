package ecs

import (
	"testing"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func TestContainerExecTarget(t *testing.T) {
	cl := &containerListScreen{m: NewContainerList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "abc123")}
	cl.m, _ = cl.m.Update(containersLoadedMsg{task: "abc123", containers: []awsx.ContainerSummary{{Name: "web"}}})

	cluster, task, container, ok := cl.ExecTarget()
	if !ok {
		t.Fatal("ExecTarget should be ok with a selection")
	}
	if cluster != "demo-cluster" || task != "abc123" || container != "web" {
		t.Fatalf("ExecTarget = (%q,%q,%q), want (demo-cluster, abc123, web)", cluster, task, container)
	}

	empty := &containerListScreen{m: NewContainerList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "abc123")}
	if _, _, _, ok := empty.ExecTarget(); ok {
		t.Fatal("ExecTarget should be !ok with no selection")
	}
}
