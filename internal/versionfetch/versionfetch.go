// Package versionfetch provides concurrent helpers for checking the latest
// versions of installed agents against their package registries.
package versionfetch

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
)

// DefaultConcurrency is the default number of parallel version-check workers.
const DefaultConcurrency = 8

// LatestVersionFetcher is the subset of installer.Manager used by CheckLatestVersions.
// Accepting an interface keeps the helper decoupled from the concrete manager and
// makes it straightforward to unit test.
type LatestVersionFetcher interface {
	GetLatestVersion(ctx context.Context, method catalog.InstallMethodDef) (agent.Version, error)
}

// CheckLatestVersions fills installations[i].LatestVersion concurrently by
// consulting the registry-appropriate provider via fetcher.GetLatestVersion.
//
// The returned errors slice is parallel to installations: errs[i] holds the
// error (if any) encountered for installations[i]. Installations without a
// matching agent definition or install method are skipped (nil entry).
//
// If concurrency <= 0, DefaultConcurrency is used.
//
// Results are written back into installations by index, so the caller's slice
// retains stable ordering and there are no data races on the slice header.
func CheckLatestVersions(
	ctx context.Context,
	fetcher LatestVersionFetcher,
	installations []*agent.Installation,
	agentDefs map[string]catalog.AgentDef,
	concurrency int,
) []error {
	if len(installations) == 0 {
		return nil
	}
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	errs := make([]error, len(installations))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for i := range installations {
		i := i // capture a fresh copy for the goroutine
		inst := installations[i]
		if inst == nil {
			continue
		}

		agentDef, ok := agentDefs[inst.AgentID]
		if !ok {
			continue
		}

		methodStr := string(inst.Method)
		method, ok := agentDef.InstallMethods[methodStr]
		if !ok {
			continue
		}

		g.Go(func() error {
			// Honour parent context cancellation between dispatches.
			if err := gctx.Err(); err != nil {
				errs[i] = err
				return nil //nolint:nilerr // cancellation is recorded per-index, never aborts the group
			}

			latestVer, err := fetcher.GetLatestVersion(gctx, method)
			if err != nil {
				errs[i] = fmt.Errorf("%s (%s): %w", inst.AgentName, methodStr, err)
				// Do not propagate so a single failure does not cancel sibling workers.
				return nil
			}

			installations[i].LatestVersion = &latestVer
			return nil
		})
	}

	// We never return a non-nil error from g.Go, so Wait will not fail.
	_ = g.Wait()
	return errs
}

// NonNilErrors filters the parallel error slice returned by CheckLatestVersions
// to just the entries that actually failed. Preserves order.
func NonNilErrors(errs []error) []error {
	out := make([]error, 0, len(errs))
	for _, e := range errs {
		if e != nil {
			out = append(out, e)
		}
	}
	return out
}
