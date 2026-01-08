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

// PipStrategy detects agents installed via pip, pipx, or uv.
type PipStrategy struct {
	platform platform.Platform
}

// NewPipStrategy creates a new pip detection strategy.
func NewPipStrategy(p platform.Platform) *PipStrategy {
	return &PipStrategy{platform: p}
}

// Name returns the strategy name.
func (s *PipStrategy) Name() string {
	return "pip"
}

// Method returns the install method this strategy detects.
func (s *PipStrategy) Method() agent.InstallMethod {
	return agent.MethodPip
}

// IsApplicable returns true if pip, pipx, or uv is available.
func (s *PipStrategy) IsApplicable(p platform.Platform) bool {
	return p.IsExecutableInPath("pip") ||
		p.IsExecutableInPath("pip3") ||
		p.IsExecutableInPath("pipx") ||
		p.IsExecutableInPath("uv")
}

// pipPackage represents a pip package from list output.
type pipPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// pipxPackage represents a pipx package from list output.
type pipxPackage struct {
	Metadata struct {
		MainPackage struct {
			PackageVersion string `json:"package_version"`
		} `json:"main_package"`
	} `json:"metadata"`
}

// Detect scans for pip/pipx/uv installed agents.
func (s *PipStrategy) Detect(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error) {
	var installations []*agent.Installation

	// Get pip packages
	pipPackages := s.getPipPackages(ctx)

	// Get pipx packages
	pipxPackages := s.getPipxPackages(ctx)

	// Get uv packages
	uvPackages := s.getUVPackages(ctx)

	for _, agentDef := range agents {
		// Check pip method
		if pipMethod, ok := agentDef.InstallMethods["pip"]; ok {
			packageName := extractPipPackageName(pipMethod.Package, pipMethod.Command)
			if pkg, found := pipPackages[strings.ToLower(packageName)]; found {
				version, _ := agent.ParseVersion(pkg.Version)
				installations = append(installations, &agent.Installation{
					AgentID:          agentDef.ID,
					AgentName:        agentDef.Name,
					Method:           agent.MethodPip,
					InstalledVersion: version,
					ExecutablePath:   s.findExecutable(agentDef),
					Metadata: map[string]string{
						"detected_by": "pip",
						"package":     packageName,
					},
				})
				continue
			}
		}

		// Check pipx method
		if pipxMethod, ok := agentDef.InstallMethods["pipx"]; ok {
			packageName := extractPipPackageName(pipxMethod.Package, pipxMethod.Command)
			if pkg, found := pipxPackages[strings.ToLower(packageName)]; found {
				version, _ := agent.ParseVersion(pkg.Metadata.MainPackage.PackageVersion)
				installations = append(installations, &agent.Installation{
					AgentID:          agentDef.ID,
					AgentName:        agentDef.Name,
					Method:           agent.MethodPipx,
					InstalledVersion: version,
					ExecutablePath:   s.findExecutable(agentDef),
					Metadata: map[string]string{
						"detected_by": "pipx",
						"package":     packageName,
					},
				})
				continue
			}
		}

		// Check uv method
		if uvMethod, ok := agentDef.InstallMethods["uv"]; ok {
			packageName := extractPipPackageName(uvMethod.Package, uvMethod.Command)
			if pkg, found := uvPackages[strings.ToLower(packageName)]; found {
				version, _ := agent.ParseVersion(pkg.Version)
				installations = append(installations, &agent.Installation{
					AgentID:          agentDef.ID,
					AgentName:        agentDef.Name,
					Method:           agent.MethodUV,
					InstalledVersion: version,
					ExecutablePath:   s.findExecutable(agentDef),
					Metadata: map[string]string{
						"detected_by": "uv",
						"package":     packageName,
					},
				})
			}
		}
	}

	return installations, nil
}

// getPipPackages retrieves pip-installed packages.
func (s *PipStrategy) getPipPackages(ctx context.Context) map[string]pipPackage {
	packages := make(map[string]pipPackage)

	// Try pip3 first, then pip
	pipCmd := "pip3"
	if !s.platform.IsExecutableInPath("pip3") {
		pipCmd = "pip"
	}
	if !s.platform.IsExecutableInPath(pipCmd) {
		return packages
	}

	cmd := exec.CommandContext(ctx, pipCmd, "list", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return packages
	}

	var pkgList []pipPackage
	if err := json.Unmarshal(output, &pkgList); err != nil {
		return packages
	}

	for _, pkg := range pkgList {
		packages[strings.ToLower(pkg.Name)] = pkg
	}

	return packages
}

// getPipxPackages retrieves pipx-installed packages.
func (s *PipStrategy) getPipxPackages(ctx context.Context) map[string]pipxPackage {
	packages := make(map[string]pipxPackage)

	if !s.platform.IsExecutableInPath("pipx") {
		return packages
	}

	cmd := exec.CommandContext(ctx, "pipx", "list", "--json")
	output, err := cmd.Output()
	if err != nil {
		return packages
	}

	var result struct {
		Venvs map[string]pipxPackage `json:"venvs"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return packages
	}

	for name, pkg := range result.Venvs {
		packages[strings.ToLower(name)] = pkg
	}

	return packages
}

// getUVPackages retrieves uv-installed packages.
func (s *PipStrategy) getUVPackages(ctx context.Context) map[string]pipPackage {
	packages := make(map[string]pipPackage)

	if !s.platform.IsExecutableInPath("uv") {
		return packages
	}

	// uv tool list shows installed tools
	cmd := exec.CommandContext(ctx, "uv", "tool", "list", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		// Try alternative format
		cmd = exec.CommandContext(ctx, "uv", "tool", "list")
		output, err = cmd.Output()
		if err != nil {
			return packages
		}
		// Parse text output
		return s.parseUVTextOutput(string(output))
	}

	var tools []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(output, &tools); err != nil {
		return packages
	}

	for _, tool := range tools {
		packages[strings.ToLower(tool.Name)] = pipPackage{
			Name:    tool.Name,
			Version: tool.Version,
		}
	}

	return packages
}

// parseUVTextOutput parses the text output of uv tool list.
func (s *PipStrategy) parseUVTextOutput(output string) map[string]pipPackage {
	packages := make(map[string]pipPackage)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}

		// Format: "package-name v1.2.3" or "package-name 1.2.3"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			version := strings.TrimPrefix(parts[1], "v")
			packages[strings.ToLower(name)] = pipPackage{
				Name:    name,
				Version: version,
			}
		}
	}

	return packages
}

// findExecutable attempts to find the executable for an agent.
func (s *PipStrategy) findExecutable(agentDef catalog.AgentDef) string {
	for _, exec := range agentDef.Detection.Executables {
		if path, err := s.platform.FindExecutable(exec); err == nil {
			return path
		}
	}
	return ""
}

// extractPipPackageName extracts the package name from pip/pipx install.
func extractPipPackageName(packageField, command string) string {
	if packageField != "" {
		return packageField
	}

	// Extract from command
	parts := strings.Fields(command)
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if !strings.HasPrefix(part, "-") && part != "install" && part != "pip" && part != "pipx" && part != "uv" && part != "tool" {
			// Remove version specifier
			if idx := strings.Index(part, "=="); idx > 0 {
				return part[:idx]
			}
			if idx := strings.Index(part, ">="); idx > 0 {
				return part[:idx]
			}
			return part
		}
	}

	return ""
}
