package agent

import (
	"testing"
	"time"
)

func TestFilterMatches(t *testing.T) {
	now := time.Now()
	latestVer := MustParseVersion("2.0.0")

	// Create test installations
	claudeNPM := Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		Method:           InstallMethodNPM,
		InstalledVersion: MustParseVersion("1.0.0"),
		LatestVersion:    &latestVer,
		ExecutablePath:   "/usr/local/bin/claude",
		IsGlobal:         true,
		DetectedAt:       now,
	}

	aiderPipx := Installation{
		AgentID:          "aider",
		AgentName:        "Aider",
		Method:           InstallMethodPipx,
		InstalledVersion: MustParseVersion("0.50.0"),
		ExecutablePath:   "/home/user/.local/bin/aider",
		IsGlobal:         false,
		DetectedAt:       now,
	}

	tests := []struct {
		name     string
		filter   Filter
		inst     Installation
		expected bool
	}{
		// AgentID filter
		{
			name:     "match singular agent ID",
			filter:   Filter{AgentID: "claude-code"},
			inst:     claudeNPM,
			expected: true,
		},
		{
			name:     "no match singular agent ID",
			filter:   Filter{AgentID: "aider"},
			inst:     claudeNPM,
			expected: false,
		},

		// AgentIDs filter (plural)
		{
			name:     "match plural agent IDs",
			filter:   Filter{AgentIDs: []string{"claude-code", "aider"}},
			inst:     claudeNPM,
			expected: true,
		},
		{
			name:     "no match plural agent IDs",
			filter:   Filter{AgentIDs: []string{"copilot", "gemini"}},
			inst:     claudeNPM,
			expected: false,
		},

		// Method filter (singular)
		{
			name:     "match singular method",
			filter:   Filter{Method: InstallMethodNPM},
			inst:     claudeNPM,
			expected: true,
		},
		{
			name:     "no match singular method",
			filter:   Filter{Method: InstallMethodPip},
			inst:     claudeNPM,
			expected: false,
		},

		// Methods filter (plural)
		{
			name:     "match plural methods",
			filter:   Filter{Methods: []InstallMethod{InstallMethodNPM, InstallMethodBrew}},
			inst:     claudeNPM,
			expected: true,
		},
		{
			name:     "no match plural methods",
			filter:   Filter{Methods: []InstallMethod{InstallMethodPip, InstallMethodPipx}},
			inst:     claudeNPM,
			expected: false,
		},

		// HasUpdate filter
		{
			name:     "match has update true",
			filter:   Filter{HasUpdate: boolPtr(true)},
			inst:     claudeNPM,
			expected: true,
		},
		{
			name:     "no match has update true",
			filter:   Filter{HasUpdate: boolPtr(true)},
			inst:     aiderPipx,
			expected: false,
		},
		{
			name:     "match has update false",
			filter:   Filter{HasUpdate: boolPtr(false)},
			inst:     aiderPipx,
			expected: true,
		},

		// IsGlobal filter
		{
			name:     "match is global true",
			filter:   Filter{IsGlobal: boolPtr(true)},
			inst:     claudeNPM,
			expected: true,
		},
		{
			name:     "no match is global true",
			filter:   Filter{IsGlobal: boolPtr(true)},
			inst:     aiderPipx,
			expected: false,
		},
		{
			name:     "match is global false",
			filter:   Filter{IsGlobal: boolPtr(false)},
			inst:     aiderPipx,
			expected: true,
		},

		// Query filter
		{
			name:     "match query on agent ID",
			filter:   Filter{Query: "claude"},
			inst:     claudeNPM,
			expected: true,
		},
		{
			name:     "match query on agent name",
			filter:   Filter{Query: "Code"},
			inst:     claudeNPM,
			expected: true,
		},
		{
			name:     "match query case insensitive",
			filter:   Filter{Query: "CLAUDE"},
			inst:     claudeNPM,
			expected: true,
		},
		{
			name:     "no match query",
			filter:   Filter{Query: "copilot"},
			inst:     claudeNPM,
			expected: false,
		},

		// Combined filters
		{
			name: "match all filters",
			filter: Filter{
				AgentID:  "claude-code",
				Method:   InstallMethodNPM,
				IsGlobal: boolPtr(true),
			},
			inst:     claudeNPM,
			expected: true,
		},
		{
			name: "fail one of multiple filters",
			filter: Filter{
				AgentID:  "claude-code",
				Method:   InstallMethodPip, // Wrong method
				IsGlobal: boolPtr(true),
			},
			inst:     claudeNPM,
			expected: false,
		},

		// Empty filter (matches all)
		{
			name:     "empty filter matches all",
			filter:   Filter{},
			inst:     claudeNPM,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.Matches(tt.inst)
			if got != tt.expected {
				t.Errorf("Filter.Matches() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultRegistryOptions(t *testing.T) {
	opts := DefaultRegistryOptions()

	if opts.AutoRefresh != false {
		t.Errorf("AutoRefresh = %v, want false", opts.AutoRefresh)
	}
	if opts.RefreshInterval != 3600 {
		t.Errorf("RefreshInterval = %d, want 3600", opts.RefreshInterval)
	}
	if opts.IncludeHidden != false {
		t.Errorf("IncludeHidden = %v, want false", opts.IncludeHidden)
	}
	if opts.PlatformFilter != nil {
		t.Errorf("PlatformFilter = %v, want nil", opts.PlatformFilter)
	}
}

func TestDefaultListOptions(t *testing.T) {
	opts := DefaultListOptions()

	if opts.Filter != nil {
		t.Errorf("Filter = %v, want nil", opts.Filter)
	}
	if opts.SortBy != SortByName {
		t.Errorf("SortBy = %v, want %v", opts.SortBy, SortByName)
	}
	if opts.SortOrder != SortAsc {
		t.Errorf("SortOrder = %v, want %v", opts.SortOrder, SortAsc)
	}
	if opts.Limit != 0 {
		t.Errorf("Limit = %d, want 0", opts.Limit)
	}
	if opts.Offset != 0 {
		t.Errorf("Offset = %d, want 0", opts.Offset)
	}
}

func TestRegistryEventType(t *testing.T) {
	tests := []struct {
		eventType RegistryEventType
		expected  string
	}{
		{RegistryEventAdded, "added"},
		{RegistryEventUpdated, "updated"},
		{RegistryEventRemoved, "removed"},
		{RegistryEventRefreshed, "refreshed"},
		{RegistryEventError, "error"},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			if string(tt.eventType) != tt.expected {
				t.Errorf("RegistryEventType = %q, want %q", tt.eventType, tt.expected)
			}
		})
	}
}

func TestSortFieldAndOrder(t *testing.T) {
	// Test sort fields
	sortFields := []SortField{SortByName, SortByMethod, SortByVersion, SortByUpdatedAt, SortByStatus}
	expectedFields := []string{"name", "method", "version", "updated_at", "status"}

	for i, sf := range sortFields {
		if string(sf) != expectedFields[i] {
			t.Errorf("SortField = %q, want %q", sf, expectedFields[i])
		}
	}

	// Test sort orders
	if string(SortAsc) != "asc" {
		t.Errorf("SortAsc = %q, want %q", SortAsc, "asc")
	}
	if string(SortDesc) != "desc" {
		t.Errorf("SortDesc = %q, want %q", SortDesc, "desc")
	}
}

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusCurrent, "current"},
		{StatusOutdated, "outdated"},
		{StatusUnknown, "unknown"},
		{StatusError, "error"},
		{StatusInstalling, "installing"},
		{StatusUpdating, "updating"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Status = %q, want %q", tt.status, tt.expected)
			}
		})
	}
}
