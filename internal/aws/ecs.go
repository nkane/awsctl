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

// ServiceSummary is the UI-facing view of an ECS service.
type ServiceSummary struct {
	Name       string
	Status     string
	Desired    int32
	Running    int32
	Pending    int32
	LaunchType string
	TaskDef    string // family:revision
	Rollout    string // primary deployment rollout state
}

// ListServices returns every service in a cluster with its deployment stats.
// ListServices pages ARNs; DescribeServices is batched at 10 per call (the API
// maximum).
func (c *EcsClient) ListServices(ctx context.Context, cluster string) ([]ServiceSummary, error) {
	arns := []string{}
	p := ecs.NewListServicesPaginator(c.api, &ecs.ListServicesInput{Cluster: &cluster})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ecs: list services %q: %w", cluster, err)
		}
		arns = append(arns, page.ServiceArns...)
	}
	if len(arns) == 0 {
		return nil, nil
	}

	out := make([]ServiceSummary, 0, len(arns))
	for start := 0; start < len(arns); start += 10 {
		end := start + 10
		if end > len(arns) {
			end = len(arns)
		}
		resp, err := c.api.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  &cluster,
			Services: arns[start:end],
		})
		if err != nil {
			return nil, fmt.Errorf("ecs: describe services %q: %w", cluster, err)
		}
		for _, svc := range resp.Services {
			out = append(out, serviceSummary(svc))
		}
	}
	return out, nil
}

// TaskSummary is the UI-facing view of an ECS task.
type TaskSummary struct {
	ID         string // task id (ARN tail)
	LastStatus string
	Desired    string // desired status
	Health     string
	LaunchType string
	TaskDef    string // family:revision
	StartedAt  string // RFC3339, or "" if not started
}

// ListTasks returns the tasks for a service in a cluster with their status.
// ListTasks pages task ARNs; DescribeTasks is batched at 100 per call.
func (c *EcsClient) ListTasks(ctx context.Context, cluster, service string) ([]TaskSummary, error) {
	arns := []string{}
	p := ecs.NewListTasksPaginator(c.api, &ecs.ListTasksInput{
		Cluster:     &cluster,
		ServiceName: &service,
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ecs: list tasks %q/%q: %w", cluster, service, err)
		}
		arns = append(arns, page.TaskArns...)
	}
	if len(arns) == 0 {
		return nil, nil
	}

	out := make([]TaskSummary, 0, len(arns))
	for start := 0; start < len(arns); start += 100 {
		end := start + 100
		if end > len(arns) {
			end = len(arns)
		}
		resp, err := c.api.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: &cluster,
			Tasks:   arns[start:end],
		})
		if err != nil {
			return nil, fmt.Errorf("ecs: describe tasks %q: %w", cluster, err)
		}
		for _, tk := range resp.Tasks {
			out = append(out, taskSummary(tk))
		}
	}
	return out, nil
}

func taskSummary(tk ecstypes.Task) TaskSummary {
	s := TaskSummary{
		Health:     string(tk.HealthStatus),
		LaunchType: string(tk.LaunchType),
	}
	if tk.TaskArn != nil {
		s.ID = shortTaskDef(*tk.TaskArn) // ARN tail is the task id
	}
	if tk.LastStatus != nil {
		s.LastStatus = *tk.LastStatus
	}
	if tk.DesiredStatus != nil {
		s.Desired = *tk.DesiredStatus
	}
	if tk.TaskDefinitionArn != nil {
		s.TaskDef = shortTaskDef(*tk.TaskDefinitionArn)
	}
	if tk.StartedAt != nil {
		s.StartedAt = tk.StartedAt.Format("2006-01-02 15:04:05")
	}
	return s
}

// ContainerLogTarget locates a container's CloudWatch logs.
type ContainerLogTarget struct {
	LogGroup     string
	StreamPrefix string // {prefix}/{container}/{taskId}
}

// ResolveContainerLog resolves the awslogs CloudWatch target for a container by
// reading the task's definition. Errors if the container does not use the
// awslogs log driver.
func (c *EcsClient) ResolveContainerLog(ctx context.Context, cluster, task, container string) (*ContainerLogTarget, error) {
	dt, err := c.api.DescribeTasks(ctx, &ecs.DescribeTasksInput{Cluster: &cluster, Tasks: []string{task}})
	if err != nil {
		return nil, fmt.Errorf("ecs: describe task %q: %w", task, err)
	}
	if len(dt.Tasks) == 0 || dt.Tasks[0].TaskDefinitionArn == nil {
		return nil, fmt.Errorf("ecs: task %q not found", task)
	}
	taskID := shortTaskDef(*dt.Tasks[0].TaskArn)
	tdArn := *dt.Tasks[0].TaskDefinitionArn

	dtd, err := c.api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{TaskDefinition: &tdArn})
	if err != nil {
		return nil, fmt.Errorf("ecs: describe task-definition: %w", err)
	}
	if dtd.TaskDefinition == nil {
		return nil, fmt.Errorf("ecs: empty task-definition for %q", tdArn)
	}

	for _, cd := range dtd.TaskDefinition.ContainerDefinitions {
		if cd.Name == nil || *cd.Name != container {
			continue
		}
		lc := cd.LogConfiguration
		if lc == nil || lc.LogDriver != ecstypes.LogDriverAwslogs {
			return nil, fmt.Errorf("ecs: container %q does not use the awslogs driver", container)
		}
		group := lc.Options["awslogs-group"]
		if group == "" {
			return nil, fmt.Errorf("ecs: container %q has no awslogs-group", container)
		}
		prefix := lc.Options["awslogs-stream-prefix"]
		stream := container + "/" + taskID
		if prefix != "" {
			stream = prefix + "/" + stream
		}
		return &ContainerLogTarget{LogGroup: group, StreamPrefix: stream}, nil
	}
	return nil, fmt.Errorf("ecs: container %q not found in task definition", container)
}

// ContainerSummary is the UI-facing view of a container within a task.
type ContainerSummary struct {
	Name       string
	Image      string
	LastStatus string
	Health     string
	ExitCode   string // "" when the container has not exited
	Reason     string
}

// DescribeTaskContainers returns the containers of a single task. The task may
// be given as an id or full ARN.
func (c *EcsClient) DescribeTaskContainers(ctx context.Context, cluster, task string) ([]ContainerSummary, error) {
	resp, err := c.api.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   []string{task},
	})
	if err != nil {
		return nil, fmt.Errorf("ecs: describe task %q: %w", task, err)
	}
	if len(resp.Tasks) == 0 {
		return nil, fmt.Errorf("ecs: task %q not found in cluster %q", task, cluster)
	}
	cs := resp.Tasks[0].Containers
	out := make([]ContainerSummary, 0, len(cs))
	for _, ct := range cs {
		out = append(out, containerSummary(ct))
	}
	return out, nil
}

func containerSummary(ct ecstypes.Container) ContainerSummary {
	s := ContainerSummary{Health: string(ct.HealthStatus)}
	if ct.Name != nil {
		s.Name = *ct.Name
	}
	if ct.Image != nil {
		s.Image = *ct.Image
	}
	if ct.LastStatus != nil {
		s.LastStatus = *ct.LastStatus
	}
	if ct.ExitCode != nil {
		s.ExitCode = fmt.Sprintf("%d", *ct.ExitCode)
	}
	if ct.Reason != nil {
		s.Reason = *ct.Reason
	}
	return s
}

// DescribeService returns the full description of a single service, including
// deployments, events, network config, and load balancers.
func (c *EcsClient) DescribeService(ctx context.Context, cluster, name string) (*ecstypes.Service, error) {
	resp, err := c.api.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{name},
	})
	if err != nil {
		return nil, fmt.Errorf("ecs: describe service %q: %w", name, err)
	}
	if len(resp.Services) == 0 {
		return nil, fmt.Errorf("ecs: service %q not found in cluster %q", name, cluster)
	}
	return &resp.Services[0], nil
}

func serviceSummary(svc ecstypes.Service) ServiceSummary {
	s := ServiceSummary{
		Desired:    svc.DesiredCount,
		Running:    svc.RunningCount,
		Pending:    svc.PendingCount,
		LaunchType: string(svc.LaunchType),
	}
	if svc.ServiceName != nil {
		s.Name = *svc.ServiceName
	}
	if svc.Status != nil {
		s.Status = *svc.Status
	}
	if svc.TaskDefinition != nil {
		s.TaskDef = shortTaskDef(*svc.TaskDefinition)
	}
	// The primary deployment carries the live rollout state.
	for _, d := range svc.Deployments {
		if d.Status != nil && *d.Status == "PRIMARY" {
			s.Rollout = string(d.RolloutState)
			break
		}
	}
	return s
}

// shortTaskDef trims a task-definition ARN to its family:revision tail.
func shortTaskDef(arn string) string {
	for i := len(arn) - 1; i >= 0; i-- {
		if arn[i] == '/' {
			return arn[i+1:]
		}
	}
	return arn
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
