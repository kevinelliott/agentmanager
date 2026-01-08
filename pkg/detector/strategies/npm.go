package strategies

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

// NPMStrategy detects agents installed via npm.
type NPMStrategy struct {
	platform platform.Platform
}

// NewNPMStrategy creates a new NPM detection strategy.
func NewNPMStrategy(p platform.Platform) *NPMStrategy {
	return &NPMStrategy{platform: p}
}

// Name returns the strategy name.
func (s *NPMStrategy) Name() string {
	return "npm"
}

// Method returns the install method this strategy detects.
func (s *NPMStrategy) Method() agent.InstallMethod {
	return agent.MethodNPM
}

// IsApplicable returns true if npm is available.
func (s *NPMStrategy) IsApplicable(p platform.Platform) bool {
	return p.IsExecutableInPath("npm")
}

// npmPackage represents an npm package from list output.
type npmPackage struct {
	Version string `json:"version"`
}

// npmListOutput represents the output of npm list --json.
type npmListOutput struct {
	Dependencies map[string]npmPackage `json:"dependencies"`
}

// Detect scans for npm-installed agents.
func (s *NPMStrategy) Detect(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error) {
	// Get list of globally installed packages
	globalPackages, err := s.getGlobalPackages(ctx)
	if err != nil {
		return nil, err
	}

	var installations []*agent.Installation

	for _, agentDef := range agents {
		// Check if this agent has npm as an install method
		npmMethod, hasNPM := agentDef.InstallMethods["npm"]
		if !hasNPM {
			continue
		}

		// Get the package name
		packageName := npmMethod.Package
		if packageName == "" {
			// Try to extract from command
			packageName = extractNPMPackageName(npmMethod.Command)
		}
		if packageName == "" {
			continue
		}

		// Check if the package is installed
		pkg, found := globalPackages[packageName]
		if !found {
			continue
		}

		// Parse version
		version, _ := agent.ParseVersion(pkg.Version)

		// Get executable path
		execPath := s.findExecutable(agentDef)

		inst := &agent.Installation{
			AgentID:          agentDef.ID,
			AgentName:        agentDef.Name,
			Method:           agent.MethodNPM,
			InstalledVersion: version,
			ExecutablePath:   execPath,
			Metadata: map[string]string{
				"detected_by": "npm",
				"package":     packageName,
			},
		}

		installations = append(installations, inst)
	}

	return installations, nil
}

// getGlobalPackages retrieves globally installed npm packages.
func (s *NPMStrategy) getGlobalPackages(ctx context.Context) (map[string]npmPackage, error) {
	cmd := exec.CommandContext(ctx, "npm", "list", "-g", "--depth=0", "--json")
	output, err := cmd.Output()
	if err != nil {
		// npm list returns exit code 1 if there are peer dependency issues
		// but still outputs valid JSON, so we continue
		if len(output) == 0 {
			return nil, err
		}
	}

	var result npmListOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	return result.Dependencies, nil
}

// findExecutable attempts to find the executable for an agent.
func (s *NPMStrategy) findExecutable(agentDef catalog.AgentDef) string {
	for _, exec := range agentDef.Detection.Executables {
		if path, err := s.platform.FindExecutable(exec); err == nil {
			return path
		}
	}
	return ""
}

// extractNPMPackageName extracts the package name from an npm install command.
func extractNPMPackageName(command string) string {
	// Common patterns:
	// npm install -g @scope/package
	// npm i -g package
	parts := strings.Fields(command)
	for i, part := range parts {
		if part == "-g" || part == "--global" {
			// The next non-flag argument should be the package name
			for j := i + 1; j < len(parts); j++ {
				if !strings.HasPrefix(parts[j], "-") {
					// Remove any version specifier
					pkg := parts[j]
					if idx := strings.LastIndex(pkg, "@"); idx > 0 {
						pkg = pkg[:idx]
					}
					return pkg
				}
			}
		}
	}
	return ""
}
