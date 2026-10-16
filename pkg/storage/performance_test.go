package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

func TestRefreshLockAcrossStoreInstances(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	peer := &SQLiteStore{dbPath: store.dbPath}
	release, err := store.AcquireRefreshLock(t.Context(), "inventory")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	if _, err := peer.AcquireRefreshLock(ctx, "inventory"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked peer: %v", err)
	}
	// Different keys can refresh concurrently.
	other, err := peer.AcquireRefreshLock(t.Context(), "another-package")
	if err != nil {
		t.Fatal(err)
	}
	other()
	release()
	again, err := peer.AcquireRefreshLock(t.Context(), "inventory")
	if err != nil {
		t.Fatal(err)
	}
	again()
}

func TestVersionMetadataPreservesDetectionTimeAndCurrentMembership(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()
	inst := &agent.Installation{AgentID: "a", Method: agent.MethodNPM, InstalledVersion: agent.MustParseVersion("1.0.0")}
	if err := store.SaveDetectionCache(t.Context(), []*agent.Installation{inst}); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetDetectionCacheTime(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	latest := agent.MustParseVersion("2.0.0")
	inst.LatestVersion = &latest
	removed := &agent.Installation{AgentID: "removed", Method: agent.MethodNPM, LatestVersion: inst.LatestVersion}
	if err := store.UpdateDetectionVersions(t.Context(), []*agent.Installation{inst, removed}); err != nil {
		t.Fatal(err)
	}
	snapshot, after, err := store.GetDetectionCache(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !before.Equal(after) {
		t.Fatalf("version refresh extended inventory TTL: %v -> %v", before, after)
	}
	if len(snapshot) != 1 || snapshot[0].LatestVersion == nil || snapshot[0].LatestVersion.String() != "2.0.0" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if err := store.ClearDetectionCache(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateDetectionVersions(t.Context(), []*agent.Installation{inst}); err != nil {
		t.Fatal(err)
	}
	snapshot, _, err = store.GetDetectionCache(t.Context())
	if err != nil || snapshot != nil {
		t.Fatalf("version refresh resurrected a cleared snapshot: %+v, %v", snapshot, err)
	}
}
