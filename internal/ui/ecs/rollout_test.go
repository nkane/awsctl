package ecs

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	awsx "github.com/nkane/awsctl/internal/aws"
)

func svcWithRollout(state ecstypes.DeploymentRolloutState, running, desired int32) *ecstypes.Service {
	return &ecstypes.Service{
		ServiceName: aws.String("api"),
		Status:      aws.String("ACTIVE"),
		Deployments: []ecstypes.Deployment{{
			Status:       aws.String("PRIMARY"),
			RolloutState: state,
			RunningCount: running,
			DesiredCount: desired,
		}},
	}
}

func TestRolloutBar(t *testing.T) {
	cases := []struct {
		running, desired int32
		want             string
	}{
		{0, 4, "[----------]"},
		{2, 4, "[#####-----]"},
		{4, 4, "[##########]"},
		{1, 0, "[##########]"}, // desired 0 -> treat as full
	}
	for _, c := range cases {
		if got := rolloutBar(c.running, c.desired); got != c.want {
			t.Fatalf("rolloutBar(%d,%d) = %q, want %q", c.running, c.desired, got, c.want)
		}
	}
}

func TestRolloutLineStates(t *testing.T) {
	if got := rolloutLine(svcWithRollout(ecstypes.DeploymentRolloutStateInProgress, 2, 3)); !strings.Contains(got, "IN_PROGRESS") || !strings.Contains(got, "2/3") {
		t.Fatalf("in-progress line wrong: %q", got)
	}
	if got := rolloutLine(nil); got != "" {
		t.Fatalf("nil service should yield empty rollout line, got %q", got)
	}
}

// TestServiceDescribeWatchesInProgress verifies the describe enters watch mode
// on an in-progress rollout and leaves it once the rollout completes.
func TestServiceDescribeWatchesInProgress(t *testing.T) {
	m := NewServiceDescribe(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")
	m.SetSize(100, 30)

	// In-progress -> watching, and a poll tick is scheduled.
	var cmd interface{}
	m, c := m.Update(serviceDescribeLoadedMsg{name: "api", svc: svcWithRollout(ecstypes.DeploymentRolloutStateInProgress, 1, 3)})
	cmd = c
	if !m.watching {
		t.Fatal("should be watching an in-progress rollout")
	}
	if cmd == nil {
		t.Fatal("expected a poll tick command while watching")
	}
	if !strings.Contains(m.View(), "watching rollout") {
		t.Fatalf("footer should show watching badge; view:\n%s", m.View())
	}

	// Completed -> stop watching.
	m, _ = m.Update(serviceDescribeLoadedMsg{name: "api", svc: svcWithRollout(ecstypes.DeploymentRolloutStateCompleted, 3, 3)})
	if m.watching {
		t.Fatal("should stop watching once rollout completes")
	}
	if !strings.Contains(m.View(), "COMPLETED") {
		t.Fatalf("view should show COMPLETED rollout; view:\n%s", m.View())
	}
}
