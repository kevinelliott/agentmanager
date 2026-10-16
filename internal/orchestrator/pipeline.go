// Package orchestrator wires the shared detect-and-version-check pipeline
// used by the CLI, TUI, and systray entry points.
//
// The pipeline consolidates the sequence previously duplicated across call
// sites:
//
//  1. Load the agent catalog for the current platform.
//  2. Consult the detection cache (unless forced to refresh).
//  3. Run detector.DetectAll when no usable cache entry exists.
//  4. Check each installed agent's registry for its latest version.
//  5. Persist the refreshed detection cache and the last-update-check time.
//
// Consolidating this flow behind a single Pipeline type removes the drift
// between call sites that previously caused subtle bugs, while leaving the
// existing concrete types (storage.Store, catalog.Manager, detector.Detector,
// installer.Manager) untouched.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kevinelliott/agentmanager/internal/versionfetch"
	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/detector"
	"github.com/kevinelliott/agentmanager/pkg/installer"
	"github.com/kevinelliott/agentmanager/pkg/platform"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

// CatalogSource is the subset of catalog.Manager used by the pipeline.
// Accepting a narrow interface makes the pipeline trivially testable.
type CatalogSource interface {
	GetAgentsForPlatform(ctx context.Context, platformID string) ([]catalog.AgentDef, error)
}

// Detector is the subset of detector.Detector used by the pipeline.
type Detector interface {
	DetectAll(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error)
}

// VersionFetcher is the subset of installer.Manager used by the pipeline.
// It mirrors versionfetch.LatestVersionFetcher so the pipeline can hand the
// installer directly to the parallel version-check helper.
type VersionFetcher = versionfetch.LatestVersionFetcher

// Pipeline bundles the collaborators required to run the shared detect and
// version-check flow. Construct it with New.
//
// A zero-value Pipeline is not usable; callers must go through New so the
// platform and config defaults are captured consistently.
type Pipeline struct {
	store     storage.Store
	catalog   CatalogSource
	detector  Detector
	installer VersionFetcher
	cfg       *config.Config
	plat      platform.Platform
}

// New returns a Pipeline backed by the provided collaborators.
//
// All arguments are required. The pipeline does not take ownership of any
// collaborator — the caller retains lifecycle responsibility for the storage
// handle and any long-lived managers.
func New(
	cfg *config.Config,
	plat platform.Platform,
	store storage.Store,
	catMgr CatalogSource,
	det Detector,
	inst VersionFetcher,
) *Pipeline {
	return &Pipeline{
		cfg:       cfg,
		plat:      plat,
		store:     store,
		catalog:   catMgr,
		detector:  det,
		installer: inst,
	}
}

// NewFromManagers is a convenience constructor that accepts the concrete
// manager types used across the production call sites. It exists so callers
// do not have to know about the narrow interfaces used for testing.
func NewFromManagers(
	cfg *config.Config,
	plat platform.Platform,
	store storage.Store,
	catMgr *catalog.Manager,
	det *detector.Detector,
	inst *installer.Manager,
) *Pipeline {
	return New(cfg, plat, store, catMgr, det, inst)
}

// Options tunes the behavior of DetectAndCheckVersions.
//
// The zero value performs a normal cache-aware detection with default
// version-check concurrency.
type Options struct {
	// AgentIDs limits a targeted command to the requested agents. Scoped scans
	// never replace the complete installation snapshot or its freshness clock.
	AgentIDs []string

	// ForceRefresh bypasses the detection cache. The cache is also cleared
	// so the next cold start does not see stale data.
	ForceRefresh bool

	// SkipVersionCheck suppresses the parallel latest-version fetch. Useful
	// for fast paths where only the raw detection set is needed.
	SkipVersionCheck bool

	// VersionCheckConcurrency caps the number of parallel version-fetch
	// workers. Defaults to versionfetch.DefaultConcurrency when <= 0.
	VersionCheckConcurrency int

	// TolerateCatalogError swallows errors from the catalog fetch and
	// continues with an empty catalog slice. The systray historically does
	// this so it can start even when the network is unavailable; cached
	// installations remain usable and the menu shows whatever the last
	// detection cache captured.
	TolerateCatalogError bool
}

// Result is the output of DetectAndCheckVersions.
type Result struct {
	// NeedsVersionCheck allows local-first consumers to schedule metadata work
	// only when the version-check TTL has expired or a new scan completed.
	NeedsVersionCheck bool

	// Installations is the merged set of detected agents. LatestVersion is
	// populated where the registry lookup succeeded.
	Installations []*agent.Installation

	// AgentDefs is the catalog slice used for detection, filtered to the
	// current platform.
	AgentDefs []catalog.AgentDef

	// AgentDefMap is a lookup of agent ID to definition for convenience.
	AgentDefMap map[string]catalog.AgentDef

	// UsedDetectionCache reports whether Installations came from the
	// detection cache rather than a fresh detector run.
	UsedDetectionCache bool

	// RanVersionCheck reports whether the latest-version fetch executed.
	// It is false when the cache was fresh and the update-check TTL had
	// not yet elapsed, or when Options.SkipVersionCheck is true.
	RanVersionCheck bool

	// VersionCheckErrors holds one error per installation, aligned with
	// Installations by index. Use versionfetch.NonNilErrors to filter.
	//
	// Nil when RanVersionCheck is false.
	VersionCheckErrors []error
}

// DetectAndCheckVersions uses independent clocks for inventory and registry data.
// Inventory snapshots are saved before optional version checks so local-only
// callers and concurrent processes can immediately reuse a completed scan.
func (p *Pipeline) DetectAndCheckVersions(ctx context.Context, opts Options) (*Result, error) {
	started := time.Now()
	result := &Result{}
	scoped := len(opts.AgentIDs) > 0
	if p.cfg.Detection.CacheEnabled && !opts.ForceRefresh {
		if cached, ok := p.cachedInstallations(ctx, time.Time{}); ok {
			result.Installations = filterInstallations(cached, opts.AgentIDs)
			result.UsedDetectionCache = true
		}
	}
	needVersions := !result.UsedDetectionCache || scoped || p.versionsExpired(ctx)
	result.NeedsVersionCheck = needVersions
	if result.UsedDetectionCache && (opts.SkipVersionCheck || !needVersions) {
		return result, nil
	}

	catalogStart := time.Now()
	defs, err := p.catalog.GetAgentsForPlatform(ctx, string(p.plat.ID()))
	if err != nil && !opts.TolerateCatalogError {
		return nil, fmt.Errorf("failed to load catalog: %w", err)
	}
	if err != nil {
		defs = nil
	}
	result.AgentDefs = filterDefinitions(defs, opts.AgentIDs)
	result.AgentDefMap = make(map[string]catalog.AgentDef, len(result.AgentDefs))
	for _, def := range result.AgentDefs {
		result.AgentDefMap[def.ID] = def
	}
	slog.DebugContext(ctx, "catalog loaded", "duration", time.Since(catalogStart), "agents", len(result.AgentDefs))

	if !result.UsedDetectionCache {
		detectStart := time.Now()
		if err := p.detect(ctx, opts, started, result); err != nil {
			return nil, err
		}
		slog.DebugContext(ctx, "detection completed", "duration", time.Since(detectStart), "cache_hit", result.UsedDetectionCache, "installations", len(result.Installations))
		needVersions = scoped || !result.UsedDetectionCache || p.versionsExpired(ctx)
		result.NeedsVersionCheck = needVersions
	}
	if opts.SkipVersionCheck || !needVersions {
		return result, nil
	}

	return p.checkVersions(ctx, opts, result, started)
}

// CheckVersions checks an already detected complete snapshot without scanning
// again. The input is copied so a visible UI snapshot is never mutated in place.
func (p *Pipeline) CheckVersions(ctx context.Context, installations []*agent.Installation) (*Result, error) {
	defs, err := p.catalog.GetAgentsForPlatform(ctx, string(p.plat.ID()))
	if err != nil {
		return nil, err
	}
	result := &Result{Installations: make([]*agent.Installation, len(installations)), AgentDefs: defs, AgentDefMap: make(map[string]catalog.AgentDef, len(defs))}
	for i, inst := range installations {
		cp := *inst
		result.Installations[i] = &cp
	}
	for _, def := range defs {
		result.AgentDefMap[def.ID] = def
	}
	return p.checkVersions(ctx, Options{}, result, time.Now())
}

func (p *Pipeline) checkVersions(ctx context.Context, opts Options, result *Result, started time.Time) (*Result, error) {
	scoped := len(opts.AgentIDs) > 0
	fetcher := p.installer
	if p.cfg.Detection.UpdateCheckCacheDuration > 0 {
		fetcher = &cachedLatestVersionFetcher{next: p.installer, store: p.store, ttl: p.cfg.Detection.UpdateCheckCacheDuration}
	}
	versionStart := time.Now()
	result.VersionCheckErrors = versionfetch.CheckLatestVersions(ctx, fetcher, result.Installations, result.AgentDefMap, opts.VersionCheckConcurrency)
	result.RanVersionCheck = true
	result.NeedsVersionCheck = ctx.Err() != nil
	if ctx.Err() == nil && !scoped {
		_ = p.store.SetLastUpdateCheckTime(ctx, time.Now()) //nolint:errcheck // best-effort metadata; TTL also bounds retries for unsupported providers
	}
	if p.cfg.Detection.CacheEnabled {
		if updater, ok := p.store.(storage.DetectionCacheUpdater); ok {
			_ = updater.UpdateDetectionVersions(ctx, result.Installations) //nolint:errcheck // preserve scan timestamp; cache is best-effort
		}
	}
	slog.DebugContext(ctx, "version checks completed", "duration", time.Since(versionStart), "total_duration", time.Since(started), "installations", len(result.Installations))
	return result, nil
}

func (p *Pipeline) versionsExpired(ctx context.Context) bool {
	last, err := p.store.GetLastUpdateCheckTime(ctx)
	return err != nil || last.IsZero() || time.Since(last) >= p.cfg.Detection.UpdateCheckCacheDuration
}

func (p *Pipeline) cachedInstallations(ctx context.Context, newerThan time.Time) ([]*agent.Installation, bool) {
	cached, at, err := p.store.GetDetectionCache(ctx)
	if err != nil || cached == nil || at.IsZero() || time.Since(at) >= p.cfg.Detection.CacheDuration || (!newerThan.IsZero() && at.Before(newerThan)) {
		return nil, false
	}
	// The caller owns the snapshot, including fields mutated during version checks.
	out := make([]*agent.Installation, len(cached))
	for i, inst := range cached {
		cp := *inst
		out[i] = &cp
	}
	return out, true
}

func (p *Pipeline) detect(ctx context.Context, opts Options, started time.Time, result *Result) error {
	detectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	scoped := len(opts.AgentIDs) > 0
	if p.cfg.Detection.CacheEnabled && !scoped {
		if locker, ok := p.store.(storage.RefreshLocker); ok {
			release, err := locker.AcquireRefreshLock(detectCtx, "detection:"+string(p.plat.ID()))
			if err != nil && detectCtx.Err() != nil {
				return fmt.Errorf("waiting for detection: %w", err)
			}
			if err != nil {
				slog.DebugContext(ctx, "inventory coordination unavailable", "error", err)
			} else {
				defer release()
				// A scan completed while this request waited. Forced refreshes may share
				// only a scan newer than their own start, never an older cached snapshot.
				newerThan := time.Time{}
				if opts.ForceRefresh {
					newerThan = started
				}
				if cached, fresh := p.cachedInstallations(detectCtx, newerThan); fresh {
					result.Installations = cached
					result.UsedDetectionCache = true
					return nil
				}
			}
		}
	}
	installations, err := p.detector.DetectAll(detectCtx, result.AgentDefs)
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}
	if err := detectCtx.Err(); err != nil {
		return fmt.Errorf("detection interrupted: %w", err)
	}
	result.Installations = installations
	if p.cfg.Detection.CacheEnabled && !scoped {
		_ = p.store.SaveDetectionCache(ctx, installations) //nolint:errcheck // best-effort inventory snapshot
		// The new snapshot has not had a complete registry check yet.
		_ = p.store.SetLastUpdateCheckTime(ctx, time.Time{}) //nolint:errcheck // best-effort metadata
	}
	return nil
}

func filterDefinitions(defs []catalog.AgentDef, ids []string) []catalog.AgentDef {
	if len(ids) == 0 {
		return defs
	}
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	out := make([]catalog.AgentDef, 0, len(ids))
	for _, def := range defs {
		if wanted[def.ID] {
			out = append(out, def)
		}
	}
	return out
}

func filterInstallations(installations []*agent.Installation, ids []string) []*agent.Installation {
	if len(ids) == 0 {
		return installations
	}
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	out := make([]*agent.Installation, 0, len(ids))
	for _, inst := range installations {
		if wanted[inst.AgentID] {
			out = append(out, inst)
		}
	}
	return out
}
