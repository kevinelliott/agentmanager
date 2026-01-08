package agent

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version.
type Version struct {
	Major      int    `json:"major"`
	Minor      int    `json:"minor"`
	Patch      int    `json:"patch"`
	Prerelease string `json:"prerelease,omitempty"`
	Build      string `json:"build,omitempty"`
	Raw        string `json:"raw"`
}

// semverRegex matches semantic versions with optional v prefix.
var semverRegex = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([0-9A-Za-z\-\.]+))?(?:\+([0-9A-Za-z\-\.]+))?$`)

// ParseVersion parses a version string into a Version struct.
// Handles formats: v1.2.3, 1.2.3, 1.2.3-beta.1, 1.2.3+build.123
func ParseVersion(s string) (Version, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Version{}, fmt.Errorf("empty version string")
	}

	matches := semverRegex.FindStringSubmatch(s)
	if matches == nil {
		// Try to extract just a version number from the string
		numRegex := regexp.MustCompile(`(\d+(?:\.\d+)*(?:-[a-zA-Z0-9\.\-]+)?)`)
		numMatch := numRegex.FindString(s)
		if numMatch != "" {
			matches = semverRegex.FindStringSubmatch(numMatch)
		}
		if matches == nil {
			return Version{Raw: s}, nil // Return raw version if parsing fails
		}
	}

	v := Version{Raw: s}

	// Parse major version (required)
	if matches[1] != "" {
		major, err := strconv.Atoi(matches[1])
		if err != nil {
			return Version{Raw: s}, nil
		}
		v.Major = major
	}

	// Parse minor version (optional)
	if matches[2] != "" {
		minor, err := strconv.Atoi(matches[2])
		if err == nil {
			v.Minor = minor
		}
	}

	// Parse patch version (optional)
	if matches[3] != "" {
		patch, err := strconv.Atoi(matches[3])
		if err == nil {
			v.Patch = patch
		}
	}

	// Prerelease and build metadata
	v.Prerelease = matches[4]
	v.Build = matches[5]

	return v, nil
}

// MustParseVersion parses a version string and panics on error.
func MustParseVersion(s string) Version {
	v, err := ParseVersion(s)
	if err != nil {
		panic(err)
	}
	return v
}

// String returns the string representation of the version.
func (v Version) String() string {
	if v.Raw != "" {
		return v.Raw
	}
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// IsZero returns true if this is a zero/empty version.
func (v Version) IsZero() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0 && v.Raw == ""
}

// Compare returns -1, 0, or 1 for less than, equal, or greater than.
func (v Version) Compare(other Version) int {
	// Compare major version
	if v.Major != other.Major {
		return compareInt(v.Major, other.Major)
	}

	// Compare minor version
	if v.Minor != other.Minor {
		return compareInt(v.Minor, other.Minor)
	}

	// Compare patch version
	if v.Patch != other.Patch {
		return compareInt(v.Patch, other.Patch)
	}

	// Compare prerelease
	// A version without prerelease is greater than one with prerelease
	// e.g., 1.0.0 > 1.0.0-alpha
	return comparePrerelease(v.Prerelease, other.Prerelease)
}

// IsNewerThan returns true if this version is newer than the other.
func (v Version) IsNewerThan(other Version) bool {
	return v.Compare(other) > 0
}

// IsOlderThan returns true if this version is older than the other.
func (v Version) IsOlderThan(other Version) bool {
	return v.Compare(other) < 0
}

// Equals returns true if the versions are equal.
func (v Version) Equals(other Version) bool {
	return v.Compare(other) == 0
}

// compareInt compares two integers and returns -1, 0, or 1.
func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// comparePrerelease compares prerelease strings.
// An empty prerelease is considered greater than a non-empty one.
func comparePrerelease(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1 // No prerelease > with prerelease
	}
	if b == "" {
		return -1 // With prerelease < no prerelease
	}

	// Compare prerelease identifiers
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		cmp := comparePrereleaseIdentifier(aParts[i], bParts[i])
		if cmp != 0 {
			return cmp
		}
	}

	// If all compared parts are equal, the one with more parts is greater
	return compareInt(len(aParts), len(bParts))
}

// comparePrereleaseIdentifier compares individual prerelease identifiers.
// Numeric identifiers are compared as integers, others lexicographically.
func comparePrereleaseIdentifier(a, b string) int {
	aNum, aErr := strconv.Atoi(a)
	bNum, bErr := strconv.Atoi(b)

	// Both numeric
	if aErr == nil && bErr == nil {
		return compareInt(aNum, bNum)
	}

	// Numeric identifiers have lower precedence than non-numeric
	if aErr == nil {
		return -1
	}
	if bErr == nil {
		return 1
	}

	// Both non-numeric, compare lexicographically
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// VersionRange represents a range of versions between two endpoints.
type VersionRange struct {
	From Version `json:"from"`
	To   Version `json:"to"`
}

// Contains returns true if the given version is within this range.
func (r VersionRange) Contains(v Version) bool {
	return !v.IsOlderThan(r.From) && !v.IsNewerThan(r.To)
}

// VersionConstraint represents a version constraint (e.g., >=1.0.0).
type VersionConstraint struct {
	Operator string  `json:"operator"` // =, >, >=, <, <=, ~, ^
	Version  Version `json:"version"`
}

// Matches returns true if the given version matches this constraint.
func (c VersionConstraint) Matches(v Version) bool {
	cmp := v.Compare(c.Version)

	switch c.Operator {
	case "=", "==":
		return cmp == 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	case "~": // Tilde: allows patch-level changes
		return v.Major == c.Version.Major &&
			v.Minor == c.Version.Minor &&
			v.Patch >= c.Version.Patch
	case "^": // Caret: allows minor and patch-level changes
		if c.Version.Major == 0 {
			return v.Major == 0 && v.Minor == c.Version.Minor && v.Patch >= c.Version.Patch
		}
		return v.Major == c.Version.Major && !v.IsOlderThan(c.Version)
	default:
		return cmp == 0
	}
}
