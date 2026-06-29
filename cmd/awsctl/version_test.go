package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestFormatVersion(t *testing.T) {
	out := formatVersion("v1.2.3", "abc1234", "2026-06-29T00:00:00Z", "go1.24.2")
	for _, want := range []string{"awsctl v1.2.3", "commit: abc1234", "built:  2026-06-29T00:00:00Z", "go:     go1.24.2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("version output missing %q; got:\n%s", want, out)
		}
	}
}

func TestFormatVersionOmitsEmpty(t *testing.T) {
	out := formatVersion("dev", "", "", "")
	if out != "awsctl dev" {
		t.Fatalf("bare build should print just the version, got %q", out)
	}
}

func TestFormatVersionDefaultsEmpty(t *testing.T) {
	if out := formatVersion("", "", "", ""); out != "awsctl dev" {
		t.Fatalf("empty version should default to dev, got %q", out)
	}
}

func TestResolveBuildInfoBackfills(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "go1.24.2",
		Main:      debug.Module{Version: "v0.9.0"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "deadbeef"},
			{Key: "vcs.time", Value: "2026-06-29T12:00:00Z"},
		},
	}
	// "dev" placeholder + empty commit/date → backfilled from build info.
	v, c, d, goVer := resolveBuildInfo(info, "dev", "", "")
	if v != "v0.9.0" {
		t.Fatalf("version = %q, want v0.9.0 (from module)", v)
	}
	if c != "deadbeef" {
		t.Fatalf("commit = %q, want deadbeef (from vcs.revision)", c)
	}
	if d != "2026-06-29T12:00:00Z" {
		t.Fatalf("date = %q, want vcs.time", d)
	}
	if goVer != "go1.24.2" {
		t.Fatalf("goVer = %q, want go1.24.2", goVer)
	}
}

func TestResolveBuildInfoKeepsLdflags(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "go1.24.2",
		Main:      debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "fromvcs"},
		},
	}
	// Explicit ldflags values win over build-info backfill.
	v, c, _, _ := resolveBuildInfo(info, "v2.0.0", "ldflagcommit", "2026-01-01")
	if v != "v2.0.0" {
		t.Fatalf("version = %q, want ldflags v2.0.0", v)
	}
	if c != "ldflagcommit" {
		t.Fatalf("commit = %q, want ldflags value", c)
	}
}

func TestResolveBuildInfoMarksDirty(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "go1.24.2",
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "cafe"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	_, c, _, _ := resolveBuildInfo(info, "dev", "", "")
	if !strings.Contains(c, "cafe") || !strings.Contains(c, "dirty") {
		t.Fatalf("commit = %q, want cafe + dirty marker", c)
	}
}
