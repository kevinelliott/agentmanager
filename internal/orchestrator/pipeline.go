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

// DetectAndCheckVersions runs the shared detect-and-version-check pipeline.
//
// Semantics (preserved from the original call sites):
//
//   - If the detection cache is enabled, not stale, and ForceRefresh is false,
//     the cached installations are used and the version check only runs when
//     the update-check TTL has expired.
//   - Otherwise the detector runs fresh. When ForceRefresh is true the cache
//     is also cleared so a follow-up cold start does not see a stale entry.
//   - After a version check the refreshed installations (with LatestVersion
//     filled) are written back to the cache and the last-update-check
//     timestamp is advanced. Cache-write errors are intentionally swallowed
//     because they are non-fatal and match existing behavior.
//   - A catalog fetch error is fatal by default. Callers that need to
//     continue with no definitions (e.g. the systray on a flaky network)
//     can set Options.TolerateCatalogError. Version-check errors are
//     captured per index and returned to the caller, never aborting the
//     pipeline.
func (p *Pipeline) DetectAndCheckVersions(ctx context.Context, opts Options) (*Result, error) {
	agentDefs, err := p.catalog.GetAgentsForPlatform(ctx, string(p.plat.ID()))
	if err != nil {
		if !opts.TolerateCatalogError {
			return nil, fmt.Errorf("failed to load catalog: %w", err)
		}
		// Fall through with a nil slice; downstream handles that safely.
		agentDefs = nil
	}

	agentDefMap := make(map[string]catalog.AgentDef, len(agentDefs))
	for _, def := range agentDefs {
		agentDefMap[def.ID] = def
	}

	var (
		installations      []*agent.Installation
		usedDetectionCache bool
		needUpdateCheck    bool
	)

	// Try to use the detection cache unless we're forced to refresh.
	if p.cfg.Detection.CacheEnabled && !opts.ForceRefresh {
		cached, cachedAt, cacheErr := p.store.GetDetectionCache(ctx)
		if cacheErr == nil && cached != nil && time.Since(cachedAt) < p.cfg.Detection.CacheDuration {
			installations = cached
			usedDetectionCache = true

			// Still refresh latest-version metadata if the update-check TTL
			// has elapsed since the last check.
			lastCheck, lastErr := p.store.GetLastUpdateCheckTime(ctx)
			if lastErr != nil || lastCheck.IsZero() || time.Since(lastCheck) >= p.cfg.Detection.UpdateCheckCacheDuration {
				needUpdateCheck = true
			}
		}
	}

	// Cold detect path (cache disabled, empty, expired, or forced).
	if !usedDetectionCache {
		if opts.ForceRefresh {
			//nolint:errcheck // best-effort cache clear; stale entries will be overwritten below
			_ = p.store.ClearDetectionCache(ctx)
		}

		installations, err = p.detector.DetectAll(ctx, agentDefs)
		if err != nil {
			return nil, fmt.Errorf("detection failed: %w", err)
		}
		// A fresh detection always warrants a version check.
		needUpdateCheck = true
	}

	result := &Result{
		Installations:      installations,
		AgentDefs:          agentDefs,
		AgentDefMap:        agentDefMap,
		UsedDetectionCache: usedDetectionCache,
	}

	if opts.SkipVersionCheck || !needUpdateCheck {
		return result, nil
	}

	concurrency := opts.VersionCheckConcurrency
	if concurrency <= 0 {
		concurrency = versionfetch.DefaultConcurrency
	}

	errs := versionfetch.CheckLatestVersions(ctx, p.installer, installations, agentDefMap, concurrency)
	result.RanVersionCheck = true
	result.VersionCheckErrors = errs

	// Persist the refreshed snapshot. Both writes are best-effort to preserve
	// the silent-failure semantics the previous hand-rolled pipelines relied on.
	//nolint:errcheck // best-effort timestamp; cached data is still usable without it
	_ = p.store.SetLastUpdateCheckTime(ctx, time.Now())
	if p.cfg.Detection.CacheEnabled {
		//nolint:errcheck // best-effort cache; agents will be redetected on the next run
		_ = p.store.SaveDetectionCache(ctx, installations)
	}

	return result, nil
}
