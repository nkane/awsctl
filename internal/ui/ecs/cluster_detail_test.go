package ecs

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	awsx "github.com/nkane/awsctl/internal/aws"
)

func TestClusterDescribeRenders(t *testing.T) {
	m := NewClusterDescribe(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster")
	m.SetSize(100, 30)

	cl := &ecstypes.Cluster{
		ClusterName:                       aws.String("demo-cluster"),
		Status:                            aws.String("ACTIVE"),
		RunningTasksCount:                 5,
		ActiveServicesCount:               2,
		RegisteredContainerInstancesCount: 3,
		CapacityProviders:                 []string{"FARGATE", "FARGATE_SPOT"},
		Settings: []ecstypes.ClusterSetting{
			{Name: ecstypes.ClusterSettingNameContainerInsights, Value: aws.String("enabled")},
		},
		Statistics: []ecstypes.KeyValuePair{
			{Name: aws.String("runningFargateTasksCount"), Value: aws.String("5")},
		},
		Tags: []ecstypes.Tag{{Key: aws.String("env"), Value: aws.String("prod")}},
	}

	m, _ = m.Update(clusterDescribeLoadedMsg{name: "demo-cluster", cluster: cl})

	v := m.View()
	for _, want := range []string{"cluster: demo-cluster", "ACTIVE", "FARGATE_SPOT", "containerInsights", "runningFargateTasksCount", "env", "prod"} {
		if !strings.Contains(v, want) {
			t.Fatalf("cluster describe view missing %q; got:\n%s", want, v)
		}
	}
}

func TestClusterDescribeIgnoresStaleLoad(t *testing.T) {
	m := NewClusterDescribe(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster")
	m.SetSize(100, 30)
	m, _ = m.Update(clusterDescribeLoadedMsg{name: "other", cluster: &ecstypes.Cluster{ClusterName: aws.String("other")}})
	if strings.Contains(m.View(), "other") {
		t.Fatalf("stale cluster describe load leaked into view")
	}
}

// TestClusterDrillOpensDescribe verifies 'd' on the cluster list builds the
// cluster describe (#43 -> #44 drill).
func TestClusterDrillOpensDescribe(t *testing.T) {
	rl := NewListScreen()
	rl.SetClient(awsx.NewEcsClient(&awsx.Config{}))
	rl.Update(loadedMsg{clusters: []awsx.ClusterSummary{{Name: "demo-cluster", Status: "ACTIVE"}}})

	scr := rl.OpenDescribe(&awsx.Config{})
	if scr == nil || scr.Title() != "describe" {
		t.Fatalf("OpenDescribe should yield a 'describe' screen, got %v", scr)
	}

	empty := NewListScreen()
	if got := empty.OpenDescribe(&awsx.Config{}); got != nil {
		t.Fatalf("OpenDescribe with no selection should be nil, got %v", got)
	}
}
