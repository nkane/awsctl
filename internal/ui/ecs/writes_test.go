package ecs

import "testing"

func TestFamilyOf(t *testing.T) {
	cases := map[string]string{
		"api:7":      "api",
		"web-worker": "web-worker",
		"a:b:12":     "a:b",
		"":           "",
	}
	for in, want := range cases {
		if got := familyOf(in); got != want {
			t.Fatalf("familyOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNoticeFor(t *testing.T) {
	cases := []struct {
		action, target, want string
	}{
		{actionScale, "api", "scaled api"},
		{actionForceDeploy, "api", "forced new deployment of api"},
		{actionUpdateTaskDef, "api", "updated task definition for api"},
		{actionStopTask, "task-1", "stopped task task-1"},
		{"unknown", "x", "done"},
	}
	for _, c := range cases {
		if got := noticeFor(c.action, c.target); got != c.want {
			t.Fatalf("noticeFor(%q,%q) = %q, want %q", c.action, c.target, got, c.want)
		}
	}
}

// The write-command builders must surface a nil-client guard as a result
// message tagged with the right action/target rather than panicking.
func TestWriteCmdsNilClientGuard(t *testing.T) {
	cases := []struct {
		name   string
		msg    ecsWriteDoneMsg
		action string
		target string
	}{
		{"scale", scaleCmd(nil, "c", "svc", 3)().(ecsWriteDoneMsg), actionScale, "svc"},
		{"forceDeploy", forceDeployCmd(nil, "c", "svc")().(ecsWriteDoneMsg), actionForceDeploy, "svc"},
		{"updateTaskDef", updateTaskDefCmd(nil, "c", "svc", "api:7")().(ecsWriteDoneMsg), actionUpdateTaskDef, "svc"},
		{"stopTask", stopTaskCmd(nil, "c", "task-1", "reason")().(ecsWriteDoneMsg), actionStopTask, "task-1"},
	}
	for _, c := range cases {
		if c.msg.action != c.action {
			t.Errorf("%s: action = %q, want %q", c.name, c.msg.action, c.action)
		}
		if c.msg.target != c.target {
			t.Errorf("%s: target = %q, want %q", c.name, c.msg.target, c.target)
		}
		if c.msg.err == nil {
			t.Errorf("%s: expected nil-client error, got nil", c.name)
		}
	}
}
