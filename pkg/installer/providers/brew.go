package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

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
	cmd := exec.CommandContext(ctx, "brew", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("brew install failed: %w\n%s", err, stderr.String())
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
	cmd := exec.CommandContext(ctx, "brew", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// brew upgrade returns error if already up to date
		if !strings.Contains(stderr.String(), "already installed") {
			return nil, fmt.Errorf("brew upgrade failed: %w\n%s", err, stderr.String())
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
				return segments[len(segments)-1], isCask
			}
			return part, isCask
		}
	}

	return "", isCask
}
