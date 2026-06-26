package ui

import (
	"strings"
	"testing"

	awsx "github.com/nkane/awsctl/internal/aws"
)

func joined(args []string) string { return strings.Join(args, " ") }

func TestEcsExecArgs(t *testing.T) {
	got := joined(ecsExecArgs(&awsx.Config{Region: "us-east-1", Profile: "dev"}, "c1", "t1", "web"))
	for _, want := range []string{
		"ecs execute-command",
		"--cluster c1",
		"--task t1",
		"--container web",
		"--interactive",
		"--command /bin/sh",
		"--region us-east-1",
		"--profile dev",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("args missing %q; got: %s", want, got)
		}
	}
}

func TestEcsExecArgsOmitsEmptyRegionProfile(t *testing.T) {
	got := joined(ecsExecArgs(&awsx.Config{}, "c1", "t1", "web"))
	if strings.Contains(got, "--region") || strings.Contains(got, "--profile") {
		t.Fatalf("empty region/profile should be omitted; got: %s", got)
	}
	got = joined(ecsExecArgs(nil, "c1", "t1", "web"))
	if !strings.Contains(got, "--cluster c1") {
		t.Fatalf("nil cfg should still build target args; got: %s", got)
	}
}
