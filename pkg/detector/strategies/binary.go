// Package strategies provides detection strategies for different installation methods.
package strategies

import (
	"context"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// binaryDetectConcurrency caps concurrent --version subprocess launches.
// 4 is a reasonable default that avoids fork bombs while still giving real speedup.
const binaryDetectConcurrency = 4

// BinaryStrategy detects agents by scanning PATH for executables.
type BinaryStrategy struct {
	platform platform.Platform
}

// NewBinaryStrategy creates a new binary detection strategy.
func NewBinaryStrategy(p platform.Platform) *BinaryStrategy {
	return &BinaryStrategy{platform: p}
}

// Name returns the strategy name.
func (s *BinaryStrategy) Name() string {
	return "binary"
}

// Method returns the install method this strategy detects.
func (s *BinaryStrategy) Method() agent.InstallMethod {
	return agent.MethodNative
}

// IsApplicable returns true if this strategy can run on the given platform.
func (s *BinaryStrategy) IsApplicable(p platform.Platform) bool {
	return true // Binary detection works on all platforms
}

// binaryMethods are the install method names that represent binary/native installations.
// The order matters - we check in this order and use the first one found in the catalog.
var binaryMethods = []string{"native", "binary", "curl"}

// Detect scans for installed agents and returns found installations.
//
// Matching (LookPath + isPackageManagerPath) runs sequentially so we remain
// deterministic with respect to the input slice order. Version extraction
// (which shells out per match) is parallelized with a bounded semaphore — a
// typical multi-agent run previously did N sequential --version invocations.
func (s *BinaryStrategy) Detect(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error) {
	// Phase 1: synchronously resolve which agents match a binary on PATH and
	// allocate a deterministic slot for each. This avoids racing on LookPath
	// output and preserves the deterministic ordering contract.
	type pending struct {
		agentDef   catalog.AgentDef
		methodName string
		executable string
		path       string
	}

	pendings := make([]pending, 0, len(agents))
	for _, agentDef := range agents {
		// Check if this agent has a binary-based install method defined in the catalog.
		// This mirrors how NPMStrategy checks for "npm" before reporting.
		var methodName string
		for _, m := range binaryMethods {
			if _, ok := agentDef.InstallMethods[m]; ok {
				methodName = m
				break
			}
		}
		if methodName == "" {
			// No binary-based install method defined for this agent, skip it
			continue
		}

		for _, executable := range agentDef.Detection.Executables {
			path, err := s.platform.FindExecutable(executable)
			if err != nil {
				continue // Not found, try next executable
			}
			if isPackageManagerPath(path) {
				continue
			}
			pendings = append(pendings, pending{
				agentDef:   agentDef,
				methodName: methodName,
				executable: executable,
				path:       path,
			})
			break // Found the agent, move to next
		}
	}

	// Phase 2: run version extraction in parallel with a bounded semaphore.
	// Results are written into pre-sized slots so final order matches `pendings`.
	installations := make([]*agent.Installation, len(pendings))
	sem := make(chan struct{}, binaryDetectConcurrency)
	var wg sync.WaitGroup

	for i := range pendings {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, p pending) {
			defer wg.Done()
			defer func() { <-sem }()

			version := s.getVersion(ctx, p.agentDef, p.path)
			installations[idx] = &agent.Installation{
				AgentID:          p.agentDef.ID,
				AgentName:        p.agentDef.Name,
				Method:           agent.InstallMethod(p.methodName),
				InstalledVersion: version,
				ExecutablePath:   p.path,
				Metadata: map[string]string{
					"detected_by": "binary",
					"executable":  p.executable,
				},
			}
		}(i, pendings[i])
	}
	wg.Wait()

	return installations, nil
}

// getVersion extracts the version from the executable.
func (s *BinaryStrategy) getVersion(ctx context.Context, agentDef catalog.AgentDef, path string) agent.Version {
	if agentDef.Detection.VersionCmd == "" {
		return agent.Version{}
	}

	// Parse the version command
	parts := strings.Fields(agentDef.Detection.VersionCmd)
	if len(parts) == 0 {
		return agent.Version{}
	}

	// Replace the executable name with the full path
	parts[0] = path

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return agent.Version{}
	}

	// Extract version using regex if provided
	versionStr := strings.TrimSpace(string(output))
	if agentDef.Detection.VersionRegex != "" {
		re, err := regexp.Compile(agentDef.Detection.VersionRegex)
		if err == nil {
			matches := re.FindStringSubmatch(versionStr)
			if len(matches) > 1 {
				versionStr = matches[1]
			}
		}
	} else {
		// Try common patterns
		versionStr = extractVersionFromOutput(versionStr)
	}

	version, _ := agent.ParseVersion(versionStr)
	return version
}

// extractVersionFromOutput tries to extract a version number from command output.
func extractVersionFromOutput(output string) string {
	// Common version patterns
	patterns := []string{
		`v?(\d+\.\d+\.\d+(?:-[a-zA-Z0-9.]+)?)`,
		`version\s+v?(\d+\.\d+\.\d+)`,
		`(\d+\.\d+\.\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// isPackageManagerPath checks if a path belongs to a package manager installation.
// This helps avoid reporting npm/pip/brew/etc. installations as "native".
func isPackageManagerPath(path string) bool {
	// Normalize path separators for comparison
	normalizedPath := strings.ToLower(path)
	if runtime.GOOS == "windows" {
		normalizedPath = strings.ReplaceAll(normalizedPath, "\\", "/")
	}

	// Package manager path patterns to exclude from native detection
	packageManagerPatterns := []string{
		// npm/node paths
		"/node_modules/",
		"/npm/",
		"/node/",
		"/.npm/",
		"/pnpm/",
		"/yarn/",
		"/.bun/",
		"/fnm/",
		"/.nvm/",
		"/.volta/",
		"/asdf/installs/nodejs/",
		"/mise/installs/node/",

		// Python paths
		"/pip/",
		"/pipx/",
		"/site-packages/",
		"/.local/pipx/",
		"/.pyenv/",
		"/conda/",
		"/virtualenv/",
		"/venv/",
		"/.venv/",
		"/uv/",
		"/asdf/installs/python/",
		"/mise/installs/python/",

		// Homebrew paths
		"/homebrew/",
		"/cellar/",
		"/linuxbrew/",

		// Go paths
		"/go/bin/",
		"/gopath/",

		// Rust/Cargo paths
		"/.cargo/",

		// Ruby paths
		"/.gem/",
		"/.rbenv/",
		"/.rvm/",
		"/asdf/installs/ruby/",

		// Generic version managers
		"/.asdf/",
		"/mise/",
		"/rtx/",

		// Scoop (Windows)
		"/scoop/",
	}

	for _, pattern := range packageManagerPatterns {
		if strings.Contains(normalizedPath, pattern) {
			return true
		}
	}

	return false
}
