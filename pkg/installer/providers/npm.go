// Package providers contains installation provider implementations.
package providers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// NPMProvider handles npm-based installations.
type NPMProvider struct {
	platform platform.Platform
}

// NewNPMProvider creates a new NPM provider.
func NewNPMProvider(p platform.Platform) *NPMProvider {
	return &NPMProvider{platform: p}
}

// Name returns the provider name.
func (p *NPMProvider) Name() string {
	return "npm"
}

// Method returns the install method this provider handles.
func (p *NPMProvider) Method() agent.InstallMethod {
	return agent.MethodNPM
}

// IsAvailable returns true if npm is available.
func (p *NPMProvider) IsAvailable() bool {
	return p.platform.IsExecutableInPath("npm")
}

// Install installs an agent via npm.
func (p *NPMProvider) Install(ctx context.Context, agentDef catalog.AgentDef, method catalog.InstallMethodDef, force bool) (*Result, error) {
	start := time.Now()

	packageName := method.Package
	if packageName == "" {
		packageName = extractNPMPackage(method.Command)
	}
	if packageName == "" {
		return nil, fmt.Errorf("could not determine npm package name")
	}

	// Build install command
	args := []string{"install", "-g"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, packageName)

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("npm install failed: %w\n%s%s", err, stderr.String(), formatNPMPermissionHint(stderr.String()))
	}

	// Get installed version
	version := p.getInstalledVersion(ctx, packageName)

	// Find executable
	execPath := p.findExecutable(agentDef)

	return &Result{
		AgentID:        agentDef.ID,
		AgentName:      agentDef.Name,
		Method:         agent.MethodNPM,
		Version:        version,
		ExecutablePath: execPath,
		Duration:       time.Since(start),
		Output:         stdout.String(),
	}, nil
}

// Update updates an npm-installed agent.
func (p *NPMProvider) Update(ctx context.Context, inst *agent.Installation, agentDef catalog.AgentDef, method catalog.InstallMethodDef) (*Result, error) {
	start := time.Now()

	packageName := method.Package
	if packageName == "" {
		packageName = extractNPMPackage(method.Command)
	}
	if packageName == "" {
		return nil, fmt.Errorf("could not determine npm package name")
	}

	fromVersion := inst.InstalledVersion

	// Run update command
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "npm", "update", "-g", packageName)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("npm update failed: %w\n%s%s", err, stderr.String(), formatNPMPermissionHint(stderr.String()))
	}

	// Get new version
	toVersion := p.getInstalledVersion(ctx, packageName)

	return &Result{
		AgentID:        agentDef.ID,
		AgentName:      agentDef.Name,
		Method:         agent.MethodNPM,
		FromVersion:    fromVersion,
		Version:        toVersion,
		Duration:       time.Since(start),
		Output:         stdout.String(),
		WasUpdated:     toVersion.IsNewerThan(fromVersion),
		ExecutablePath: inst.ExecutablePath,
	}, nil
}

// Uninstall removes an npm-installed agent.
func (p *NPMProvider) Uninstall(ctx context.Context, inst *agent.Installation, method catalog.InstallMethodDef) error {
	packageName := method.Package
	if packageName == "" {
		packageName = extractNPMPackage(method.Command)
	}
	if packageName == "" {
		return fmt.Errorf("could not determine npm package name")
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "npm", "uninstall", "-g", packageName)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm uninstall failed: %w\n%s%s", err, stderr.String(), formatNPMPermissionHint(stderr.String()))
	}

	return nil
}

// getInstalledVersion gets the installed version of an npm package.
func (p *NPMProvider) getInstalledVersion(ctx context.Context, packageName string) agent.Version {
	cmd := exec.CommandContext(ctx, "npm", "list", "-g", "--depth=0", packageName)
	output, err := cmd.Output()
	if err != nil {
		return agent.Version{}
	}

	// Parse output to extract version
	// Format: package@version
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, packageName+"@") {
			// Extract version after @
			parts := strings.Split(line, "@")
			if len(parts) >= 2 {
				versionStr := strings.TrimSpace(parts[len(parts)-1])
				version, _ := agent.ParseVersion(versionStr)
				return version
			}
		}
	}

	return agent.Version{}
}

// findExecutable attempts to find the executable for an agent.
func (p *NPMProvider) findExecutable(agentDef catalog.AgentDef) string {
	for _, exec := range agentDef.Detection.Executables {
		if path, err := p.platform.FindExecutable(exec); err == nil {
			return path
		}
	}
	return ""
}

// GetLatestVersion returns the latest version of an npm package from the registry.
func (p *NPMProvider) GetLatestVersion(ctx context.Context, method catalog.InstallMethodDef) (agent.Version, error) {
	packageName := method.Package
	if packageName == "" {
		packageName = extractNPMPackage(method.Command)
	}
	if packageName == "" {
		return agent.Version{}, fmt.Errorf("could not determine npm package name")
	}

	// Use npm view to get the latest version
	cmd := exec.CommandContext(ctx, "npm", "view", packageName, "version")
	output, err := cmd.Output()
	if err != nil {
		return agent.Version{}, fmt.Errorf("npm view failed: %w", err)
	}

	versionStr := strings.TrimSpace(string(output))
	version, err := agent.ParseVersion(versionStr)
	if err != nil {
		return agent.Version{}, fmt.Errorf("failed to parse version %q: %w", versionStr, err)
	}

	return version, nil
}

// extractNPMPackage extracts the package name from an npm install command.
func extractNPMPackage(command string) string {
	parts := strings.Fields(command)
	for i, part := range parts {
		if part == "-g" || part == "--global" {
			for j := i + 1; j < len(parts); j++ {
				if !strings.HasPrefix(parts[j], "-") {
					pkg := parts[j]
					// Remove version specifier
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

// formatNPMPermissionHint returns a helpful hint if the error is a permission issue.
func formatNPMPermissionHint(stderr string) string {
	if !strings.Contains(stderr, "EACCES") {
		return ""
	}

	return `

To fix npm global permission issues, configure npm to use a directory in your home folder:

  mkdir -p ~/.npm-global
  npm config set prefix '~/.npm-global'
  echo 'export PATH=~/.npm-global/bin:$PATH' >> ~/.bashrc
  source ~/.bashrc

Then retry the installation. Alternatively, use a different install method (e.g., --method shell).
`
}

// parseNPMListOutput parses npm list output to extract a package version.
func parseNPMListOutput(output, packageName string) agent.Version {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, packageName+"@") {
			parts := strings.Split(line, "@")
			if len(parts) >= 2 {
				versionStr := strings.TrimSpace(parts[len(parts)-1])
				version, _ := agent.ParseVersion(versionStr)
				return version
			}
		}
	}
	return agent.Version{}
}
