package ecs

import (
	"strings"
	"testing"

	awsx "github.com/nkane/awsctl/internal/aws"
)

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
