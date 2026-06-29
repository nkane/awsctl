package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// MetricClient wraps CloudWatch GetMetricData with TUI-friendly helpers.
type MetricClient struct {
	api *cloudwatch.Client
}

// NewMetricClient constructs a MetricClient from a resolved Config.
func NewMetricClient(cfg *Config) *MetricClient {
	return &MetricClient{api: cloudwatch.NewFromConfig(cfg.AWS)}
}

// MetricPoint is one timestamped datapoint.
type MetricPoint struct {
	Timestamp time.Time
	Value     float64
}

// MetricSeries is one metric's points, ordered oldest→newest.
type MetricSeries struct {
	Label  string // e.g. "Invocations"
	Unit   string // e.g. "Count", "Milliseconds", "Percent"
	Points []MetricPoint
}

// MetricQuery requests one metric over a window.
type MetricQuery struct {
	Namespace  string            // "AWS/Lambda" | "AWS/DynamoDB" | "AWS/ECS"
	MetricName string            // "Invocations", "Errors", ...
	Dimensions map[string]string // {"FunctionName": "demo-hello"}
	Stat       string            // "Sum" | "Average" | "Maximum"
	Period     time.Duration     // bucket width, e.g. 5*time.Minute
	Start, End time.Time
}

// GetMetrics fetches one series per query in a single GetMetricData call.
// Series are returned in the SAME ORDER as qs. A query that matches no data
// yields a MetricSeries with an empty (non-nil) Points slice, not an error.
// Errors are returned only on API failure.
func (c *MetricClient) GetMetrics(ctx context.Context, qs []MetricQuery) ([]MetricSeries, error) {
	if len(qs) == 0 {
		return []MetricSeries{}, nil
	}

	queries := make([]cwtypes.MetricDataQuery, len(qs))
	for i, q := range qs {
		id := fmt.Sprintf("m%d", i)
		queries[i] = cwtypes.MetricDataQuery{
			Id: awssdk.String(id),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  awssdk.String(q.Namespace),
					MetricName: awssdk.String(q.MetricName),
					Dimensions: dimensions(q.Dimensions),
				},
				Period: awssdk.Int32(int32(q.Period.Seconds())),
				Stat:   awssdk.String(q.Stat),
			},
			ReturnData: awssdk.Bool(true),
		}
	}

	out, err := c.api.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
		MetricDataQueries: queries,
		StartTime:         awssdk.Time(qs[0].Start),
		EndTime:           awssdk.Time(qs[0].End),
	})
	if err != nil {
		return nil, fmt.Errorf("cloudwatch: get metric data: %w", err)
	}

	return mapResults(qs, out.MetricDataResults), nil
}

// mapResults restores input order by Id and converts each MetricDataResult into
// a MetricSeries with points sorted oldest→newest. A query with no matching
// result yields a series with an empty but non-nil Points slice.
func mapResults(qs []MetricQuery, results []cwtypes.MetricDataResult) []MetricSeries {
	// Index results by Id so we can restore input order regardless of how the
	// API returns them.
	byID := make(map[string]cwtypes.MetricDataResult, len(results))
	for _, r := range results {
		if r.Id != nil {
			byID[*r.Id] = r
		}
	}

	series := make([]MetricSeries, len(qs))
	for i, q := range qs {
		s := MetricSeries{
			Label:  q.MetricName,
			Points: []MetricPoint{},
		}
		if r, ok := byID[fmt.Sprintf("m%d", i)]; ok {
			if r.Label != nil && *r.Label != "" {
				s.Label = *r.Label
			}
			// Timestamps[x] corresponds to Values[x]; guard against any
			// length mismatch defensively.
			n := len(r.Timestamps)
			if len(r.Values) < n {
				n = len(r.Values)
			}
			pts := make([]MetricPoint, 0, n)
			for j := 0; j < n; j++ {
				pts = append(pts, MetricPoint{Timestamp: r.Timestamps[j], Value: r.Values[j]})
			}
			// The API returns datapoints newest-first; present oldest→newest.
			sort.Slice(pts, func(a, b int) bool {
				return pts[a].Timestamp.Before(pts[b].Timestamp)
			})
			s.Points = pts
		}
		series[i] = s
	}
	return series
}

// dimensions converts a name→value map into the SDK's Dimension slice. Returns
// nil for an empty map so the marshaled request omits the field entirely.
func dimensions(m map[string]string) []cwtypes.Dimension {
	if len(m) == 0 {
		return nil
	}
	// Sort by name for deterministic request payloads (helps test stability).
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]cwtypes.Dimension, 0, len(names))
	for _, name := range names {
		v := m[name]
		out = append(out, cwtypes.Dimension{
			Name:  awssdk.String(name),
			Value: awssdk.String(v),
		})
	}
	return out
}
