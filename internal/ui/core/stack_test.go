package core

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// fakeScreen is a minimal Screen for exercising the Stack.
type fakeScreen struct {
	title   string
	resized bool
	msgs    int // count of messages seen via Update
}

func (s *fakeScreen) Init() tea.Cmd { return nil }
func (s *fakeScreen) Update(tea.Msg) (Screen, tea.Cmd) {
	s.msgs++
	return s, nil
}
func (s *fakeScreen) View() string            { return s.title }
func (s *fakeScreen) Title() string           { return s.title }
func (s *fakeScreen) KeyHints() []key.Binding { return nil }
func (s *fakeScreen) SetSize(int, int)        { s.resized = true }

func TestStackPushPop(t *testing.T) {
	root := &fakeScreen{title: "root"}
	st := NewStack(root)

	if !st.AtRoot() || st.Depth() != 1 {
		t.Fatalf("new stack should be at root with depth 1, got depth %d", st.Depth())
	}

	child := &fakeScreen{title: "child"}
	st.Push(child)
	if st.Depth() != 2 || st.Top() != child {
		t.Fatalf("after push: depth=%d top=%v", st.Depth(), st.Top().Title())
	}

	if popped := st.Pop(); popped != child {
		t.Fatalf("pop should return child, got %v", popped)
	}
	if st.Top() != root || !st.AtRoot() {
		t.Fatalf("after pop should be back at root")
	}

	// Root is never popped.
	if popped := st.Pop(); popped != nil || st.Depth() != 1 {
		t.Fatalf("popping root must be a no-op returning nil, got %v depth %d", popped, st.Depth())
	}
}

func TestStackCrumbs(t *testing.T) {
	st := NewStack(&fakeScreen{title: "tables"})
	st.Push(&fakeScreen{title: "describe"})
	st.Push(&fakeScreen{title: "scan"})

	got := st.Crumbs()
	want := []string{"tables", "describe", "scan"}
	if len(got) != len(want) {
		t.Fatalf("crumbs len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("crumb[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStackBroadcastAndSetSize(t *testing.T) {
	root := &fakeScreen{title: "root"}
	child := &fakeScreen{title: "child"}
	st := NewStack(root)
	st.Push(child)

	st.SetSize(80, 24)
	if !root.resized || !child.resized {
		t.Fatalf("SetSize should reach every screen: root=%v child=%v", root.resized, child.resized)
	}

	st.Broadcast(struct{}{})
	if root.msgs != 1 || child.msgs != 1 {
		t.Fatalf("Broadcast should reach every screen once: root=%d child=%d", root.msgs, child.msgs)
	}
}
