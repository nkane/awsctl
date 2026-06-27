package ui

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func newTestApp() tea.Model {
	app := NewApp(Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	m, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

// TestModeSwitchWiring verifies the global tab keys move between per-mode stacks
// without any AWS calls (screens render their "waiting for config" state).
func TestModeSwitchWiring(t *testing.T) {
	m := newTestApp()
	if m.(App).mode != ModeLambda {
		t.Fatalf("default mode = %v, want ModeLambda", m.(App).mode)
	}

	cases := []struct {
		key  string
		mode Mode
		tab  int
	}{
		{"2", ModeDynamo, 1},
		{"3", ModeEcs, 2},
		{"1", ModeLambda, 0},
	}
	for _, c := range cases {
		m, _ = m.Update(runes(c.key))
		app := m.(App)
		if app.mode != c.mode {
			t.Fatalf("after %q: mode = %v, want %v", c.key, app.mode, c.mode)
		}
		if app.tabs.Active != c.tab {
			t.Fatalf("after %q: tab = %d, want %d", c.key, app.tabs.Active, c.tab)
		}
		if app.active() != app.stackFor(c.mode) {
			t.Fatalf("after %q: active stack mismatch", c.key)
		}
	}
}

// TestHelpToggle verifies '?' opens the full-help overlay and esc closes it,
// and that the short-help footer surfaces the active screen's drill keys.
func TestHelpToggle(t *testing.T) {
	m := newTestApp()

	// Footer should advertise the lambda root's drill keys.
	if v := m.(App).View(); !strings.Contains(v, "detail") || !strings.Contains(v, "invoke") {
		t.Fatalf("short-help footer missing lambda drill keys; view:\n%s", v)
	}

	m, _ = m.Update(runes("?"))
	if !m.(App).showHelp {
		t.Fatal("'?' should open the help overlay")
	}
	if v := m.(App).View(); !strings.Contains(v, "Help") {
		t.Fatalf("help overlay not rendered; view:\n%s", v)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.(App).showHelp {
		t.Fatal("esc should close the help overlay")
	}
}

// TestEcsTabRendered checks the ECS tab is present in the header chrome.
func TestEcsTabRendered(t *testing.T) {
	m := newTestApp()
	m, _ = m.Update(runes("3"))
	if v := m.(App).View(); !strings.Contains(v, "ECS") {
		t.Fatalf("ECS tab not rendered; view:\n%s", v)
	}
}
