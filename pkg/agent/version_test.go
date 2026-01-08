package agent

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Version
		wantErr  bool
	}{
		{
			name:  "simple version",
			input: "1.2.3",
			expected: Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Raw:   "1.2.3",
			},
		},
		{
			name:  "version with v prefix",
			input: "v1.2.3",
			expected: Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Raw:   "v1.2.3",
			},
		},
		{
			name:  "version with prerelease",
			input: "1.2.3-beta.1",
			expected: Version{
				Major:      1,
				Minor:      2,
				Patch:      3,
				Prerelease: "beta.1",
				Raw:        "1.2.3-beta.1",
			},
		},
		{
			name:  "version with build metadata",
			input: "1.2.3+build.123",
			expected: Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Build: "build.123",
				Raw:   "1.2.3+build.123",
			},
		},
		{
			name:  "version with prerelease and build",
			input: "1.2.3-alpha.1+build.456",
			expected: Version{
				Major:      1,
				Minor:      2,
				Patch:      3,
				Prerelease: "alpha.1",
				Build:      "build.456",
				Raw:        "1.2.3-alpha.1+build.456",
			},
		},
		{
			name:  "major only",
			input: "1",
			expected: Version{
				Major: 1,
				Minor: 0,
				Patch: 0,
				Raw:   "1",
			},
		},
		{
			name:  "major.minor only",
			input: "1.2",
			expected: Version{
				Major: 1,
				Minor: 2,
				Patch: 0,
				Raw:   "1.2",
			},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Major != tt.expected.Major {
					t.Errorf("Major = %d, want %d", got.Major, tt.expected.Major)
				}
				if got.Minor != tt.expected.Minor {
					t.Errorf("Minor = %d, want %d", got.Minor, tt.expected.Minor)
				}
				if got.Patch != tt.expected.Patch {
					t.Errorf("Patch = %d, want %d", got.Patch, tt.expected.Patch)
				}
				if got.Prerelease != tt.expected.Prerelease {
					t.Errorf("Prerelease = %q, want %q", got.Prerelease, tt.expected.Prerelease)
				}
				if got.Build != tt.expected.Build {
					t.Errorf("Build = %q, want %q", got.Build, tt.expected.Build)
				}
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"equal versions", "1.2.3", "1.2.3", 0},
		{"major difference", "2.0.0", "1.0.0", 1},
		{"minor difference", "1.2.0", "1.1.0", 1},
		{"patch difference", "1.2.3", "1.2.2", 1},
		{"reverse major", "1.0.0", "2.0.0", -1},
		{"reverse minor", "1.1.0", "1.2.0", -1},
		{"reverse patch", "1.2.2", "1.2.3", -1},
		{"prerelease vs release", "1.0.0-alpha", "1.0.0", -1},
		{"release vs prerelease", "1.0.0", "1.0.0-alpha", 1},
		{"alpha vs beta", "1.0.0-alpha", "1.0.0-beta", -1},
		{"beta vs alpha", "1.0.0-beta", "1.0.0-alpha", 1},
		{"numeric prerelease comparison", "1.0.0-1", "1.0.0-2", -1},
		{"prerelease part count", "1.0.0-alpha.1", "1.0.0-alpha", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustParseVersion(tt.v1)
			v2 := MustParseVersion(tt.v2)
			got := v1.Compare(v2)
			if got != tt.expected {
				t.Errorf("Version(%q).Compare(%q) = %d, want %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestVersionIsNewerThan(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{"newer major", "2.0.0", "1.0.0", true},
		{"newer minor", "1.2.0", "1.1.0", true},
		{"newer patch", "1.2.3", "1.2.2", true},
		{"equal", "1.2.3", "1.2.3", false},
		{"older", "1.0.0", "2.0.0", false},
		{"release newer than prerelease", "1.0.0", "1.0.0-alpha", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustParseVersion(tt.v1)
			v2 := MustParseVersion(tt.v2)
			got := v1.IsNewerThan(v2)
			if got != tt.expected {
				t.Errorf("Version(%q).IsNewerThan(%q) = %v, want %v", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestVersionIsOlderThan(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{"older major", "1.0.0", "2.0.0", true},
		{"older minor", "1.1.0", "1.2.0", true},
		{"older patch", "1.2.2", "1.2.3", true},
		{"equal", "1.2.3", "1.2.3", false},
		{"newer", "2.0.0", "1.0.0", false},
		{"prerelease older than release", "1.0.0-alpha", "1.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustParseVersion(tt.v1)
			v2 := MustParseVersion(tt.v2)
			got := v1.IsOlderThan(v2)
			if got != tt.expected {
				t.Errorf("Version(%q).IsOlderThan(%q) = %v, want %v", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestVersionEquals(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{"equal simple", "1.2.3", "1.2.3", true},
		{"equal with prerelease", "1.2.3-alpha", "1.2.3-alpha", true},
		{"different major", "1.2.3", "2.2.3", false},
		{"different minor", "1.2.3", "1.3.3", false},
		{"different patch", "1.2.3", "1.2.4", false},
		{"different prerelease", "1.2.3-alpha", "1.2.3-beta", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustParseVersion(tt.v1)
			v2 := MustParseVersion(tt.v2)
			got := v1.Equals(v2)
			if got != tt.expected {
				t.Errorf("Version(%q).Equals(%q) = %v, want %v", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple version", "1.2.3", "1.2.3"},
		{"version with v prefix", "v1.2.3", "v1.2.3"},
		{"version with prerelease", "1.2.3-beta.1", "1.2.3-beta.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := MustParseVersion(tt.input)
			got := v.String()
			if got != tt.expected {
				t.Errorf("Version.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestVersionIsZero(t *testing.T) {
	tests := []struct {
		name     string
		version  Version
		expected bool
	}{
		{"zero version", Version{}, true},
		{"non-zero major", Version{Major: 1}, false},
		{"non-zero minor", Version{Minor: 1}, false},
		{"non-zero patch", Version{Patch: 1}, false},
		{"raw version only", Version{Raw: "unknown"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.version.IsZero()
			if got != tt.expected {
				t.Errorf("Version.IsZero() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVersionRangeContains(t *testing.T) {
	tests := []struct {
		name     string
		from     string
		to       string
		check    string
		expected bool
	}{
		{"within range", "1.0.0", "2.0.0", "1.5.0", true},
		{"at lower bound", "1.0.0", "2.0.0", "1.0.0", true},
		{"at upper bound", "1.0.0", "2.0.0", "2.0.0", true},
		{"below range", "1.0.0", "2.0.0", "0.5.0", false},
		{"above range", "1.0.0", "2.0.0", "2.5.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := VersionRange{
				From: MustParseVersion(tt.from),
				To:   MustParseVersion(tt.to),
			}
			v := MustParseVersion(tt.check)
			got := r.Contains(v)
			if got != tt.expected {
				t.Errorf("VersionRange.Contains(%q) = %v, want %v", tt.check, got, tt.expected)
			}
		})
	}
}

func TestVersionConstraintMatches(t *testing.T) {
	tests := []struct {
		name       string
		operator   string
		constraint string
		version    string
		expected   bool
	}{
		// Equality
		{"equal match", "=", "1.2.3", "1.2.3", true},
		{"equal no match", "=", "1.2.3", "1.2.4", false},

		// Greater than
		{"greater match", ">", "1.0.0", "1.0.1", true},
		{"greater no match", ">", "1.0.0", "1.0.0", false},
		{"greater no match lower", ">", "1.0.0", "0.9.9", false},

		// Greater than or equal
		{"gte match greater", ">=", "1.0.0", "1.0.1", true},
		{"gte match equal", ">=", "1.0.0", "1.0.0", true},
		{"gte no match", ">=", "1.0.0", "0.9.9", false},

		// Less than
		{"less match", "<", "1.0.0", "0.9.9", true},
		{"less no match", "<", "1.0.0", "1.0.0", false},
		{"less no match higher", "<", "1.0.0", "1.0.1", false},

		// Less than or equal
		{"lte match less", "<=", "1.0.0", "0.9.9", true},
		{"lte match equal", "<=", "1.0.0", "1.0.0", true},
		{"lte no match", "<=", "1.0.0", "1.0.1", false},

		// Tilde (patch level)
		{"tilde match same", "~", "1.2.3", "1.2.3", true},
		{"tilde match higher patch", "~", "1.2.3", "1.2.5", true},
		{"tilde no match minor", "~", "1.2.3", "1.3.0", false},
		{"tilde no match lower patch", "~", "1.2.3", "1.2.2", false},

		// Caret (minor level for non-zero major)
		{"caret match same", "^", "1.2.3", "1.2.3", true},
		{"caret match higher minor", "^", "1.2.3", "1.3.0", true},
		{"caret match higher patch", "^", "1.2.3", "1.2.5", true},
		{"caret no match major", "^", "1.2.3", "2.0.0", false},
		{"caret no match lower", "^", "1.2.3", "1.2.2", false},

		// Caret with zero major
		{"caret zero major match", "^", "0.2.3", "0.2.5", true},
		{"caret zero major no match minor", "^", "0.2.3", "0.3.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := VersionConstraint{
				Operator: tt.operator,
				Version:  MustParseVersion(tt.constraint),
			}
			v := MustParseVersion(tt.version)
			got := c.Matches(v)
			if got != tt.expected {
				t.Errorf("VersionConstraint(%s%s).Matches(%q) = %v, want %v",
					tt.operator, tt.constraint, tt.version, got, tt.expected)
			}
		})
	}
}
