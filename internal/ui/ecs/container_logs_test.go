package ecs

import (
	"strings"
	"testing"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func newLogsModel(t *testing.T) ContainerLogsModel {
	t.Helper()
	m := NewContainerLogs(
		awsx.NewEcsClient(&awsx.Config{}),
		awsx.NewLogsClient(&awsx.Config{}),
		"demo-cluster", "abc123", "web",
	)
	m.SetSize(100, 30)
	return m
}

func TestContainerLogsResolvingThenTail(t *testing.T) {
	m := newLogsModel(t)
	if v := m.View(); !strings.Contains(v, "resolving log config") {
		t.Fatalf("expected resolving state; got:\n%s", v)
	}

	m, _ = m.Update(containerLogTargetMsg{
		container: "web",
		target:    &awsx.ContainerLogTarget{LogGroup: "/ecs/demo", StreamPrefix: "ecs/web/abc123"},
	})

	v := m.View()
	if strings.Contains(v, "resolving") {
		t.Fatalf("should have left resolving state; got:\n%s", v)
	}
	for _, want := range []string{"logs: web", "/ecs/demo"} {
		if !strings.Contains(v, want) {
			t.Fatalf("tail view missing %q; got:\n%s", want, v)
		}
	}
}

func TestContainerLogsResolveError(t *testing.T) {
	m := newLogsModel(t)
	m, _ = m.Update(containerLogTargetMsg{container: "web", err: errString("no awslogs driver")})
	if v := m.View(); !strings.Contains(v, "no awslogs driver") {
		t.Fatalf("expected resolve error in view; got:\n%s", v)
	}
}

func TestContainerLogsIgnoresStaleResolve(t *testing.T) {
	m := newLogsModel(t)
	m, _ = m.Update(containerLogTargetMsg{
		container: "other",
		target:    &awsx.ContainerLogTarget{LogGroup: "/ecs/other"},
	})
	if v := m.View(); !strings.Contains(v, "resolving log config") {
		t.Fatalf("stale resolve should be ignored, still resolving; got:\n%s", v)
	}
}

// TestContainerDrillOpensLogs verifies enter on a selected container builds the
// logs screen (#49 -> #54 drill).
func TestContainerDrillOpensLogs(t *testing.T) {
	cl := &containerListScreen{m: NewContainerList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "abc123")}
	cl.m, _ = cl.m.Update(containersLoadedMsg{task: "abc123", containers: []awsx.ContainerSummary{{Name: "web"}}})

	scr := cl.OpenLogs(&awsx.Config{})
	if scr == nil || scr.Title() != "logs" {
		t.Fatalf("OpenLogs should yield a 'logs' screen, got %v", scr)
	}

	empty := &containerListScreen{m: NewContainerList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "abc123")}
	if got := empty.OpenLogs(&awsx.Config{}); got != nil {
		t.Fatalf("OpenLogs with no selection should be nil, got %v", got)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
