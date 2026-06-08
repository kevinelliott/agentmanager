package providers

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

// registryLatestVersionTTL bounds process-local latest-version cache entries.
// This keeps repeated update checks in one CLI/helper run from shelling out
// for the same package while still picking up new releases promptly.
const registryLatestVersionTTL = 5 * time.Minute

type latestVersionEntry struct {
	version  agent.Version
	err      error
	cachedAt time.Time
}

func latestVersionEntryFresh(v any) (latestVersionEntry, bool) {
	entry, ok := v.(latestVersionEntry)
	if !ok || time.Since(entry.cachedAt) >= registryLatestVersionTTL {
		return latestVersionEntry{}, false
	}
	return entry, true
}

func loadLatestVersionEntry(cache *sync.Map, key string) (latestVersionEntry, bool) {
	v, ok := cache.Load(key)
	if !ok {
		return latestVersionEntry{}, false
	}
	return latestVersionEntryFresh(v)
}

func shouldCacheLatestVersionError(err error) bool {
	return err == nil || (!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded))
}
