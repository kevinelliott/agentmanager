package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

func setupTestStore(t *testing.T) (*SQLiteStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "agentmgr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()
	if err := store.Initialize(ctx); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to initialize store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestNewSQLiteStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agentmgr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "agentmgr.db")
	if store.dbPath != expectedPath {
		t.Errorf("dbPath = %q, want %q", store.dbPath, expectedPath)
	}
}

func TestSQLiteStoreInitialize(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Verify that db is set
	if store.db == nil {
		t.Error("db should not be nil after initialization")
	}
}

func TestSQLiteStoreClose(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Close should not error
	err := store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Closing again should not error (nil db)
	store.db = nil
	err = store.Close()
	if err != nil {
		t.Errorf("Close() on nil db error = %v", err)
	}
}

func TestSaveAndGetInstallation(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

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

	// Save
	err := store.SaveInstallation(ctx, inst)
	if err != nil {
		t.Fatalf("SaveInstallation() error = %v", err)
	}

	// Get
	key := inst.Key()
	retrieved, err := store.GetInstallation(ctx, key)
	if err != nil {
		t.Fatalf("GetInstallation() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetInstallation() returned nil")
	}

	// Verify
	if retrieved.AgentID != inst.AgentID {
		t.Errorf("AgentID = %q, want %q", retrieved.AgentID, inst.AgentID)
	}
	if retrieved.AgentName != inst.AgentName {
		t.Errorf("AgentName = %q, want %q", retrieved.AgentName, inst.AgentName)
	}
	if retrieved.Method != inst.Method {
		t.Errorf("Method = %q, want %q", retrieved.Method, inst.Method)
	}
	if retrieved.InstalledVersion.String() != inst.InstalledVersion.String() {
		t.Errorf("InstalledVersion = %q, want %q", retrieved.InstalledVersion.String(), inst.InstalledVersion.String())
	}
	if retrieved.LatestVersion == nil || retrieved.LatestVersion.String() != inst.LatestVersion.String() {
		t.Error("LatestVersion mismatch")
	}
	if retrieved.Metadata["npm_version"] != "10.0.0" {
		t.Errorf("Metadata[npm_version] = %q, want %q", retrieved.Metadata["npm_version"], "10.0.0")
	}
}

func TestGetInstallationNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	retrieved, err := store.GetInstallation(ctx, "nonexistent-key")
	if err != nil {
		t.Fatalf("GetInstallation() error = %v", err)
	}
	if retrieved != nil {
		t.Error("GetInstallation() should return nil for nonexistent key")
	}
}

func TestUpdateInstallation(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()
	inst := &agent.Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		Method:           agent.InstallMethodNPM,
		InstalledVersion: agent.MustParseVersion("1.0.0"),
		ExecutablePath:   "/usr/local/bin/claude",
		DetectedAt:       now,
		LastChecked:      now,
	}

	// Save initial
	err := store.SaveInstallation(ctx, inst)
	if err != nil {
		t.Fatalf("SaveInstallation() error = %v", err)
	}

	// Update version
	newVersion := agent.MustParseVersion("2.0.0")
	inst.InstalledVersion = agent.MustParseVersion("2.0.0")
	inst.LatestVersion = &newVersion

	// Save again (should update)
	err = store.SaveInstallation(ctx, inst)
	if err != nil {
		t.Fatalf("SaveInstallation() update error = %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetInstallation(ctx, inst.Key())
	if err != nil {
		t.Fatalf("GetInstallation() error = %v", err)
	}

	if retrieved.InstalledVersion.String() != "2.0.0" {
		t.Errorf("InstalledVersion = %q, want %q", retrieved.InstalledVersion.String(), "2.0.0")
	}
}

func TestListInstallations(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()

	// Add multiple installations
	installations := []*agent.Installation{
		{
			AgentID:          "claude-code",
			AgentName:        "Claude Code",
			Method:           agent.InstallMethodNPM,
			InstalledVersion: agent.MustParseVersion("1.0.0"),
			ExecutablePath:   "/usr/local/bin/claude",
			DetectedAt:       now,
			LastChecked:      now,
		},
		{
			AgentID:          "aider",
			AgentName:        "Aider",
			Method:           agent.InstallMethodPipx,
			InstalledVersion: agent.MustParseVersion("0.50.0"),
			ExecutablePath:   "/home/user/.local/bin/aider",
			DetectedAt:       now,
			LastChecked:      now,
		},
		{
			AgentID:          "copilot",
			AgentName:        "GitHub Copilot",
			Method:           agent.InstallMethodNPM,
			InstalledVersion: agent.MustParseVersion("1.0.0"),
			ExecutablePath:   "/usr/local/bin/copilot",
			DetectedAt:       now,
			LastChecked:      now,
		},
	}

	for _, inst := range installations {
		if err := store.SaveInstallation(ctx, inst); err != nil {
			t.Fatalf("SaveInstallation() error = %v", err)
		}
	}

	// List all
	all, err := store.ListInstallations(ctx, nil)
	if err != nil {
		t.Fatalf("ListInstallations() error = %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListInstallations() count = %d, want 3", len(all))
	}

	// Filter by agent ID
	filter := &agent.Filter{AgentID: "claude-code"}
	filtered, err := store.ListInstallations(ctx, filter)
	if err != nil {
		t.Fatalf("ListInstallations() with filter error = %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("ListInstallations() filtered count = %d, want 1", len(filtered))
	}

	// Filter by method
	methodFilter := &agent.Filter{Method: agent.InstallMethodNPM}
	byMethod, err := store.ListInstallations(ctx, methodFilter)
	if err != nil {
		t.Fatalf("ListInstallations() with method filter error = %v", err)
	}
	if len(byMethod) != 2 {
		t.Errorf("ListInstallations() by method count = %d, want 2", len(byMethod))
	}
}

func TestListInstallationsWithUpdateFilter(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()
	latestVer := agent.MustParseVersion("2.0.0")

	// Installation with update available
	withUpdate := &agent.Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		Method:           agent.InstallMethodNPM,
		InstalledVersion: agent.MustParseVersion("1.0.0"),
		LatestVersion:    &latestVer,
		ExecutablePath:   "/usr/local/bin/claude",
		DetectedAt:       now,
		LastChecked:      now,
	}

	// Installation without update
	currentVer := agent.MustParseVersion("0.50.0")
	withoutUpdate := &agent.Installation{
		AgentID:          "aider",
		AgentName:        "Aider",
		Method:           agent.InstallMethodPipx,
		InstalledVersion: agent.MustParseVersion("0.50.0"),
		LatestVersion:    &currentVer,
		ExecutablePath:   "/home/user/.local/bin/aider",
		DetectedAt:       now,
		LastChecked:      now,
	}

	if err := store.SaveInstallation(ctx, withUpdate); err != nil {
		t.Fatalf("SaveInstallation() error = %v", err)
	}
	if err := store.SaveInstallation(ctx, withoutUpdate); err != nil {
		t.Fatalf("SaveInstallation() error = %v", err)
	}

	// Filter for those with updates
	hasUpdate := true
	filter := &agent.Filter{HasUpdate: &hasUpdate}
	withUpdates, err := store.ListInstallations(ctx, filter)
	if err != nil {
		t.Fatalf("ListInstallations() error = %v", err)
	}
	if len(withUpdates) != 1 {
		t.Errorf("ListInstallations() with updates count = %d, want 1", len(withUpdates))
	}

	// Filter for those without updates
	noUpdate := false
	noUpdateFilter := &agent.Filter{HasUpdate: &noUpdate}
	withoutUpdates, err := store.ListInstallations(ctx, noUpdateFilter)
	if err != nil {
		t.Fatalf("ListInstallations() error = %v", err)
	}
	if len(withoutUpdates) != 1 {
		t.Errorf("ListInstallations() without updates count = %d, want 1", len(withoutUpdates))
	}
}

func TestDeleteInstallation(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()
	inst := &agent.Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		Method:           agent.InstallMethodNPM,
		InstalledVersion: agent.MustParseVersion("1.0.0"),
		ExecutablePath:   "/usr/local/bin/claude",
		DetectedAt:       now,
		LastChecked:      now,
	}

	// Save
	if err := store.SaveInstallation(ctx, inst); err != nil {
		t.Fatalf("SaveInstallation() error = %v", err)
	}

	// Delete
	err := store.DeleteInstallation(ctx, inst.Key())
	if err != nil {
		t.Fatalf("DeleteInstallation() error = %v", err)
	}

	// Verify deleted - GetInstallation returns nil, nil for not found
	retrieved, err := store.GetInstallation(ctx, inst.Key())
	if err != nil {
		t.Fatalf("GetInstallation() error = %v", err)
	}
	if retrieved != nil {
		t.Error("Installation should be deleted")
	}
}

func TestDeleteInstallationNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	err := store.DeleteInstallation(ctx, "nonexistent-key")
	if err == nil {
		t.Error("DeleteInstallation() should return error for nonexistent key")
	}
}

func TestSaveAndGetUpdateEvent(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()
	event := &UpdateEvent{
		AgentID:       "claude-code",
		AgentName:     "Claude Code",
		InstallMethod: "npm",
		FromVersion:   "1.0.0",
		ToVersion:     "2.0.0",
		Status:        UpdateStatusRunning,
		StartedAt:     now,
	}

	// Save new event
	err := store.SaveUpdateEvent(ctx, event)
	if err != nil {
		t.Fatalf("SaveUpdateEvent() error = %v", err)
	}
	if event.ID == 0 {
		t.Error("Event ID should be set after insert")
	}

	// Update event
	completedAt := now.Add(time.Minute)
	event.Status = UpdateStatusCompleted
	event.CompletedAt = &completedAt

	err = store.SaveUpdateEvent(ctx, event)
	if err != nil {
		t.Fatalf("SaveUpdateEvent() update error = %v", err)
	}

	// Get history
	history, err := store.GetUpdateHistory(ctx, "claude-code", 10)
	if err != nil {
		t.Fatalf("GetUpdateHistory() error = %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("GetUpdateHistory() count = %d, want 1", len(history))
	}

	retrieved := history[0]
	if retrieved.Status != UpdateStatusCompleted {
		t.Errorf("Status = %q, want %q", retrieved.Status, UpdateStatusCompleted)
	}
	if retrieved.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestGetUpdateHistoryOrdering(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	// Create multiple events with different timestamps
	for i := 0; i < 5; i++ {
		event := &UpdateEvent{
			AgentID:       "claude-code",
			AgentName:     "Claude Code",
			InstallMethod: "npm",
			FromVersion:   "1.0.0",
			ToVersion:     "2.0.0",
			Status:        UpdateStatusCompleted,
			StartedAt:     time.Now().Add(time.Duration(i) * time.Hour),
		}
		if err := store.SaveUpdateEvent(ctx, event); err != nil {
			t.Fatalf("SaveUpdateEvent() error = %v", err)
		}
	}

	// Get limited history
	history, err := store.GetUpdateHistory(ctx, "claude-code", 3)
	if err != nil {
		t.Fatalf("GetUpdateHistory() error = %v", err)
	}
	if len(history) != 3 {
		t.Errorf("GetUpdateHistory() count = %d, want 3", len(history))
	}

	// Verify ordering (most recent first)
	for i := 0; i < len(history)-1; i++ {
		if history[i].StartedAt.Before(history[i+1].StartedAt) {
			t.Error("History should be ordered by started_at DESC")
		}
	}
}

func TestCatalogCache(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	testData := []byte(`{"version": "1.0.0", "agents": {}}`)
	testEtag := "abc123"

	// Save cache
	err := store.SaveCatalogCache(ctx, testData, testEtag)
	if err != nil {
		t.Fatalf("SaveCatalogCache() error = %v", err)
	}

	// Get cache
	data, etag, cachedAt, err := store.GetCatalogCache(ctx)
	if err != nil {
		t.Fatalf("GetCatalogCache() error = %v", err)
	}

	if string(data) != string(testData) {
		t.Errorf("data = %q, want %q", string(data), string(testData))
	}
	if etag != testEtag {
		t.Errorf("etag = %q, want %q", etag, testEtag)
	}
	if cachedAt.IsZero() {
		t.Error("cachedAt should not be zero")
	}

	// Update cache
	newData := []byte(`{"version": "2.0.0", "agents": {}}`)
	newEtag := "def456"

	err = store.SaveCatalogCache(ctx, newData, newEtag)
	if err != nil {
		t.Fatalf("SaveCatalogCache() update error = %v", err)
	}

	data2, etag2, _, err := store.GetCatalogCache(ctx)
	if err != nil {
		t.Fatalf("GetCatalogCache() error = %v", err)
	}

	if string(data2) != string(newData) {
		t.Errorf("data = %q, want %q", string(data2), string(newData))
	}
	if etag2 != newEtag {
		t.Errorf("etag = %q, want %q", etag2, newEtag)
	}
}

func TestCatalogCacheEmpty(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	data, etag, cachedAt, err := store.GetCatalogCache(ctx)
	if err != nil {
		t.Fatalf("GetCatalogCache() error = %v", err)
	}
	if data != nil {
		t.Error("data should be nil for empty cache")
	}
	if etag != "" {
		t.Error("etag should be empty for empty cache")
	}
	if !cachedAt.IsZero() {
		t.Error("cachedAt should be zero for empty cache")
	}
}

func TestSettings(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	// Set setting
	err := store.SetSetting(ctx, "test_key", "test_value")
	if err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}

	// Get setting
	value, err := store.GetSetting(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if value != "test_value" {
		t.Errorf("value = %q, want %q", value, "test_value")
	}

	// Update setting
	err = store.SetSetting(ctx, "test_key", "new_value")
	if err != nil {
		t.Fatalf("SetSetting() update error = %v", err)
	}

	value2, err := store.GetSetting(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if value2 != "new_value" {
		t.Errorf("value = %q, want %q", value2, "new_value")
	}

	// Delete setting
	err = store.DeleteSetting(ctx, "test_key")
	if err != nil {
		t.Fatalf("DeleteSetting() error = %v", err)
	}

	// Verify deleted - GetSetting returns "", nil for not found
	value3, err := store.GetSetting(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if value3 != "" {
		t.Errorf("value should be empty after delete, got %q", value3)
	}
}

func TestGetSettingNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	value, err := store.GetSetting(ctx, "nonexistent_key")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if value != "" {
		t.Errorf("value should be empty for nonexistent key, got %q", value)
	}
}
