package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func mkKey(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestConfirmActiveLifecycle(t *testing.T) {
	c := NewConfirm(ConfirmTheme{})
	if c.Active() {
		t.Fatal("new confirm should not be active")
	}
	c = c.Show("Delete?", "really?")
	if !c.Active() {
		t.Fatal("confirm should be active after Show")
	}
	c = c.Hide()
	if c.Active() {
		t.Fatal("confirm should be inactive after Hide")
	}
}

func TestConfirmYesKey(t *testing.T) {
	c := NewConfirm(ConfirmTheme{}).Show("t", "b")
	c, res := c.Update(mkKey("y"))
	if res != ConfirmYes {
		t.Fatalf("want ConfirmYes, got %v", res)
	}
	if c.Active() {
		t.Fatal("confirm should auto-hide after yes")
	}
}

func TestConfirmNoKeys(t *testing.T) {
	for _, k := range []string{"n", "esc"} {
		c := NewConfirm(ConfirmTheme{}).Show("t", "b")
		c, res := c.Update(mkKey(k))
		if res != ConfirmNo {
			t.Fatalf("key %q: want ConfirmNo, got %v", k, res)
		}
		if c.Active() {
			t.Fatalf("key %q: confirm should auto-hide after no", k)
		}
	}
}

func TestConfirmEnterDefaultsToSafe(t *testing.T) {
	c := NewConfirm(ConfirmTheme{}).Show("t", "b")
	c, res := c.Update(mkKey("enter"))
	if res != ConfirmNo {
		t.Fatalf("Enter at default cursor: want ConfirmNo (safe), got %v", res)
	}
	if c.Active() {
		t.Fatal("confirm should auto-hide after enter")
	}
}

func TestConfirmRightThenEnterConfirms(t *testing.T) {
	c := NewConfirm(ConfirmTheme{}).Show("t", "b")
	c, res := c.Update(mkKey("right"))
	if res != ConfirmNone {
		t.Fatalf("moving cursor: want ConfirmNone, got %v", res)
	}
	if !c.Active() {
		t.Fatal("confirm should still be active after cursor move")
	}
	c, res = c.Update(mkKey("enter"))
	if res != ConfirmYes {
		t.Fatalf("right-then-enter: want ConfirmYes, got %v", res)
	}
}

func TestConfirmTabTogglesCursor(t *testing.T) {
	c := NewConfirm(ConfirmTheme{}).Show("t", "b")
	c, _ = c.Update(mkKey("tab")) // -> Confirm
	c, res := c.Update(mkKey("enter"))
	if res != ConfirmYes {
		t.Fatalf("tab-then-enter: want ConfirmYes, got %v", res)
	}
}

func TestConfirmUnknownKeyIsNoop(t *testing.T) {
	c := NewConfirm(ConfirmTheme{}).Show("t", "b")
	c2, res := c.Update(mkKey("x"))
	if res != ConfirmNone {
		t.Fatalf("unknown key: want ConfirmNone, got %v", res)
	}
	if !c2.Active() {
		t.Fatal("unknown key should leave confirm active")
	}
}

func TestConfirmView(t *testing.T) {
	c := NewConfirm(ConfirmTheme{})
	if c.View(80, 24) != "" {
		t.Fatal("inactive View should be empty")
	}
	c = c.Show("Delete bucket?", "This cannot be undone.")
	out := c.View(80, 24)
	if !strings.Contains(out, "Delete bucket?") {
		t.Error("View should contain the title")
	}
	if !strings.Contains(out, "Cancel") {
		t.Error("View should contain the Cancel button label")
	}
	if !strings.Contains(out, "Confirm") {
		t.Error("View should contain the Confirm button label")
	}
}
