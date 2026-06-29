package main

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// Build metadata. Version is set via -ldflags in release builds (goreleaser);
// Commit and Date likewise. For plain `go build` / `go install` builds the
// ldflags are absent, so resolveBuildInfo backfills these from the embedded
// build info (VCS stamp) where available.
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// versionString renders the user-facing `--version` output, backfilling missing
// fields from the embedded build info.
func versionString() string {
	v, c, d := Version, Commit, Date
	goVer := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		v, c, d, goVer = resolveBuildInfo(info, v, c, d)
	}
	return formatVersion(v, c, d, goVer)
}

// resolveBuildInfo fills empty/default fields from the module build info: the
// module version (e.g. a `go install module@v1.2.3` build) and the VCS
// revision/time stamps that `go build` embeds for repo builds.
func resolveBuildInfo(info *debug.BuildInfo, ver, commit, date string) (v, c, d, goVer string) {
	v, c, d = ver, commit, date
	goVer = info.GoVersion

	// Prefer a real module version over the "dev" placeholder.
	if (v == "" || v == "dev") && info.Main.Version != "" && info.Main.Version != "(devel)" {
		v = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if c == "" {
				c = s.Value
			}
		case "vcs.time":
			if d == "" {
				d = s.Value
			}
		case "vcs.modified":
			if s.Value == "true" {
				c = strings.TrimSpace(c+" (dirty)")
			}
		}
	}
	return v, c, d, goVer
}

// formatVersion lays out the version block, omitting unknown fields.
func formatVersion(ver, commit, date, goVer string) string {
	if ver == "" {
		ver = "dev"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "awsctl %s", ver)
	if commit != "" {
		fmt.Fprintf(&b, "\n  commit: %s", commit)
	}
	if date != "" {
		fmt.Fprintf(&b, "\n  built:  %s", date)
	}
	if goVer != "" {
		fmt.Fprintf(&b, "\n  go:     %s", goVer)
	}
	return b.String()
}
