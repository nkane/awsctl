package ecs

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	awsx "github.com/nkane/awsctl/internal/aws"
)

func TestServiceDescribeRenders(t *testing.T) {
	m := NewServiceDescribe(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")
	m.SetSize(100, 30)

	svc := &ecstypes.Service{
		ServiceName:    aws.String("api"),
		Status:         aws.String("ACTIVE"),
		DesiredCount:   3,
		RunningCount:   2,
		PendingCount:   1,
		LaunchType:     ecstypes.LaunchTypeFargate,
		TaskDefinition: aws.String("arn:aws:ecs:us-east-1:0:task-definition/api:7"),
		Deployments: []ecstypes.Deployment{{
			Status:       aws.String("PRIMARY"),
			RolloutState: ecstypes.DeploymentRolloutStateInProgress,
			DesiredCount: 3,
			RunningCount: 2,
		}},
		LoadBalancers: []ecstypes.LoadBalancer{{
			ContainerName:  aws.String("web"),
			ContainerPort:  aws.Int32(8080),
			TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:tg/web"),
		}},
		NetworkConfiguration: &ecstypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
				Subnets:        []string{"subnet-1", "subnet-2"},
				SecurityGroups: []string{"sg-1"},
				AssignPublicIp: ecstypes.AssignPublicIpEnabled,
			},
		},
		Events: []ecstypes.ServiceEvent{{Message: aws.String("steady state")}},
	}

	m, _ = m.Update(serviceDescribeLoadedMsg{name: "api", svc: svc})

	v := m.View()
	for _, want := range []string{"api:7", "FARGATE", "IN_PROGRESS", "web:8080", "subnet-1", "steady state"} {
		if !strings.Contains(v, want) {
			t.Fatalf("describe view missing %q; got:\n%s", want, v)
		}
	}
}

func TestServiceDescribeIgnoresStaleLoad(t *testing.T) {
	m := NewServiceDescribe(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")
	m.SetSize(100, 30)
	m, _ = m.Update(serviceDescribeLoadedMsg{name: "other", svc: &ecstypes.Service{ServiceName: aws.String("other")}})
	if strings.Contains(m.View(), "other") {
		t.Fatalf("stale describe load leaked into view")
	}
}

// TestServiceDrillOpensDescribe verifies enter on a selected service builds the
// describe screen (#45 -> #46 drill).
func TestServiceDrillOpensDescribe(t *testing.T) {
	sl := &serviceListScreen{m: NewServiceList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster")}
	sl.m, _ = sl.m.Update(servicesLoadedMsg{cluster: "demo-cluster", services: []awsx.ServiceSummary{{Name: "api"}}})

	scr := sl.OpenDescribe(&awsx.Config{})
	if scr == nil {
		t.Fatal("OpenDescribe returned nil for a selected service")
	}
	if scr.Title() != "describe" {
		t.Fatalf("drilled screen Title = %q, want describe", scr.Title())
	}

	empty := &serviceListScreen{m: NewServiceList(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster")}
	if got := empty.OpenDescribe(&awsx.Config{}); got != nil {
		t.Fatalf("OpenDescribe with no selection should be nil, got %v", got)
	}
}
