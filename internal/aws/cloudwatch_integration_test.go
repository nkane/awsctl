//go:build integration

package aws

import (
	"context"
	"testing"
	"time"
)

// ---------- CloudWatch metrics ----------

// TestMetricsGetEmpty asserts GetMetrics succeeds against LocalStack CloudWatch
// even when the metrics have no data, returning one series per query with an
// empty but non-nil Points slice.
func TestMetricsGetEmpty(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	mc := NewMetricClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	end := time.Now()
	start := end.Add(-3 * time.Hour)
	qs := []MetricQuery{
		{
			Namespace:  "AWS/Lambda",
			MetricName: "Invocations",
			Dimensions: map[string]string{"FunctionName": uniqueName("no-such-fn")},
			Stat:       "Sum",
			Period:     5 * time.Minute,
			Start:      start,
			End:        end,
		},
		{
			Namespace:  "AWS/Lambda",
			MetricName: "Errors",
			Dimensions: map[string]string{"FunctionName": uniqueName("no-such-fn")},
			Stat:       "Sum",
			Period:     5 * time.Minute,
			Start:      start,
			End:        end,
		},
	}

	series, err := mc.GetMetrics(ctx, qs)
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if len(series) != len(qs) {
		t.Fatalf("expected %d series, got %d", len(qs), len(series))
	}
	for i, s := range series {
		if s.Points == nil {
			t.Fatalf("series[%d] (%s) Points must be non-nil even with no data", i, s.Label)
		}
	}
}
