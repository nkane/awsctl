package ecs

import (
	"strings"
	"testing"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func TestServiceEventsRenders(t *testing.T) {
	m := NewServiceEvents(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")
	m.SetSize(100, 30)

	m, _ = m.Update(eventsLoadedMsg{service: "api", events: []awsx.EventSummary{
		{CreatedAt: "2026-06-26 09:05:00", Message: "(service api) has reached a steady state."},
		{CreatedAt: "2026-06-26 09:04:00", Message: "(service api) registered 1 targets."},
	}})

	v := m.View()
	for _, want := range []string{"events: api", "steady state", "registered 1 targets", "09:05:00"} {
		if !strings.Contains(v, want) {
			t.Fatalf("events view missing %q; got:\n%s", want, v)
		}
	}
}

func TestServiceEventsEmpty(t *testing.T) {
	m := NewServiceEvents(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")
	m.SetSize(100, 30)
	m, _ = m.Update(eventsLoadedMsg{service: "api", events: nil})
	if v := m.View(); !strings.Contains(v, "no events") {
		t.Fatalf("expected empty-state message; got:\n%s", v)
	}
}

func TestServiceEventsIgnoresStaleLoad(t *testing.T) {
	m := NewServiceEvents(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")
	m.SetSize(100, 30)
	m, _ = m.Update(eventsLoadedMsg{service: "other", events: []awsx.EventSummary{{Message: "ghost"}}})
	if strings.Contains(m.View(), "ghost") {
		t.Fatalf("stale events load leaked into view")
	}
}

// TestDescribeDrillOpensEvents verifies 'e' on the service describe builds the
// events panel (#46 -> #47 drill).
func TestDescribeDrillOpensEvents(t *testing.T) {
	sd := &serviceDescribeScreen{m: NewServiceDescribe(awsx.NewEcsClient(&awsx.Config{}), "demo-cluster", "api")}
	scr := sd.OpenEvents(&awsx.Config{})
	if scr == nil || scr.Title() != "events" {
		t.Fatalf("OpenEvents should yield an 'events' screen, got %v", scr)
	}
}
