package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kevinelliott/agentmanager/internal/versionfetch"
	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/platform"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

// --- fakes ---------------------------------------------------------------

type fakePlatform struct{}

func (fakePlatform) ID() platform.ID                                             { return platform.Darwin }
func (fakePlatform) Architecture() string                                        { return "amd64" }
func (fakePlatform) Name() string                                                { return "macOS" }
func (fakePlatform) GetDataDir() string                                          { return "/tmp/data" }
func (fakePlatform) GetConfigDir() string                                        { return "/tmp/config" }
func (fakePlatform) GetCacheDir() string                                         { return "/tmp/cache" }
func (fakePlatform) GetLogDir() string                                           { return "/tmp/log" }
func (fakePlatform) GetIPCSocketPath() string                                    { return "/tmp/ipc.sock" }
func (fakePlatform) EnableAutoStart(ctx context.Context) error                   { return nil }
func (fakePlatform) DisableAutoStart(ctx context.Context) error                  { return nil }
func (fakePlatform) IsAutoStartEnabled(ctx context.Context) (bool, error)        { return false, nil }
func (fakePlatform) FindExecutable(name string) (string, error)                  { return "", nil }
func (fakePlatform) FindExecutables(name string) ([]string, error)               { return nil, nil }
func (fakePlatform) IsExecutableInPath(name string) bool                         { return false }
func (fakePlatform) GetPathDirs() []string                                       { return nil }
func (fakePlatform) GetShell() string                                            { return "/bin/bash" }
func (fakePlatform) GetShellArg() string                                         { return "-c" }
func (fakePlatform) ShowNotification(title, message string) error                { return nil }
func (fakePlatform) ShowChangelogDialog(a, b, c, d string) platform.DialogResult { return 0 }

type fakeCatalog struct {
	agents []catalog.AgentDef
	err    error
	calls  int
	mu     sync.Mutex
}

func (f *fakeCatalog) GetAgentsForPlatform(_ context.Context, _ string) ([]catalog.AgentDef, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.agents, f.err
}

type fakeDetector struct {
	installations []*agent.Installation
	err           error
	calls         int
	mu            sync.Mutex
}

func (f *fakeDetector) DetectAll(_ context.Context, _ []catalog.AgentDef) ([]*agent.Installation, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	// Return a deep-enough copy so callers mutating LatestVersion don't leak across tests.
	out := make([]*agent.Installation, len(f.installations))
	for i, inst := range f.installations {
		cp := *inst
		out[i] = &cp
	}
	return out, nil
}

type fakeFetcher struct {
	versions map[string]agent.Version
	errs     map[string]error
	calls    map[string]int
	mu       sync.Mutex
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{
		versions: map[string]agent.Version{},
		errs:     map[string]error{},
		calls:    map[string]int{},
	}
}

func (f *fakeFetcher) GetLatestVersion(_ context.Context, method catalog.InstallMethodDef) (agent.Version, error) {
	f.mu.Lock()
	f.calls[method.Package]++
	err := f.errs[method.Package]
	ver, ok := f.versions[method.Package]
	f.mu.Unlock()

	if err != nil {
		return agent.Version{}, err
	}
	if !ok {
		ver = agent.Version{Major: 1, Minor: 0, Patch: 0}
	}
	return ver, nil
}

// fakeStore is a minimal in-memory storage.Store covering only the calls the
// pipeline exercises. Unused methods return zero values.
type fakeStore struct {
	mu sync.Mutex

	detection        []*agent.Installation
	detectionAt      time.Time
	detectionErr     error
	saveCount        int
	saveCache        []*agent.Installation
	clearCount       int
	lastUpdateCheck  time.Time
	lastUpdateErr    error
	setLastUpdateAt  time.Time
	setLastUpdateCnt int
}

var _ storage.Store = (*fakeStore)(nil)

func (s *fakeStore) Initialize(context.Context) error { return nil }
func (s *fakeStore) Close() error                     { return nil }

func (s *fakeStore) SaveInstallation(context.Context, *agent.Installation) error { return nil }
func (s *fakeStore) GetInstallation(context.Context, string) (*agent.Installation, error) {
	return nil, nil
}
func (s *fakeStore) ListInstallations(context.Context, *agent.Filter) ([]*agent.Installation, error) {
	return nil, nil
}
func (s *fakeStore) DeleteInstallation(context.Context, string) error            { return nil }
func (s *fakeStore) SaveUpdateEvent(context.Context, *storage.UpdateEvent) error { return nil }
func (s *fakeStore) GetUpdateHistory(context.Context, string, int) ([]*storage.UpdateEvent, error) {
	return nil, nil
}
func (s *fakeStore) SaveCatalogCache(context.Context, []byte, string) error { return nil }
func (s *fakeStore) GetCatalogCache(context.Context) ([]byte, string, time.Time, error) {
	return nil, "", time.Time{}, nil
}

func (s *fakeStore) SaveDetectionCache(_ context.Context, insts []*agent.Installation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveCount++
	// Store a copy to avoid accidental alias sharing.
	cp := make([]*agent.Installation, len(insts))
	for i, inst := range insts {
		c := *inst
		cp[i] = &c
	}
	s.saveCache = cp
	s.detection = cp
	s.detectionAt = time.Now()
	return nil
}

func (s *fakeStore) GetDetectionCache(context.Context) ([]*agent.Installation, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.detectionErr != nil {
		return nil, time.Time{}, s.detectionErr
	}
	return s.detection, s.detectionAt, nil
}

func (s *fakeStore) ClearDetectionCache(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearCount++
	s.detection = nil
	s.detectionAt = time.Time{}
	return nil
}

func (s *fakeStore) GetDetectionCacheTime(context.Context) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.detectionAt, nil
}

func (s *fakeStore) SetLastUpdateCheckTime(_ context.Context, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setLastUpdateCnt++
	s.setLastUpdateAt = t
	s.lastUpdateCheck = t
	return nil
}

func (s *fakeStore) GetLastUpdateCheckTime(context.Context) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastUpdateErr != nil {
		return time.Time{}, s.lastUpdateErr
	}
	return s.lastUpdateCheck, nil
}

func (s *fakeStore) GetSetting(context.Context, string) (string, error) { return "", nil }
func (s *fakeStore) SetSetting(context.Context, string, string) error   { return nil }
func (s *fakeStore) DeleteSetting(context.Context, string) error        { return nil }

// --- helpers -------------------------------------------------------------

func defaultConfig() *config.Config {
	return &config.Config{
		Detection: config.DetectionConfig{
			CacheEnabled:             true,
			CacheDuration:            time.Hour,
			UpdateCheckCacheDuration: 15 * time.Minute,
		},
	}
}

func makeAgentDef(id string) catalog.AgentDef {
	return catalog.AgentDef{
		ID:   id,
		Name: id,
		InstallMethods: map[string]catalog.InstallMethodDef{
			"npm": {Method: "npm", Package: id},
		},
	}
}

func makeInstallation(id string) *agent.Installation {
	return &agent.Installation{
		AgentID:          id,
		AgentName:        id,
		Method:           agent.InstallMethod("npm"),
		InstalledVersion: agent.Version{Major: 0, Minor: 9, Patch: 0},
	}
}

// --- tests ---------------------------------------------------------------

func TestPipeline_ColdDetectFillsVersionsAndSavesCache(t *testing.T) {
	cfg := defaultConfig()
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a"), makeAgentDef("b")}}
	det := &fakeDetector{installations: []*agent.Installation{makeInstallation("a"), makeInstallation("b")}}
	fetcher := newFakeFetcher()
	fetcher.versions["a"] = agent.Version{Major: 2, Minor: 0, Patch: 0}
	fetcher.versions["b"] = agent.Version{Major: 3, Minor: 1, Patch: 0}
	store := &fakeStore{}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	res, err := p.DetectAndCheckVersions(context.Background(), Options{})
	if err != nil {
		t.Fatalf("DetectAndCheckVersions: %v", err)
	}

	if res.UsedDetectionCache {
		t.Errorf("UsedDetectionCache = true, want false (cold detect)")
	}
	if !res.RanVersionCheck {
		t.Errorf("RanVersionCheck = false, want true on cold detect")
	}
	if det.calls != 1 {
		t.Errorf("detector.DetectAll called %d times, want 1", det.calls)
	}
	if len(res.Installations) != 2 {
		t.Fatalf("Installations len = %d, want 2", len(res.Installations))
	}
	if res.Installations[0].LatestVersion == nil || *res.Installations[0].LatestVersion != (agent.Version{Major: 2}) {
		t.Errorf("Installations[0].LatestVersion = %v, want 2.0.0", res.Installations[0].LatestVersion)
	}
	if _, ok := res.AgentDefMap["a"]; !ok {
		t.Errorf("AgentDefMap missing 'a'")
	}
	if store.saveCount != 1 {
		t.Errorf("store.SaveDetectionCache called %d times, want 1", store.saveCount)
	}
	if store.setLastUpdateCnt != 1 {
		t.Errorf("store.SetLastUpdateCheckTime called %d times, want 1", store.setLastUpdateCnt)
	}
	if filtered := versionfetch.NonNilErrors(res.VersionCheckErrors); len(filtered) != 0 {
		t.Errorf("NonNilErrors len = %d, want 0", len(filtered))
	}
}

func TestPipeline_CacheHitSkipsDetectionAndVersionCheckWhenFresh(t *testing.T) {
	cfg := defaultConfig()
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}
	det := &fakeDetector{installations: []*agent.Installation{makeInstallation("a")}}
	fetcher := newFakeFetcher()
	store := &fakeStore{
		detection:       []*agent.Installation{makeInstallation("a")},
		detectionAt:     time.Now(),
		lastUpdateCheck: time.Now(), // fresh
	}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	res, err := p.DetectAndCheckVersions(context.Background(), Options{})
	if err != nil {
		t.Fatalf("DetectAndCheckVersions: %v", err)
	}

	if !res.UsedDetectionCache {
		t.Errorf("UsedDetectionCache = false, want true (fresh cache)")
	}
	if res.RanVersionCheck {
		t.Errorf("RanVersionCheck = true, want false (fresh update-check TTL)")
	}
	if det.calls != 0 {
		t.Errorf("detector.DetectAll called %d times, want 0", det.calls)
	}
	if store.saveCount != 0 {
		t.Errorf("store.SaveDetectionCache called %d times, want 0", store.saveCount)
	}
	if store.setLastUpdateCnt != 0 {
		t.Errorf("store.SetLastUpdateCheckTime called %d times, want 0", store.setLastUpdateCnt)
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("fetcher was called: %+v", fetcher.calls)
	}
}

func TestPipeline_CacheHitWithStaleUpdateCheckRunsVersionCheckOnly(t *testing.T) {
	cfg := defaultConfig()
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}
	det := &fakeDetector{installations: []*agent.Installation{makeInstallation("a")}}
	fetcher := newFakeFetcher()
	fetcher.versions["a"] = agent.Version{Major: 2, Minor: 0, Patch: 0}

	store := &fakeStore{
		detection:       []*agent.Installation{makeInstallation("a")},
		detectionAt:     time.Now(),
		lastUpdateCheck: time.Now().Add(-1 * time.Hour), // stale: older than UpdateCheckCacheDuration (15m)
	}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	res, err := p.DetectAndCheckVersions(context.Background(), Options{})
	if err != nil {
		t.Fatalf("DetectAndCheckVersions: %v", err)
	}

	if !res.UsedDetectionCache {
		t.Errorf("UsedDetectionCache = false, want true")
	}
	if !res.RanVersionCheck {
		t.Errorf("RanVersionCheck = false, want true (stale update-check TTL)")
	}
	if det.calls != 0 {
		t.Errorf("detector.DetectAll called %d times, want 0 (cache hit)", det.calls)
	}
	if store.saveCount != 1 {
		t.Errorf("store.SaveDetectionCache called %d times, want 1", store.saveCount)
	}
	if store.setLastUpdateCnt != 1 {
		t.Errorf("store.SetLastUpdateCheckTime called %d times, want 1", store.setLastUpdateCnt)
	}
	if fetcher.calls["a"] != 1 {
		t.Errorf("fetcher.calls[a] = %d, want 1", fetcher.calls["a"])
	}
}

func TestPipeline_ForceRefreshBypassesCacheAndClearsIt(t *testing.T) {
	cfg := defaultConfig()
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}
	det := &fakeDetector{installations: []*agent.Installation{makeInstallation("a")}}
	fetcher := newFakeFetcher()
	store := &fakeStore{
		detection:       []*agent.Installation{makeInstallation("stale")},
		detectionAt:     time.Now(),
		lastUpdateCheck: time.Now(),
	}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	res, err := p.DetectAndCheckVersions(context.Background(), Options{ForceRefresh: true})
	if err != nil {
		t.Fatalf("DetectAndCheckVersions: %v", err)
	}

	if res.UsedDetectionCache {
		t.Errorf("UsedDetectionCache = true, want false (force-refresh)")
	}
	if !res.RanVersionCheck {
		t.Errorf("RanVersionCheck = false, want true (cold detect)")
	}
	if store.clearCount != 1 {
		t.Errorf("store.ClearDetectionCache called %d times, want 1", store.clearCount)
	}
	if det.calls != 1 {
		t.Errorf("detector.DetectAll called %d times, want 1", det.calls)
	}
}

func TestPipeline_SkipVersionCheckSkipsFetchAndCacheWrite(t *testing.T) {
	cfg := defaultConfig()
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}
	det := &fakeDetector{installations: []*agent.Installation{makeInstallation("a")}}
	fetcher := newFakeFetcher()
	store := &fakeStore{}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	res, err := p.DetectAndCheckVersions(context.Background(), Options{SkipVersionCheck: true})
	if err != nil {
		t.Fatalf("DetectAndCheckVersions: %v", err)
	}

	if res.RanVersionCheck {
		t.Errorf("RanVersionCheck = true, want false (SkipVersionCheck)")
	}
	if store.saveCount != 0 {
		t.Errorf("store.SaveDetectionCache called %d times, want 0 (skipped)", store.saveCount)
	}
	if store.setLastUpdateCnt != 0 {
		t.Errorf("store.SetLastUpdateCheckTime called %d times, want 0 (skipped)", store.setLastUpdateCnt)
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("fetcher called when SkipVersionCheck was true: %+v", fetcher.calls)
	}
}

func TestPipeline_VersionCheckErrorsAggregatedAndReturned(t *testing.T) {
	cfg := defaultConfig()
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a"), makeAgentDef("b"), makeAgentDef("c")}}
	det := &fakeDetector{installations: []*agent.Installation{
		makeInstallation("a"), makeInstallation("b"), makeInstallation("c"),
	}}
	fetcher := newFakeFetcher()
	boom := errors.New("boom")
	fetcher.errs["b"] = boom
	store := &fakeStore{}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	res, err := p.DetectAndCheckVersions(context.Background(), Options{})
	if err != nil {
		t.Fatalf("DetectAndCheckVersions: %v", err)
	}
	if !res.RanVersionCheck {
		t.Fatalf("RanVersionCheck = false, want true")
	}
	if len(res.VersionCheckErrors) != 3 {
		t.Fatalf("VersionCheckErrors len = %d, want 3 (parallel to installations)", len(res.VersionCheckErrors))
	}
	if res.VersionCheckErrors[0] != nil {
		t.Errorf("VersionCheckErrors[0] = %v, want nil", res.VersionCheckErrors[0])
	}
	if res.VersionCheckErrors[1] == nil || !errors.Is(res.VersionCheckErrors[1], boom) {
		t.Errorf("VersionCheckErrors[1] = %v, want wrap of boom", res.VersionCheckErrors[1])
	}
	if res.VersionCheckErrors[2] != nil {
		t.Errorf("VersionCheckErrors[2] = %v, want nil", res.VersionCheckErrors[2])
	}

	filtered := versionfetch.NonNilErrors(res.VersionCheckErrors)
	if len(filtered) != 1 {
		t.Errorf("NonNilErrors len = %d, want 1", len(filtered))
	}
}

func TestPipeline_CatalogErrorIsFatal(t *testing.T) {
	cfg := defaultConfig()
	cat := &fakeCatalog{err: errors.New("network down")}
	det := &fakeDetector{}
	fetcher := newFakeFetcher()
	store := &fakeStore{}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	_, err := p.DetectAndCheckVersions(context.Background(), Options{})
	if err == nil {
		t.Fatalf("DetectAndCheckVersions returned nil, want error")
	}
	if det.calls != 0 {
		t.Errorf("detector called despite catalog error: %d", det.calls)
	}
}

func TestPipeline_TolerateCatalogErrorContinuesWithEmptyDefs(t *testing.T) {
	cfg := defaultConfig()
	cat := &fakeCatalog{err: errors.New("network down")}
	det := &fakeDetector{} // returns nil installations
	fetcher := newFakeFetcher()
	store := &fakeStore{}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	res, err := p.DetectAndCheckVersions(context.Background(), Options{TolerateCatalogError: true})
	if err != nil {
		t.Fatalf("DetectAndCheckVersions with TolerateCatalogError = %v, want nil", err)
	}
	if det.calls != 1 {
		t.Errorf("detector.DetectAll called %d times, want 1 (should proceed even without catalog)", det.calls)
	}
	if len(res.AgentDefs) != 0 {
		t.Errorf("AgentDefs len = %d, want 0", len(res.AgentDefs))
	}
	if len(res.AgentDefMap) != 0 {
		t.Errorf("AgentDefMap len = %d, want 0", len(res.AgentDefMap))
	}
}

func TestPipeline_DetectorErrorIsFatal(t *testing.T) {
	cfg := defaultConfig()
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}
	det := &fakeDetector{err: errors.New("detection failed")}
	fetcher := newFakeFetcher()
	store := &fakeStore{}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	_, err := p.DetectAndCheckVersions(context.Background(), Options{})
	if err == nil {
		t.Fatalf("DetectAndCheckVersions returned nil, want error from detector")
	}
	if len(fetcher.calls) != 0 {
		t.Errorf("fetcher called despite detector error: %+v", fetcher.calls)
	}
}

func TestPipeline_CacheDisabledAlwaysDetects(t *testing.T) {
	cfg := defaultConfig()
	cfg.Detection.CacheEnabled = false
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}
	det := &fakeDetector{installations: []*agent.Installation{makeInstallation("a")}}
	fetcher := newFakeFetcher()
	store := &fakeStore{
		// cache is populated but must be ignored.
		detection:   []*agent.Installation{makeInstallation("stale")},
		detectionAt: time.Now(),
	}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	res, err := p.DetectAndCheckVersions(context.Background(), Options{})
	if err != nil {
		t.Fatalf("DetectAndCheckVersions: %v", err)
	}
	if res.UsedDetectionCache {
		t.Errorf("UsedDetectionCache = true, want false when cache disabled")
	}
	if det.calls != 1 {
		t.Errorf("detector.DetectAll called %d times, want 1", det.calls)
	}
	if store.saveCount != 0 {
		t.Errorf("store.SaveDetectionCache called %d times, want 0 (cache disabled)", store.saveCount)
	}
}

func TestPipeline_CustomConcurrencyPassedToFetcher(t *testing.T) {
	// We can't observe internal goroutine count here without plumbing it
	// through, so this test simply verifies the pipeline does not reject
	// a user-specified concurrency and still dispatches calls.
	cfg := defaultConfig()
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}
	det := &fakeDetector{installations: []*agent.Installation{makeInstallation("a")}}
	fetcher := newFakeFetcher()
	store := &fakeStore{}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	res, err := p.DetectAndCheckVersions(context.Background(), Options{VersionCheckConcurrency: 2})
	if err != nil {
		t.Fatalf("DetectAndCheckVersions: %v", err)
	}
	if !res.RanVersionCheck {
		t.Fatalf("RanVersionCheck = false, want true")
	}
	if fetcher.calls["a"] != 1 {
		t.Errorf("fetcher.calls[a] = %d, want 1", fetcher.calls["a"])
	}
}

func TestPipeline_StaleCacheIsTreatedAsMiss(t *testing.T) {
	cfg := defaultConfig()
	cat := &fakeCatalog{agents: []catalog.AgentDef{makeAgentDef("a")}}
	det := &fakeDetector{installations: []*agent.Installation{makeInstallation("a")}}
	fetcher := newFakeFetcher()
	store := &fakeStore{
		detection:   []*agent.Installation{makeInstallation("old")},
		detectionAt: time.Now().Add(-2 * time.Hour), // older than CacheDuration (1h)
	}

	p := New(cfg, fakePlatform{}, store, cat, det, fetcher)

	res, err := p.DetectAndCheckVersions(context.Background(), Options{})
	if err != nil {
		t.Fatalf("DetectAndCheckVersions: %v", err)
	}
	if res.UsedDetectionCache {
		t.Errorf("UsedDetectionCache = true, want false when cache is stale")
	}
	if det.calls != 1 {
		t.Errorf("detector.DetectAll called %d times, want 1", det.calls)
	}
}
