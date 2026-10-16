package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

func TestTargetedScanPreservesCompleteCache(t *testing.T) {
	cfg := defaultConfig()
	store := &fakeStore{detection: []*agent.Installation{makeInstallation("b")}, detectionAt: time.Now(), lastUpdateCheck: time.Now()}
	before := store.detectionAt
	det := &fakeDetector{installations: []*agent.Installation{makeInstallation("a")}}
	fetcher := newFakeFetcher()
	p := New(cfg, fakePlatform{}, store, &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a"), makeAgentDef("b")}}, det, fetcher)
	result, err := p.DetectAndCheckVersions(t.Context(), Options{AgentIDs: []string{"a"}, ForceRefresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AgentDefs) != 1 || result.AgentDefs[0].ID != "a" || fetcher.calls["a"] != 1 || fetcher.calls["b"] != 0 {
		t.Fatalf("unrelated agents checked: %+v, %+v", result.AgentDefs, fetcher.calls)
	}
	if len(store.detection) != 1 || store.detection[0].AgentID != "b" || !before.Equal(store.detectionAt) || store.setLastUpdateCnt != 0 {
		t.Fatal("targeted scan changed complete snapshot or its freshness")
	}
}

func TestVersionOnlyRefreshPreservesDetectionFreshness(t *testing.T) {
	at := time.Now().Add(-10 * time.Minute)
	store := &fakeStore{detection: []*agent.Installation{makeInstallation("a")}, detectionAt: at}
	p := New(defaultConfig(), fakePlatform{}, store, &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}, &fakeDetector{}, newFakeFetcher())
	result, err := p.DetectAndCheckVersions(t.Context(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.UsedDetectionCache || !result.RanVersionCheck || !at.Equal(store.detectionAt) || store.versionSaveCount != 1 {
		t.Fatal("version-only refresh extended inventory lifetime")
	}
	if store.detection[0].LatestVersion == nil {
		t.Fatal("latest version not persisted")
	}
}

type countedDetector struct{ calls atomic.Int32 }

func (d *countedDetector) DetectAll(ctx context.Context, _ []catalog.AgentDef) ([]*agent.Installation, error) {
	d.calls.Add(1)
	timer := time.NewTimer(30 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return []*agent.Installation{makeInstallation("a")}, nil
	}
}

func TestConcurrentColdPipelinesShareInventoryAndVersions(t *testing.T) {
	dir := t.TempDir()
	det := &countedDetector{}
	fetcher := newFakeFetcher()
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 8 {
		store, _ := storage.NewSQLiteStore(dir)
		if err := store.Initialize(t.Context()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = store.Close() })
		p := New(defaultConfig(), fakePlatform{}, store, cat, det, fetcher)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := p.DetectAndCheckVersions(t.Context(), Options{})
			if err != nil {
				t.Error(err)
				return
			}
			if len(result.Installations) != 1 || result.Installations[0].LatestVersion == nil {
				t.Error("missing shared result")
			}
		}()
	}
	close(start)
	wg.Wait()
	if det.calls.Load() != 1 || fetcher.calls["a"] != 1 {
		t.Fatalf("expected one inventory/version fetch, got %d/%d", det.calls.Load(), fetcher.calls["a"])
	}
}

func BenchmarkPipelineCachePaths(b *testing.B) {
	for _, count := range []int{0, 10, 50} {
		for _, mode := range []string{"warm", "cold", "versions-expired"} {
			b.Run(fmt.Sprintf("%s/%d", mode, count), func(b *testing.B) {
				store, _ := storage.NewSQLiteStore(b.TempDir())
				if err := store.Initialize(b.Context()); err != nil {
					b.Fatal(err)
				}
				defer store.Close()
				defs := make([]catalog.AgentDef, count)
				installations := make([]*agent.Installation, count)
				for i := range count {
					id := fmt.Sprintf("agent-%d", i)
					defs[i] = makeAgentDef(id)
					installations[i] = makeInstallation(id)
				}
				p := New(defaultConfig(), fakePlatform{}, store, &fakeCatalog{agents: defs}, &fakeDetector{installations: installations}, newFakeFetcher())
				if _, err := p.DetectAndCheckVersions(b.Context(), Options{}); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if mode == "versions-expired" {
						_ = store.SetLastUpdateCheckTime(b.Context(), time.Time{})
					}
					if _, err := p.DetectAndCheckVersions(b.Context(), Options{ForceRefresh: mode == "cold"}); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func TestLocalPhaseReportsWhetherVersionWorkIsDue(t *testing.T) {
	store := &fakeStore{detection: []*agent.Installation{makeInstallation("a")}, detectionAt: time.Now(), lastUpdateCheck: time.Now()}
	fetcher := newFakeFetcher()
	p := New(defaultConfig(), fakePlatform{}, store, &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}, &fakeDetector{}, fetcher)
	result, err := p.DetectAndCheckVersions(t.Context(), Options{SkipVersionCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.NeedsVersionCheck || len(fetcher.calls) != 0 {
		t.Fatal("warm local phase scheduled unnecessary registry work")
	}
	store.lastUpdateCheck = time.Time{}
	result, err = p.DetectAndCheckVersions(t.Context(), Options{SkipVersionCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.NeedsVersionCheck || len(fetcher.calls) != 0 {
		t.Fatal("expired versions were not deferred until after local display")
	}
}
