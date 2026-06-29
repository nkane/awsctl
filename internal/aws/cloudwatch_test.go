package aws

import (
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

func TestDimensions(t *testing.T) {
	if got := dimensions(nil); got != nil {
		t.Fatalf("dimensions(nil) = %v, want nil", got)
	}
	if got := dimensions(map[string]string{}); got != nil {
		t.Fatalf("dimensions(empty) = %v, want nil", got)
	}

	got := dimensions(map[string]string{"ServiceName": "svc", "ClusterName": "cl"})
	if len(got) != 2 {
		t.Fatalf("expected 2 dimensions, got %d", len(got))
	}
	// Sorted by name: ClusterName before ServiceName.
	if *got[0].Name != "ClusterName" || *got[0].Value != "cl" {
		t.Fatalf("dim[0] = %s=%s, want ClusterName=cl", *got[0].Name, *got[0].Value)
	}
	if *got[1].Name != "ServiceName" || *got[1].Value != "svc" {
		t.Fatalf("dim[1] = %s=%s, want ServiceName=svc", *got[1].Name, *got[1].Value)
	}
}

func TestMapResultsOrderAndSort(t *testing.T) {
	qs := []MetricQuery{
		{MetricName: "Invocations"},
		{MetricName: "Errors"},
		{MetricName: "Duration"},
	}

	t0 := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Minute)
	t2 := t0.Add(10 * time.Minute)

	// Results arrive out of input order and newest-first within each series.
	results := []cwtypes.MetricDataResult{
		{
			Id:         awssdk.String("m2"),
			Label:      awssdk.String("Duration"),
			Timestamps: []time.Time{t2, t1, t0},
			Values:     []float64{30, 20, 10},
		},
		{
			Id:         awssdk.String("m0"),
			Label:      awssdk.String("Invocations"),
			Timestamps: []time.Time{t1, t0},
			Values:     []float64{2, 1},
		},
		// m1 (Errors) intentionally absent → no data.
	}

	series := mapResults(qs, results)
	if len(series) != 3 {
		t.Fatalf("expected 3 series, got %d", len(series))
	}

	// Input order preserved.
	if series[0].Label != "Invocations" || series[1].Label != "Errors" || series[2].Label != "Duration" {
		t.Fatalf("series order wrong: %q %q %q", series[0].Label, series[1].Label, series[2].Label)
	}

	// Errors has no result → empty but non-nil Points.
	if series[1].Points == nil {
		t.Fatalf("missing series Points should be non-nil")
	}
	if len(series[1].Points) != 0 {
		t.Fatalf("expected 0 points for Errors, got %d", len(series[1].Points))
	}

	// Duration points sorted oldest→newest.
	dur := series[2].Points
	if len(dur) != 3 {
		t.Fatalf("expected 3 Duration points, got %d", len(dur))
	}
	for i := 1; i < len(dur); i++ {
		if dur[i].Timestamp.Before(dur[i-1].Timestamp) {
			t.Fatalf("Duration points not sorted ascending: %v", dur)
		}
	}
	if dur[0].Value != 10 || dur[2].Value != 30 {
		t.Fatalf("Duration values mis-ordered after sort: %v", dur)
	}
}

func TestMapResultsLabelFallback(t *testing.T) {
	qs := []MetricQuery{{MetricName: "Throttles"}}
	// Result present but no Label → falls back to the query MetricName.
	series := mapResults(qs, []cwtypes.MetricDataResult{
		{Id: awssdk.String("m0")},
	})
	if series[0].Label != "Throttles" {
		t.Fatalf("label fallback = %q, want Throttles", series[0].Label)
	}
	if series[0].Points == nil || len(series[0].Points) != 0 {
		t.Fatalf("expected empty non-nil points, got %v", series[0].Points)
	}
}

func TestGetMetricsEmptyQueries(t *testing.T) {
	c := &MetricClient{}
	got, err := c.GetMetrics(nil, nil)
	if err != nil {
		t.Fatalf("GetMetrics(nil): %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil slice, got %v", got)
	}
}
