package storage

import (
	"testing"
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
)

func TestUpdateStatusConstants(t *testing.T) {
	tests := []struct {
		status   UpdateStatus
		expected string
	}{
		{UpdateStatusPending, "pending"},
		{UpdateStatusRunning, "running"},
		{UpdateStatusCompleted, "completed"},
		{UpdateStatusFailed, "failed"},
		{UpdateStatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("UpdateStatus = %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestInstallationRecordToInstallation(t *testing.T) {
	now := time.Now()
	lastUpdated := now.Add(-time.Hour)

	record := &InstallationRecord{
		Key:              "claude-code:npm:/usr/local/bin/claude",
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		InstallMethod:    "npm",
		InstalledVersion: "1.0.0",
		LatestVersion:    "2.0.0",
		ExecutablePath:   "/usr/local/bin/claude",
		InstallPath:      "/usr/local/lib/node_modules/@anthropic-ai/claude-code",
		FirstDetectedAt:  now,
		LastCheckedAt:    now,
		LastUpdatedAt:    &lastUpdated,
		Metadata: map[string]string{
			"npm_version": "10.0.0",
		},
	}

	inst := record.ToInstallation()

	if inst.AgentID != "claude-code" {
		t.Errorf("AgentID = %q, want %q", inst.AgentID, "claude-code")
	}
	if inst.AgentName != "Claude Code" {
		t.Errorf("AgentName = %q, want %q", inst.AgentName, "Claude Code")
	}
	if inst.Method != agent.InstallMethodNPM {
		t.Errorf("Method = %q, want %q", inst.Method, agent.InstallMethodNPM)
	}
	if inst.InstalledVersion.String() != "1.0.0" {
		t.Errorf("InstalledVersion = %q, want %q", inst.InstalledVersion.String(), "1.0.0")
	}
	if inst.LatestVersion == nil || inst.LatestVersion.String() != "2.0.0" {
		t.Errorf("LatestVersion = %v, want 2.0.0", inst.LatestVersion)
	}
	if inst.ExecutablePath != "/usr/local/bin/claude" {
		t.Errorf("ExecutablePath = %q, want %q", inst.ExecutablePath, "/usr/local/bin/claude")
	}
	if inst.InstallPath != "/usr/local/lib/node_modules/@anthropic-ai/claude-code" {
		t.Errorf("InstallPath = %q, want %q", inst.InstallPath, "/usr/local/lib/node_modules/@anthropic-ai/claude-code")
	}
	if inst.Metadata["npm_version"] != "10.0.0" {
		t.Errorf("Metadata[npm_version] = %q, want %q", inst.Metadata["npm_version"], "10.0.0")
	}
}

func TestInstallationRecordToInstallationEmptyLatestVersion(t *testing.T) {
	record := &InstallationRecord{
		Key:              "aider:pipx:/home/user/.local/bin/aider",
		AgentID:          "aider",
		AgentName:        "Aider",
		InstallMethod:    "pipx",
		InstalledVersion: "0.50.0",
		LatestVersion:    "", // Empty
		FirstDetectedAt:  time.Now(),
		LastCheckedAt:    time.Now(),
	}

	inst := record.ToInstallation()

	if inst.LatestVersion != nil {
		t.Errorf("LatestVersion should be nil for empty version, got %v", inst.LatestVersion)
	}
}

func TestFromInstallation(t *testing.T) {
	now := time.Now()
	latestVer := agent.MustParseVersion("2.0.0")

	inst := &agent.Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		Method:           agent.InstallMethodNPM,
		InstalledVersion: agent.MustParseVersion("1.0.0"),
		LatestVersion:    &latestVer,
		ExecutablePath:   "/usr/local/bin/claude",
		InstallPath:      "/usr/local/lib/node_modules/@anthropic-ai/claude-code",
		IsGlobal:         true,
		DetectedAt:       now,
		LastChecked:      now,
		Metadata: map[string]string{
			"npm_version": "10.0.0",
		},
	}

	record := FromInstallation(inst)

	expectedKey := "claude-code:npm:/usr/local/bin/claude"
	if record.Key != expectedKey {
		t.Errorf("Key = %q, want %q", record.Key, expectedKey)
	}
	if record.AgentID != "claude-code" {
		t.Errorf("AgentID = %q, want %q", record.AgentID, "claude-code")
	}
	if record.AgentName != "Claude Code" {
		t.Errorf("AgentName = %q, want %q", record.AgentName, "Claude Code")
	}
	if record.InstallMethod != "npm" {
		t.Errorf("InstallMethod = %q, want %q", record.InstallMethod, "npm")
	}
	if record.InstalledVersion != "1.0.0" {
		t.Errorf("InstalledVersion = %q, want %q", record.InstalledVersion, "1.0.0")
	}
	if record.LatestVersion != "2.0.0" {
		t.Errorf("LatestVersion = %q, want %q", record.LatestVersion, "2.0.0")
	}
	if record.ExecutablePath != "/usr/local/bin/claude" {
		t.Errorf("ExecutablePath = %q, want %q", record.ExecutablePath, "/usr/local/bin/claude")
	}
	if record.Metadata["npm_version"] != "10.0.0" {
		t.Errorf("Metadata[npm_version] = %q, want %q", record.Metadata["npm_version"], "10.0.0")
	}
}

func TestFromInstallationNilLatestVersion(t *testing.T) {
	inst := &agent.Installation{
		AgentID:          "aider",
		AgentName:        "Aider",
		Method:           agent.InstallMethodPipx,
		InstalledVersion: agent.MustParseVersion("0.50.0"),
		LatestVersion:    nil,
		ExecutablePath:   "/home/user/.local/bin/aider",
		DetectedAt:       time.Now(),
		LastChecked:      time.Now(),
	}

	record := FromInstallation(inst)

	if record.LatestVersion != "" {
		t.Errorf("LatestVersion should be empty, got %q", record.LatestVersion)
	}
}

func TestUpdateEvent(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(time.Minute)

	event := &UpdateEvent{
		ID:            1,
		AgentID:       "claude-code",
		AgentName:     "Claude Code",
		InstallMethod: "npm",
		FromVersion:   "1.0.0",
		ToVersion:     "2.0.0",
		Status:        UpdateStatusCompleted,
		ErrorMessage:  "",
		StartedAt:     now,
		CompletedAt:   &completedAt,
	}

	if event.ID != 1 {
		t.Errorf("ID = %d, want 1", event.ID)
	}
	if event.AgentID != "claude-code" {
		t.Errorf("AgentID = %q, want %q", event.AgentID, "claude-code")
	}
	if event.Status != UpdateStatusCompleted {
		t.Errorf("Status = %q, want %q", event.Status, UpdateStatusCompleted)
	}
	if event.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

func TestUpdateEventFailed(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(time.Minute)

	event := &UpdateEvent{
		ID:            2,
		AgentID:       "aider",
		AgentName:     "Aider",
		InstallMethod: "pipx",
		FromVersion:   "0.49.0",
		ToVersion:     "0.50.0",
		Status:        UpdateStatusFailed,
		ErrorMessage:  "network error: connection refused",
		StartedAt:     now,
		CompletedAt:   &completedAt,
	}

	if event.Status != UpdateStatusFailed {
		t.Errorf("Status = %q, want %q", event.Status, UpdateStatusFailed)
	}
	if event.ErrorMessage == "" {
		t.Error("ErrorMessage should not be empty for failed status")
	}
}

func TestInstallationRecordRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	latestVer := agent.MustParseVersion("2.5.0")

	original := &agent.Installation{
		AgentID:          "test-agent",
		AgentName:        "Test Agent",
		Method:           agent.InstallMethodBrew,
		InstalledVersion: agent.MustParseVersion("2.0.0"),
		LatestVersion:    &latestVer,
		ExecutablePath:   "/usr/local/bin/test",
		InstallPath:      "/usr/local/Cellar/test/2.0.0",
		IsGlobal:         true,
		DetectedAt:       now,
		LastChecked:      now,
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Convert to record
	record := FromInstallation(original)

	// Convert back to installation
	restored := record.ToInstallation()

	// Verify key fields match
	if restored.AgentID != original.AgentID {
		t.Errorf("AgentID mismatch: %q != %q", restored.AgentID, original.AgentID)
	}
	if restored.AgentName != original.AgentName {
		t.Errorf("AgentName mismatch: %q != %q", restored.AgentName, original.AgentName)
	}
	if restored.Method != original.Method {
		t.Errorf("Method mismatch: %q != %q", restored.Method, original.Method)
	}
	if restored.InstalledVersion.String() != original.InstalledVersion.String() {
		t.Errorf("InstalledVersion mismatch: %q != %q", restored.InstalledVersion.String(), original.InstalledVersion.String())
	}
	if restored.LatestVersion == nil || restored.LatestVersion.String() != original.LatestVersion.String() {
		t.Errorf("LatestVersion mismatch")
	}
	if restored.ExecutablePath != original.ExecutablePath {
		t.Errorf("ExecutablePath mismatch: %q != %q", restored.ExecutablePath, original.ExecutablePath)
	}
	if restored.Metadata["key1"] != original.Metadata["key1"] {
		t.Errorf("Metadata[key1] mismatch: %q != %q", restored.Metadata["key1"], original.Metadata["key1"])
	}
}
