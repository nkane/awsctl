package ecs

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func strptr(s string) *string { return &s }

// modelWithEdit builds a service-describe model parked in an inline-edit mode
// with the field pre-seeded to value.
func modelWithEdit(mode editMode, value string, svc *ecstypes.Service) ServiceDescribeModel {
	in := textinput.New()
	in.SetValue(value)
	return ServiceDescribeModel{
		cluster: "c",
		name:    "svc",
		svc:     svc,
		editing: mode,
		input:   in,
	}
}

func TestSubmitEditScale(t *testing.T) {
	t.Run("valid emits command and exits edit", func(t *testing.T) {
		m := modelWithEdit(editScale, "5", &ecstypes.Service{DesiredCount: 1})
		nm, cmd := m.submitEdit()
		if nm.editing != editNone {
			t.Fatalf("editing = %v, want editNone", nm.editing)
		}
		if cmd == nil {
			t.Fatal("expected a ConfirmRequest command, got nil")
		}
		if nm.err != "" {
			t.Fatalf("unexpected err: %q", nm.err)
		}
	})

	for _, bad := range []string{"", "-1", "abc", "3.5"} {
		t.Run("invalid "+bad, func(t *testing.T) {
			m := modelWithEdit(editScale, bad, &ecstypes.Service{DesiredCount: 1})
			nm, cmd := m.submitEdit()
			if cmd != nil {
				t.Fatalf("expected no command for %q", bad)
			}
			if nm.editing != editScale {
				t.Fatalf("editing left %v, want still editScale", nm.editing)
			}
			if nm.err == "" {
				t.Fatalf("expected validation error for %q", bad)
			}
		})
	}
}

func TestSubmitEditTaskDef(t *testing.T) {
	svc := &ecstypes.Service{
		TaskDefinition: strptr("arn:aws:ecs:us-east-1:0:task-definition/api:7"),
	}
	t.Run("valid revision points at family:revision", func(t *testing.T) {
		m := modelWithEdit(editTaskDef, "9", svc)
		nm, cmd := m.submitEdit()
		if nm.editing != editNone {
			t.Fatalf("editing = %v, want editNone", nm.editing)
		}
		if cmd == nil {
			t.Fatal("expected a ConfirmRequest command, got nil")
		}
		if fam := m.currentFamily(); fam != "api" {
			t.Fatalf("currentFamily = %q, want api", fam)
		}
		if rev := m.currentRevision(); rev != "7" {
			t.Fatalf("currentRevision = %q, want 7", rev)
		}
	})

	for _, bad := range []string{"", "0", "-2", "latest"} {
		t.Run("invalid "+bad, func(t *testing.T) {
			m := modelWithEdit(editTaskDef, bad, svc)
			nm, cmd := m.submitEdit()
			if cmd != nil {
				t.Fatalf("expected no command for %q", bad)
			}
			if nm.err == "" {
				t.Fatalf("expected validation error for %q", bad)
			}
		})
	}

	t.Run("missing family errors", func(t *testing.T) {
		m := modelWithEdit(editTaskDef, "3", &ecstypes.Service{})
		nm, cmd := m.submitEdit()
		if cmd != nil {
			t.Fatal("expected no command when family unknown")
		}
		if nm.err == "" {
			t.Fatal("expected error when family unknown")
		}
	})
}
