// Package strategies provides detection strategies for different installation methods.
package strategies

import (
	"context"
	"os/exec"
	"regexp"
	"strings"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

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

// Detect scans for installed agents and returns found installations.
func (s *BinaryStrategy) Detect(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error) {
	var installations []*agent.Installation

	for _, agentDef := range agents {
		for _, executable := range agentDef.Detection.Executables {
			// Try to find the executable
			path, err := s.platform.FindExecutable(executable)
			if err != nil {
				continue // Not found, try next executable
			}

			// Get version
			version := s.getVersion(ctx, agentDef, path)

			inst := &agent.Installation{
				AgentID:          agentDef.ID,
				AgentName:        agentDef.Name,
				Method:           agent.MethodNative,
				InstalledVersion: version,
				ExecutablePath:   path,
				Metadata: map[string]string{
					"detected_by": "binary",
					"executable":  executable,
				},
			}

			installations = append(installations, inst)
			break // Found the agent, move to next
		}
	}

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
