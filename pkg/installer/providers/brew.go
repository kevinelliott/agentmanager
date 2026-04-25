package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// brewLatestVersionTTL bounds how long a cached `brew info --json=v2` result
// is considered fresh. 5 minutes is a compromise between avoiding repeated
// forks during a single agent run and still picking up new releases promptly.
const brewLatestVersionTTL = 5 * time.Minute

// brewLatestVersionCache memoizes GetLatestVersion results process-wide.
// Key format: "<cask?>:<package>" — see brewCacheKey.
var (
	brewLatestVersionCache sync.Map // key -> brewLatestEntry
	brewLatestVersionOnce  sync.Map // key -> *sync.Once (dedup concurrent lookups)
)

type brewLatestEntry struct {
	version  agent.Version
	err      error
	cachedAt time.Time
}

func brewCacheKey(pkg string, isCask bool) string {
	if isCask {
		return "cask:" + pkg
	}
	return "formula:" + pkg
}

// BrewProvider handles Homebrew-based installations.
type BrewProvider struct {
	platform platform.Platform
}

// NewBrewProvider creates a new Homebrew provider.
func NewBrewProvider(p platform.Platform) *BrewProvider {
	return &BrewProvider{platform: p}
}

// Name returns the provider name.
func (p *BrewProvider) Name() string {
	return "brew"
}

// Method returns the install method this provider handles.
func (p *BrewProvider) Method() agent.InstallMethod {
	return agent.MethodBrew
}

// IsAvailable returns true if brew is available.
func (p *BrewProvider) IsAvailable() bool {
	return p.platform.ID() != platform.Windows && p.platform.IsExecutableInPath("brew")
}

// Install installs an agent via Homebrew.
func (p *BrewProvider) Install(ctx context.Context, agentDef catalog.AgentDef, method catalog.InstallMethodDef, force bool) (*Result, error) {
	start := time.Now()

	packageName, isCask := p.parseBrewPackage(method)
	if packageName == "" {
		return nil, fmt.Errorf("could not determine brew package name")
	}

	// Build install command
	args := []string{"install"}
	if isCask {
		args = append(args, "--cask")
	}
	if force {
		args = append(args, "--force")
	}
	args = append(args, packageName)

	var stdout, stderr bytes.Buffer
	progress := ProgressWriter(ctx)
	cmd := exec.CommandContext(ctx, "brew", args...)
	cmd.Stdout = io.MultiWriter(&stdout, progress)
	cmd.Stderr = io.MultiWriter(&stderr, progress)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("brew install failed: %w\n%s%s", err, stderr.String(), FormatInstallError("brew", "install", stderr.String()))
	}

	// Get installed version
	version := p.getInstalledVersion(ctx, packageName, isCask)

	// Find executable
	execPath := p.findExecutable(agentDef)

	return &Result{
		AgentID:        agentDef.ID,
		AgentName:      agentDef.Name,
		Method:         agent.MethodBrew,
		Version:        version,
		ExecutablePath: execPath,
		Duration:       time.Since(start),
		Output:         stdout.String(),
	}, nil
}

// Update updates a Homebrew-installed agent.
func (p *BrewProvider) Update(ctx context.Context, inst *agent.Installation, agentDef catalog.AgentDef, method catalog.InstallMethodDef) (*Result, error) {
	start := time.Now()

	packageName, isCask := p.parseBrewPackage(method)
	if packageName == "" {
		return nil, fmt.Errorf("could not determine brew package name")
	}

	fromVersion := inst.InstalledVersion

	// Run upgrade command
	args := []string{"upgrade"}
	if isCask {
		args = append(args, "--cask")
	}
	args = append(args, packageName)

	var stdout, stderr bytes.Buffer
	progress := ProgressWriter(ctx)
	cmd := exec.CommandContext(ctx, "brew", args...)
	cmd.Stdout = io.MultiWriter(&stdout, progress)
	cmd.Stderr = io.MultiWriter(&stderr, progress)

	if err := cmd.Run(); err != nil {
		// brew upgrade returns error if already up to date
		if !strings.Contains(stderr.String(), "already installed") {
			return nil, fmt.Errorf("brew upgrade failed: %w\n%s%s", err, stderr.String(), FormatInstallError("brew", "upgrade", stderr.String()))
		}
	}

	// Get new version
	toVersion := p.getInstalledVersion(ctx, packageName, isCask)

	return &Result{
		AgentID:        agentDef.ID,
		AgentName:      agentDef.Name,
		Method:         agent.MethodBrew,
		FromVersion:    fromVersion,
		Version:        toVersion,
		Duration:       time.Since(start),
		Output:         stdout.String(),
		WasUpdated:     toVersion.IsNewerThan(fromVersion),
		ExecutablePath: inst.ExecutablePath,
	}, nil
}

// Uninstall removes a Homebrew-installed agent.
func (p *BrewProvider) Uninstall(ctx context.Context, inst *agent.Installation, method catalog.InstallMethodDef) error {
	packageName, isCask := p.parseBrewPackage(method)
	if packageName == "" {
		return fmt.Errorf("could not determine brew package name")
	}

	args := []string{"uninstall"}
	if isCask {
		args = append(args, "--cask")
	}
	args = append(args, packageName)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "brew", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew uninstall failed: %w\n%s", err, stderr.String())
	}

	return nil
}

// parseBrewPackage extracts the package name and determines if it's a cask.
func (p *BrewProvider) parseBrewPackage(method catalog.InstallMethodDef) (string, bool) {
	packageName := method.Package
	isCask := false

	// Check metadata for cask indicator
	if method.Metadata != nil {
		if method.Metadata["type"] == "cask" {
			isCask = true
		}
	}

	if packageName == "" {
		// Extract from command
		packageName, isCask = extractBrewPackageFromCommand(method.Command)
	}

	return packageName, isCask
}

// getInstalledVersion gets the installed version of a brew package.
func (p *BrewProvider) getInstalledVersion(ctx context.Context, packageName string, isCask bool) agent.Version {
	args := []string{"info", "--json=v2"}
	if isCask {
		args = append(args, "--cask")
	}
	args = append(args, packageName)

	cmd := exec.CommandContext(ctx, "brew", args...)
	output, err := cmd.Output()
	if err != nil {
		return agent.Version{}
	}

	var result struct {
		Formulae []struct {
			Installed []struct {
				Version string `json:"version"`
			} `json:"installed"`
		} `json:"formulae"`
		Casks []struct {
			Installed string `json:"installed"`
		} `json:"casks"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return agent.Version{}
	}

	var versionStr string
	if isCask && len(result.Casks) > 0 {
		versionStr = result.Casks[0].Installed
	} else if len(result.Formulae) > 0 && len(result.Formulae[0].Installed) > 0 {
		versionStr = result.Formulae[0].Installed[0].Version
	}

	version, _ := agent.ParseVersion(versionStr)
	return version
}

// GetLatestVersion returns the latest version of a brew package.
//
// Results are cached process-wide with a short TTL to avoid re-running
// `brew info --json=v2 <pkg>` once per agent per refresh. Concurrent callers
// for the same package coalesce via a per-key sync.Once so only one subprocess
// is launched at a time.
func (p *BrewProvider) GetLatestVersion(ctx context.Context, method catalog.InstallMethodDef) (agent.Version, error) {
	packageName, isCask := p.parseBrewPackage(method)
	if packageName == "" {
		return agent.Version{}, fmt.Errorf("could not determine brew package name")
	}

	key := brewCacheKey(packageName, isCask)

	// Fast path: recently cached value.
	if v, ok := brewLatestVersionCache.Load(key); ok {
		if entry, ok := v.(brewLatestEntry); ok && time.Since(entry.cachedAt) < brewLatestVersionTTL {
			return entry.version, entry.err
		}
	}

	// Coalesce concurrent lookups for the same key.
	onceI, _ := brewLatestVersionOnce.LoadOrStore(key, &sync.Once{})
	once, ok := onceI.(*sync.Once)
	if !ok {
		// Fallback — recompute directly rather than risk a nil-pointer panic.
		return p.fetchLatestVersionUncached(ctx, packageName, isCask)
	}
	once.Do(func() {
		version, err := p.fetchLatestVersionUncached(ctx, packageName, isCask)
		// Don't cache caller-specific context errors (canceled, deadline
		// exceeded). A later caller with a healthy ctx would otherwise
		// receive the first caller's transient failure for the full TTL.
		// Leaving the cache empty and clearing the Once lets the next call
		// refetch with its own ctx.
		if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			brewLatestVersionOnce.Delete(key)
			return
		}
		brewLatestVersionCache.Store(key, brewLatestEntry{
			version:  version,
			err:      err,
			cachedAt: time.Now(),
		})
		// Reset the once so a future TTL-expired call can refetch.
		brewLatestVersionOnce.Delete(key)
	})

	// After Do returns, the cache entry is guaranteed to be populated (either
	// by this goroutine or a concurrent one that used the same Once).
	if v, ok := brewLatestVersionCache.Load(key); ok {
		if entry, ok := v.(brewLatestEntry); ok {
			return entry.version, entry.err
		}
	}
	// Extremely unlikely fallback — recompute directly.
	return p.fetchLatestVersionUncached(ctx, packageName, isCask)
}

// fetchLatestVersionUncached performs the actual `brew info` subprocess call.
func (p *BrewProvider) fetchLatestVersionUncached(ctx context.Context, packageName string, isCask bool) (agent.Version, error) {
	args := []string{"info", "--json=v2"}
	if isCask {
		args = append(args, "--cask")
	}
	args = append(args, packageName)

	cmd := exec.CommandContext(ctx, "brew", args...)
	output, err := cmd.Output()
	if err != nil {
		return agent.Version{}, fmt.Errorf("brew info failed: %w", err)
	}

	var result struct {
		Formulae []struct {
			Versions struct {
				Stable string `json:"stable"`
			} `json:"versions"`
		} `json:"formulae"`
		Casks []struct {
			Version string `json:"version"`
		} `json:"casks"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return agent.Version{}, fmt.Errorf("failed to parse brew info: %w", err)
	}

	var versionStr string
	if isCask && len(result.Casks) > 0 {
		versionStr = result.Casks[0].Version
	} else if len(result.Formulae) > 0 {
		versionStr = result.Formulae[0].Versions.Stable
	}

	if versionStr == "" {
		return agent.Version{}, fmt.Errorf("no version found for %s", packageName)
	}

	version, err := agent.ParseVersion(versionStr)
	if err != nil {
		return agent.Version{}, err
	}

	return version, nil
}

// findExecutable attempts to find the executable for an agent.
func (p *BrewProvider) findExecutable(agentDef catalog.AgentDef) string {
	for _, exec := range agentDef.Detection.Executables {
		if path, err := p.platform.FindExecutable(exec); err == nil {
			return path
		}
	}
	return ""
}

// extractBrewPackageFromCommand extracts package name and cask flag from command.
func extractBrewPackageFromCommand(command string) (string, bool) {
	parts := strings.Fields(command)
	isCask := false

	for _, part := range parts {
		if part == "--cask" || part == "cask" {
			isCask = true
		}
	}

	// Get package name (last non-flag argument)
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if !strings.HasPrefix(part, "-") && part != "install" && part != "brew" && part != "cask" {
			// Handle tap format: user/tap/package -> package
			if strings.Contains(part, "/") {
				segments := strings.Split(part, "/")
				// Check if it's homebrew/cask/... format
				if len(segments) >= 2 && segments[1] == "cask" {
					isCask = true
				}
				return segments[len(segments)-1], isCask
			}
			return part, isCask
		}
	}

	return "", isCask
}

// parseBrewInfoJSON parses brew info JSON output to extract installed version.
func parseBrewInfoJSON(output []byte, isCask bool) agent.Version {
	var result struct {
		Formulae []struct {
			Installed []struct {
				Version string `json:"version"`
			} `json:"installed"`
		} `json:"formulae"`
		Casks []struct {
			Installed string `json:"installed"`
		} `json:"casks"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return agent.Version{}
	}

	var versionStr string
	if isCask && len(result.Casks) > 0 {
		versionStr = result.Casks[0].Installed
	} else if len(result.Formulae) > 0 && len(result.Formulae[0].Installed) > 0 {
		versionStr = result.Formulae[0].Installed[0].Version
	}

	version, _ := agent.ParseVersion(versionStr)
	return version
}

// parseBrewLatestVersionJSON parses brew info JSON output to extract latest version.
func parseBrewLatestVersionJSON(output []byte, isCask bool) (agent.Version, error) {
	var result struct {
		Formulae []struct {
			Versions struct {
				Stable string `json:"stable"`
			} `json:"versions"`
		} `json:"formulae"`
		Casks []struct {
			Version string `json:"version"`
		} `json:"casks"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return agent.Version{}, fmt.Errorf("failed to parse brew info: %w", err)
	}

	var versionStr string
	if isCask && len(result.Casks) > 0 {
		versionStr = result.Casks[0].Version
	} else if len(result.Formulae) > 0 {
		versionStr = result.Formulae[0].Versions.Stable
	}

	if versionStr == "" {
		return agent.Version{}, fmt.Errorf("no version found")
	}

	version, err := agent.ParseVersion(versionStr)
	if err != nil {
		return agent.Version{}, err
	}

	return version, nil
}
