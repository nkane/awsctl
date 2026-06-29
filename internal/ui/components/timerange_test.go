package components

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestNewTimeRangeDefaults(t *testing.T) {
	tr := NewTimeRange()
	if got := tr.Selected().Label; got != "3h" {
		t.Fatalf("default selection = %q, want \"3h\"", got)
	}

	want := []struct {
		label    string
		duration time.Duration
		period   time.Duration
	}{
		{"1h", time.Hour, time.Minute},
		{"3h", 3 * time.Hour, 5 * time.Minute},
		{"12h", 12 * time.Hour, 5 * time.Minute},
		{"24h", 24 * time.Hour, 15 * time.Minute},
		{"7d", 7 * 24 * time.Hour, time.Hour},
	}
	opts := DefaultRangeOptions()
	if len(opts) != len(want) {
		t.Fatalf("got %d options, want %d", len(opts), len(want))
	}
	for i, w := range want {
		if opts[i].Label != w.label || opts[i].Duration != w.duration || opts[i].Period != w.period {
			t.Errorf("option %d = %+v, want {%s %v %v}", i, opts[i], w.label, w.duration, w.period)
		}
	}
}

func TestTimeRangeUpdateRightChangesSelection(t *testing.T) {
	tr := NewTimeRange() // "3h"

	for _, k := range []string{"right", "l"} {
		start := NewTimeRange()
		next, changed := start.Update(keyMsg(k))
		if !changed {
			t.Fatalf("Update(%q) should report changed=true", k)
		}
		if next.Selected().Label != "12h" {
			t.Fatalf("Update(%q) selection = %q, want \"12h\"", k, next.Selected().Label)
		}
	}
	_ = tr
}

func TestTimeRangeUpdateLeftChangesSelection(t *testing.T) {
	for _, k := range []string{"left", "h"} {
		next, changed := NewTimeRange().Update(keyMsg(k))
		if !changed {
			t.Fatalf("Update(%q) should report changed=true", k)
		}
		if next.Selected().Label != "1h" {
			t.Fatalf("Update(%q) selection = %q, want \"1h\"", k, next.Selected().Label)
		}
	}
}

func TestTimeRangeUpdateBoundaries(t *testing.T) {
	// Left at the first option does not change.
	tr, _ := NewTimeRange().Update(keyMsg("left")) // now "1h"
	if tr.Selected().Label != "1h" {
		t.Fatalf("setup: expected \"1h\", got %q", tr.Selected().Label)
	}
	tr2, changed := tr.Update(keyMsg("left"))
	if changed {
		t.Fatalf("left at first option should report changed=false")
	}
	if tr2.Selected().Label != "1h" {
		t.Fatalf("selection should stay \"1h\", got %q", tr2.Selected().Label)
	}

	// Right at the last option does not change.
	last := NewTimeRange()
	for i := 0; i < 10; i++ {
		last, _ = last.Update(keyMsg("right"))
	}
	if last.Selected().Label != "7d" {
		t.Fatalf("setup: expected \"7d\", got %q", last.Selected().Label)
	}
	_, changed = last.Update(keyMsg("right"))
	if changed {
		t.Fatalf("right at last option should report changed=false")
	}
}

func TestTimeRangeUpdateIgnoresOtherMessages(t *testing.T) {
	if _, changed := NewTimeRange().Update(keyMsg("x")); changed {
		t.Fatalf("unrelated key should not change selection")
	}
	if _, changed := NewTimeRange().Update(tea.WindowSizeMsg{Width: 10, Height: 10}); changed {
		t.Fatalf("non-key message should not change selection")
	}
}

func TestTimeRangeViewHighlightsSelected(t *testing.T) {
	v := NewTimeRange().View()
	for _, label := range []string{"1h", "3h", "12h", "24h", "7d"} {
		if !strings.Contains(v, label) {
			t.Errorf("View() missing pill %q:\n%s", label, v)
		}
	}
}
