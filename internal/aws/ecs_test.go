package aws

import "testing"

func TestShortTaskDef(t *testing.T) {
	cases := map[string]string{
		"arn:aws:ecs:us-east-1:0:task-definition/api:7": "api:7",
		"api:7": "api:7",
		"":      "",
	}
	for in, want := range cases {
		if got := shortTaskDef(in); got != want {
			t.Fatalf("shortTaskDef(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTaskDefArn(t *testing.T) {
	cases := []struct {
		arn, family, rev string
	}{
		{"arn:aws:ecs:us-east-1:0:task-definition/api:7", "api", "7"},
		{"worker:12", "worker", "12"},
		{"noRevision", "noRevision", ""},
	}
	for _, c := range cases {
		family, rev := parseTaskDefArn(c.arn)
		if family != c.family || rev != c.rev {
			t.Fatalf("parseTaskDefArn(%q) = (%q,%q), want (%q,%q)", c.arn, family, rev, c.family, c.rev)
		}
	}
}
