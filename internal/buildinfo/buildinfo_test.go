package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	stamped := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.6.0"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.time", Value: "2026-11-13T00:00:00Z"},
		},
	}

	tests := []struct {
		name                          string
		info                          *debug.BuildInfo
		version, commit, date         string
		wantVer, wantCommit, wantDate string
	}{
		{
			name:    "go install fills placeholders from module info",
			info:    &debug.BuildInfo{Main: debug.Module{Version: "v1.6.0"}},
			version: DefaultVersion, commit: DefaultCommit, date: DefaultDate,
			wantVer: "1.6.0", wantCommit: DefaultCommit, wantDate: DefaultDate,
		},
		{
			name:    "vcs stamps fill commit and date",
			info:    stamped,
			version: DefaultVersion, commit: DefaultCommit, date: DefaultDate,
			wantVer: "1.6.0", wantCommit: "abc123", wantDate: "2026-11-13T00:00:00Z",
		},
		{
			name:    "ldflags values win",
			info:    stamped,
			version: "2.0.0", commit: "fff", date: "2027-01-01T00:00:00Z",
			wantVer: "2.0.0", wantCommit: "fff", wantDate: "2027-01-01T00:00:00Z",
		},
		{
			name:    "devel build keeps placeholders",
			info:    &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			version: DefaultVersion, commit: DefaultCommit, date: DefaultDate,
			wantVer: DefaultVersion, wantCommit: DefaultCommit, wantDate: DefaultDate,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, c, d := resolve(tt.info, tt.version, tt.commit, tt.date)
			if v != tt.wantVer || c != tt.wantCommit || d != tt.wantDate {
				t.Errorf("resolve() = %q, %q, %q; want %q, %q, %q", v, c, d, tt.wantVer, tt.wantCommit, tt.wantDate)
			}
		})
	}
}
