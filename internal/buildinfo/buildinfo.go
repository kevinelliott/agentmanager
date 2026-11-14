// Package buildinfo resolves the version metadata reported by the binaries.
//
// Release builds stamp version, commit, and date through -ldflags. Binaries
// built with `go install module/cmd/agentmgr@vX.Y.Z` have no ldflags, so the
// placeholders would survive and `agentmgr version` would report "dev" — which
// also makes `agentmgr upgrade` refuse to compare versions. In that case the
// module version and VCS stamps the Go toolchain embeds are used instead.
package buildinfo

import (
	"runtime/debug"
	"strings"
)

// Placeholder values the main packages carry when no ldflags were supplied.
const (
	DefaultVersion = "dev"
	DefaultCommit  = "none"
	DefaultDate    = "unknown"
)

// Resolve returns version, commit, and date, replacing any value still at its
// placeholder with what the Go toolchain recorded in the binary. Values that
// were stamped via ldflags are returned untouched.
func Resolve(version, commit, date string) (string, string, string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version, commit, date
	}
	return resolve(info, version, commit, date)
}

func resolve(info *debug.BuildInfo, version, commit, date string) (string, string, string) {
	if version == "" || version == DefaultVersion {
		// "(devel)" means the toolchain had no module version to record
		// (for example a build with -buildvcs=false); keep the placeholder.
		// A plain `go build` in a checkout reports a pseudo-version instead.
		if v := info.Main.Version; v != "" && v != "(devel)" {
			version = strings.TrimPrefix(v, "v")
		}
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "" || commit == DefaultCommit {
				commit = s.Value
			}
		case "vcs.time":
			if date == "" || date == DefaultDate {
				date = s.Value
			}
		}
	}
	return version, commit, date
}
