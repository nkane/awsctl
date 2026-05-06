package aws

import (
	"context"
	"errors"
	"fmt"

	cwl "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// LogsClient wraps CloudWatch Logs.
//
// v1 uses FilterLogEvents polling for the log tail viewer. The plan
// originally called for StartLiveTail (real-time event stream), but
// LocalStack community does not implement it; production builds may
// upgrade by introducing a separate LiveTail method later.
type LogsClient struct {
	api *cwl.Client
}

// NewLogsClient constructs a LogsClient.
func NewLogsClient(cfg *Config) *LogsClient {
	return &LogsClient{api: cwl.NewFromConfig(cfg.AWS)}
}

// LambdaLogGroup returns the conventional CloudWatch log group for a Lambda.
func LambdaLogGroup(functionName string) string {
	return "/aws/lambda/" + functionName
}

// FilterInput drives a single FilterLogEvents call.
type FilterInput struct {
	LogGroup    string
	StartMillis int64 // unix ms; events at or after
	NextToken   string
	Limit       int32
}

// LogEvent is a normalized log line.
type LogEvent struct {
	Timestamp int64
	Message   string
	Stream    string
}

// FilterPage holds one page of log events plus continuation token.
type FilterPage struct {
	Events    []LogEvent
	NextToken string
}

// Filter polls CloudWatch Logs once and returns the events plus a
// continuation token for the next call.
func (c *LogsClient) Filter(ctx context.Context, in FilterInput) (*FilterPage, error) {
	req := &cwl.FilterLogEventsInput{LogGroupName: &in.LogGroup}
	if in.StartMillis > 0 {
		req.StartTime = &in.StartMillis
	}
	if in.NextToken != "" {
		req.NextToken = &in.NextToken
	}
	if in.Limit > 0 {
		req.Limit = &in.Limit
	}
	resp, err := c.api.FilterLogEvents(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("logs: filter %q: %w", in.LogGroup, err)
	}
	out := &FilterPage{Events: make([]LogEvent, 0, len(resp.Events))}
	for _, e := range resp.Events {
		out.Events = append(out.Events, toEvent(e))
	}
	if resp.NextToken != nil {
		out.NextToken = *resp.NextToken
	}
	return out, nil
}

// EnsureGroup creates the log group if it does not already exist. Used by
// tests and exposed for the v2 deploy flow.
func (c *LogsClient) EnsureGroup(ctx context.Context, name string) error {
	_, err := c.api.CreateLogGroup(ctx, &cwl.CreateLogGroupInput{LogGroupName: &name})
	if err == nil {
		return nil
	}
	// Treat already-exists as success.
	var exists *cwltypes.ResourceAlreadyExistsException
	if errors.As(err, &exists) {
		return nil
	}
	return fmt.Errorf("logs: ensure group %q: %w", name, err)
}

func toEvent(e cwltypes.FilteredLogEvent) LogEvent {
	out := LogEvent{}
	if e.Timestamp != nil {
		out.Timestamp = *e.Timestamp
	}
	if e.Message != nil {
		out.Message = *e.Message
	}
	if e.LogStreamName != nil {
		out.Stream = *e.LogStreamName
	}
	return out
}
