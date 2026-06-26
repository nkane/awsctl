package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// EcsClient wraps the SDK ECS client.
type EcsClient struct {
	api *ecs.Client
}

// NewEcsClient constructs an EcsClient.
func NewEcsClient(cfg *Config) *EcsClient {
	return &EcsClient{api: ecs.NewFromConfig(cfg.AWS)}
}

// ClusterSummary is the UI-facing view of an ECS cluster, flattened so the UI
// does not import the SDK types package.
type ClusterSummary struct {
	Name               string
	Status             string
	RunningTasks       int32
	PendingTasks       int32
	ActiveServices     int32
	ContainerInstances int32
}

// ListClusters returns every cluster in the region with its task/service stats.
// It pages ListClusters for ARNs, then batches DescribeClusters (max 100 per
// call) to fill in counts.
func (c *EcsClient) ListClusters(ctx context.Context) ([]ClusterSummary, error) {
	arns := []string{}
	p := ecs.NewListClustersPaginator(c.api, &ecs.ListClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ecs: list clusters: %w", err)
		}
		arns = append(arns, page.ClusterArns...)
	}
	if len(arns) == 0 {
		return nil, nil
	}

	out := make([]ClusterSummary, 0, len(arns))
	for start := 0; start < len(arns); start += 100 {
		end := start + 100
		if end > len(arns) {
			end = len(arns)
		}
		resp, err := c.api.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: arns[start:end],
		})
		if err != nil {
			return nil, fmt.Errorf("ecs: describe clusters: %w", err)
		}
		for _, cl := range resp.Clusters {
			out = append(out, clusterSummary(cl))
		}
	}
	return out, nil
}

func clusterSummary(cl ecstypes.Cluster) ClusterSummary {
	s := ClusterSummary{
		RunningTasks:       cl.RunningTasksCount,
		PendingTasks:       cl.PendingTasksCount,
		ActiveServices:     cl.ActiveServicesCount,
		ContainerInstances: cl.RegisteredContainerInstancesCount,
	}
	if cl.ClusterName != nil {
		s.Name = *cl.ClusterName
	}
	if cl.Status != nil {
		s.Status = *cl.Status
	}
	return s
}
