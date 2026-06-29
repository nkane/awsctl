package ecs

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func dKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")} }

func TestTaskDefRevisionsRenders(t *testing.T) {
	m := NewTaskDefRevisions(awsx.NewEcsClient(&awsx.Config{}), "api")
	m.SetSize(120, 30)

	m, _ = m.Update(revisionsLoadedMsg{family: "api", revisions: []awsx.TaskDefRevision{
		{Revision: "7", Arn: "arn:aws:ecs:us-east-1:0:task-definition/api:7"},
		{Revision: "6", Arn: "arn:aws:ecs:us-east-1:0:task-definition/api:6"},
	}})

	v := m.View()
	for _, want := range []string{"revision 7", "revision 6", "ACTIVE"} {
		if !strings.Contains(v, want) {
			t.Fatalf("revisions view missing %q; got:\n%s", want, v)
		}
	}
	if got := m.Selected(); got != "7" {
		t.Fatalf("Selected() = %q, want 7", got)
	}
}

func TestTaskDefRevisionsIgnoresStaleLoad(t *testing.T) {
	m := NewTaskDefRevisions(awsx.NewEcsClient(&awsx.Config{}), "api")
	m.SetSize(120, 30)
	m, _ = m.Update(revisionsLoadedMsg{family: "other", revisions: []awsx.TaskDefRevision{{Revision: "1"}}})
	if got := m.Selected(); got != "" {
		t.Fatalf("stale load leaked; Selected() = %q, want empty", got)
	}
}

// TestRevisionDrills verifies describe -> revisions (#51 -> #52) and
// revisions -> describe of the exact revision.
func TestRevisionDrills(t *testing.T) {
	// describe 'v' -> revisions
	sd := &taskDefDescribeScreen{m: NewTaskDefDescribe(awsx.NewEcsClient(&awsx.Config{}), "api:7")}
	rev := sd.OpenRevisions(&awsx.Config{})
	if rev == nil || rev.Title() != "revisions" {
		t.Fatalf("OpenRevisions should yield a 'revisions' screen, got %v", rev)
	}

	// revisions enter -> describe of family:revision
	rl := &taskDefRevisionsScreen{m: NewTaskDefRevisions(awsx.NewEcsClient(&awsx.Config{}), "api")}
	rl.m, _ = rl.m.Update(revisionsLoadedMsg{family: "api", revisions: []awsx.TaskDefRevision{{Revision: "6"}}})
	desc := rl.OpenRevision(&awsx.Config{})
	if desc == nil || desc.Title() != "describe" {
		t.Fatalf("OpenRevision should yield a 'describe' screen, got %v", desc)
	}

	empty := &taskDefRevisionsScreen{m: NewTaskDefRevisions(awsx.NewEcsClient(&awsx.Config{}), "api")}
	if got := empty.OpenRevision(&awsx.Config{}); got != nil {
		t.Fatalf("OpenRevision with no selection should be nil, got %v", got)
	}
}

// TestRevisionDiffGesture drives the two-press 'd' diff selection: first press
// marks a base, a press on the same revision clears it, and a press on a
// different revision opens the loading diff overlay (#53).
func TestRevisionDiffGesture(t *testing.T) {
	m := NewTaskDefRevisions(awsx.NewEcsClient(&awsx.Config{}), "api")
	m.SetSize(120, 30)
	m, _ = m.Update(revisionsLoadedMsg{family: "api", revisions: []awsx.TaskDefRevision{
		{Revision: "7"}, {Revision: "6"}, {Revision: "5"},
	}})

	// First 'd' marks revision 7 as the diff base.
	m, _ = m.Update(dKey())
	if m.Diffing() {
		t.Fatal("first 'd' should mark a base, not open the diff")
	}
	if !strings.Contains(m.View(), "diff base: 7") {
		t.Fatalf("expected base marker in title; got:\n%s", m.View())
	}

	// 'd' on the same (still-highlighted) revision clears the base.
	m, _ = m.Update(dKey())
	if strings.Contains(m.View(), "diff base") {
		t.Fatalf("second 'd' on same revision should clear the base; got:\n%s", m.View())
	}

	// Re-mark 7, move to 6, then 'd' opens the diff overlay (loading).
	m, _ = m.Update(dKey())                          // mark 7
	m.list.CursorDown()                              // highlight 6
	var cmd tea.Cmd
	m, cmd = m.Update(dKey())                        // diff 7 -> 6
	if !m.Diffing() {
		t.Fatal("'d' on a different revision should open the diff overlay")
	}
	if cmd == nil {
		t.Fatal("opening the diff should return a load command")
	}
	if !strings.Contains(m.View(), "computing diff") {
		t.Fatalf("diff overlay should show a loading state; got:\n%s", m.View())
	}

	// A completed diff renders gutters + a +/- summary.
	m, _ = m.Update(diffLoadedMsg{family: "api", lines: []diffLine{
		{diffEqual, "{"}, {diffDel, `  "Cpu": "256"`}, {diffAdd, `  "Cpu": "512"`},
	}})
	v := m.View()
	for _, want := range []string{"- ", "+ ", "(+1 -1)"} {
		if !strings.Contains(v, want) {
			t.Fatalf("rendered diff missing %q; got:\n%s", want, v)
		}
	}

	// esc closes the overlay back to the list.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Diffing() {
		t.Fatal("esc should close the diff overlay")
	}
}

// TestRevisionDiffSuppressesDrill verifies the enter→describe drill is gated
// while the diff overlay owns the screen.
func TestRevisionDiffSuppressesDrill(t *testing.T) {
	s := &taskDefRevisionsScreen{m: NewTaskDefRevisions(awsx.NewEcsClient(&awsx.Config{}), "api")}
	s.m.SetSize(120, 30)
	s.m, _ = s.m.Update(revisionsLoadedMsg{family: "api", revisions: []awsx.TaskDefRevision{
		{Revision: "7"}, {Revision: "6"},
	}})
	s.m, _ = s.m.Update(dKey()) // mark 7
	s.m.list.CursorDown()       // highlight 6
	s.m, _ = s.m.Update(dKey()) // open diff overlay

	if !s.CapturesInput() || !s.WantsEsc() {
		t.Fatal("diff overlay should capture input and want esc")
	}
	if got := s.OpenRevision(&awsx.Config{}); got != nil {
		t.Fatalf("OpenRevision must be suppressed during diff, got %v", got)
	}
}

// TestTaskDefDescribeFamilyStripsRevision verifies Family() drops a :rev suffix
// so opening revisions from a specific-revision describe uses the family.
func TestTaskDefDescribeFamilyStripsRevision(t *testing.T) {
	m := NewTaskDefDescribe(awsx.NewEcsClient(&awsx.Config{}), "api:7")
	if got := m.Family(); got != "api" {
		t.Fatalf("Family() = %q, want api", got)
	}
	m2 := NewTaskDefDescribe(awsx.NewEcsClient(&awsx.Config{}), "api")
	if got := m2.Family(); got != "api" {
		t.Fatalf("Family() = %q, want api", got)
	}
}
