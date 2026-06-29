//go:build integration

package aws

import (
	"context"
	"strings"
	"testing"
	"time"
)

// cwUnsupported reports whether err is LocalStack signalling that CloudWatch
// GetMetricData is not implemented in the community edition. It surfaces as an
// HTTP 500 with a malformed smithy-protocol response header. Real AWS and
// LocalStack Pro return data, so we only skip on this specific signature.
func cwUnsupported(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "StatusCode: 500") ||
		strings.Contains(msg, "smithy-protocol") ||
		strings.Contains(msg, "not yet implemented")
}

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
	if cwUnsupported(err) {
		t.Skipf("CloudWatch GetMetricData unsupported by this endpoint: %v", err)
	}
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
