package components

import (
	"strings"
	"testing"
	"time"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func TestChartViewContainsTitle(t *testing.T) {
	c := NewChart("Invocations")
	c.SetSize(60, 12)
	if !strings.Contains(c.View(), "Invocations") {
		t.Fatalf("View() should contain the title, got:\n%s", c.View())
	}
}

func TestChartViewEmptySeriesShowsPlaceholder(t *testing.T) {
	c := NewChart("Errors")
	c.SetSize(60, 12)

	// Empty (non-nil) points.
	c = c.WithSeries(awsx.MetricSeries{Label: "Errors", Points: []awsx.MetricPoint{}})
	got := c.View()
	if !strings.Contains(got, emptyChartMsg) {
		t.Fatalf("empty series should render %q, got:\n%s", emptyChartMsg, got)
	}
	if !strings.Contains(got, "Errors") {
		t.Fatalf("empty series view should still contain the title, got:\n%s", got)
	}
}

func TestChartViewNoSizeStillRenders(t *testing.T) {
	// View must be robust even when SetSize was never called.
	c := NewChart("Duration")
	got := c.View()
	if !strings.Contains(got, emptyChartMsg) {
		t.Fatalf("unsized empty chart should render placeholder, got:\n%s", got)
	}
}

func TestChartViewWithDataNoPlaceholder(t *testing.T) {
	c := NewChart("Invocations")
	c.SetSize(60, 12)

	base := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	pts := []awsx.MetricPoint{
		{Timestamp: base, Value: 1},
		{Timestamp: base.Add(5 * time.Minute), Value: 7},
		{Timestamp: base.Add(10 * time.Minute), Value: 3},
		{Timestamp: base.Add(15 * time.Minute), Value: 9},
	}
	c = c.WithSeries(awsx.MetricSeries{Label: "Invocations", Unit: "Count", Points: pts})

	got := c.View()
	if strings.Contains(got, emptyChartMsg) {
		t.Fatalf("populated chart should not render placeholder, got:\n%s", got)
	}
	if !strings.Contains(got, "Invocations") {
		t.Fatalf("populated chart view should contain the title, got:\n%s", got)
	}
}

func TestChartWithSeriesDoesNotMutateOriginal(t *testing.T) {
	orig := NewChart("M")
	orig.SetSize(40, 8)
	withData := orig.WithSeries(awsx.MetricSeries{Points: []awsx.MetricPoint{
		{Timestamp: time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC), Value: 5},
	}})

	if !strings.Contains(orig.View(), emptyChartMsg) {
		t.Fatalf("original chart should remain empty after WithSeries copy")
	}
	if strings.Contains(withData.View(), emptyChartMsg) {
		t.Fatalf("derived chart should render data, not placeholder")
	}
}
