package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

type cachedLatestVersionFetcher struct {
	next  VersionFetcher
	store storage.Store
	ttl   time.Duration
}

type latestVersionCacheRecord struct {
	Version  string    `json:"version"`
	CachedAt time.Time `json:"cached_at"`
}

func (f *cachedLatestVersionFetcher) GetLatestVersion(ctx context.Context, method catalog.InstallMethodDef) (agent.Version, error) {
	if f.store == nil || f.ttl <= 0 {
		return f.next.GetLatestVersion(ctx, method)
	}

	key := latestVersionCacheKey(method)
	if namespaced, ok := f.next.(interface {
		VersionCacheNamespace(catalog.InstallMethodDef) string
	}); ok && key != "" {
		if namespace := namespaced.VersionCacheNamespace(method); namespace != "" {
			key += ":" + namespace
		}
	}
	if key != "" {
		if version, ok := f.get(ctx, key); ok {
			return version, nil
		}
	}

	if key != "" {
		if locker, ok := f.store.(storage.RefreshLocker); ok {
			release, err := locker.AcquireRefreshLock(ctx, key)
			if err != nil && ctx.Err() != nil {
				return agent.Version{}, ctx.Err()
			}
			if err != nil {
				slog.DebugContext(ctx, "registry coordination unavailable", "error", err)
			} else {
				defer release()
				if version, fresh := f.get(ctx, key); fresh {
					return version, nil
				}
			}
		}
	}

	version, err := f.next.GetLatestVersion(ctx, method)
	if err == nil && key != "" {
		f.set(ctx, key, version)
	}
	return version, err
}

func (f *cachedLatestVersionFetcher) get(ctx context.Context, key string) (agent.Version, bool) {
	value, err := f.store.GetSetting(ctx, key)
	if err != nil || value == "" {
		return agent.Version{}, false
	}

	var record latestVersionCacheRecord
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		return agent.Version{}, false
	}
	if record.CachedAt.IsZero() || time.Since(record.CachedAt) >= f.ttl {
		return agent.Version{}, false
	}

	version, err := agent.ParseVersion(record.Version)
	if err != nil {
		return agent.Version{}, false
	}
	return version, true
}

func (f *cachedLatestVersionFetcher) set(ctx context.Context, key string, version agent.Version) {
	data, err := json.Marshal(latestVersionCacheRecord{
		Version:  version.String(),
		CachedAt: time.Now(),
	})
	if err != nil {
		return
	}
	//nolint:errcheck // best-effort cache write; a miss just means the next call refetches
	_ = f.store.SetSetting(ctx, key, string(data))
}

func latestVersionCacheKey(method catalog.InstallMethodDef) string {
	provider := latestVersionCacheProvider(method.Method)
	pkg := strings.TrimSpace(method.Package)
	if pkg == "" {
		pkg = strings.TrimSpace(method.Command)
	}
	if provider == "" || pkg == "" {
		return ""
	}
	return "latest_version_cache:" + provider + ":" + strings.ToLower(pkg)
}

func latestVersionCacheProvider(method string) string {
	switch method {
	case "pip", "pipx", "uv":
		return "pypi"
	case "brew", "brew-cask":
		return method
	default:
		return method
	}
}
