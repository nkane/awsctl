package lambda

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	awsx "github.com/nkane/awsctl/internal/aws"
)

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestMetricsTabSmoke(t *testing.T) {
	m := NewDetail(awsx.NewLambdaClient(&awsx.Config{}), &awsx.Config{}, "demo-hello")
	m.SetSize(120, 30)
	m, _ = m.Update(detailLoadedMsg{name: "demo-hello", d: &awsx.FunctionDetail{}})
	// Switch to the Metrics tab (last tab) via shift+tab from Config (index 0).
	m, cmd := m.Update(keyMsg("shift+tab"))
	if m.tab != TabMetrics {
		t.Fatalf("expected TabMetrics, got %d", m.tab)
	}
	if cmd == nil {
		t.Fatalf("opening metrics tab should return a fetch cmd")
	}
	v := m.View()
	// Selector pills + at least one chart title.
	for _, want := range []string{"1h", "3h", "7d", "Invocations", "Duration"} {
		if !strings.Contains(v, want) {
			t.Fatalf("metrics view missing %q; got:\n%s", want, v)
		}
	}
	// A range key (l) should advance selection and trigger a refetch.
	before := m.metrics.tr.Selected().Label
	m, cmd = m.Update(keyMsg("l"))
	if cmd == nil {
		t.Fatalf("range key should trigger refetch cmd")
	}
	if m.metrics.tr.Selected().Label == before {
		t.Fatalf("range key did not change selection (still %q)", before)
	}
}
