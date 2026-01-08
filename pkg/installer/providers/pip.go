package providers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

// PipProvider handles pip/pipx/uv-based installations.
type PipProvider struct {
	platform platform.Platform
}

// NewPipProvider creates a new pip provider.
func NewPipProvider(p platform.Platform) *PipProvider {
	return &PipProvider{platform: p}
}

// Name returns the provider name.
func (p *PipProvider) Name() string {
	return "pip"
}

// Method returns the install method this provider handles.
func (p *PipProvider) Method() agent.InstallMethod {
	return agent.MethodPip
}

// IsAvailable returns true if pip, pipx, or uv is available.
func (p *PipProvider) IsAvailable() bool {
	return p.platform.IsExecutableInPath("pip") ||
		p.platform.IsExecutableInPath("pip3") ||
		p.platform.IsExecutableInPath("pipx") ||
		p.platform.IsExecutableInPath("uv")
}

// Install installs an agent via pip/pipx/uv.
func (p *PipProvider) Install(ctx context.Context, agentDef catalog.AgentDef, method catalog.InstallMethodDef, force bool) (*Result, error) {
	start := time.Now()

	manager, args, packageName, err := p.buildInstallCommand(method, force)
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, manager, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s install failed: %w\n%s", manager, err, stderr.String())
	}

	// Get installed version
	version := p.getInstalledVersion(ctx, manager, packageName)

	// Find executable
	execPath := p.findExecutable(agentDef)

	return &Result{
		AgentID:        agentDef.ID,
		AgentName:      agentDef.Name,
		Method:         p.methodFromManager(manager),
		Version:        version,
		ExecutablePath: execPath,
		Duration:       time.Since(start),
		Output:         stdout.String(),
	}, nil
}

// Update updates a pip/pipx/uv-installed agent.
func (p *PipProvider) Update(ctx context.Context, inst *agent.Installation, agentDef catalog.AgentDef, method catalog.InstallMethodDef) (*Result, error) {
	start := time.Now()

	manager, args, packageName, err := p.buildUpdateCommand(method)
	if err != nil {
		return nil, err
	}

	fromVersion := inst.InstalledVersion

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, manager, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s update failed: %w\n%s", manager, err, stderr.String())
	}

	// Get new version
	toVersion := p.getInstalledVersion(ctx, manager, packageName)

	return &Result{
		AgentID:        agentDef.ID,
		AgentName:      agentDef.Name,
		Method:         p.methodFromManager(manager),
		FromVersion:    fromVersion,
		Version:        toVersion,
		Duration:       time.Since(start),
		Output:         stdout.String(),
		WasUpdated:     toVersion.IsNewerThan(fromVersion),
		ExecutablePath: inst.ExecutablePath,
	}, nil
}

// Uninstall removes a pip/pipx/uv-installed agent.
func (p *PipProvider) Uninstall(ctx context.Context, inst *agent.Installation, method catalog.InstallMethodDef) error {
	manager, args, _, err := p.buildUninstallCommand(method)
	if err != nil {
		return err
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, manager, args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s uninstall failed: %w\n%s", manager, err, stderr.String())
	}

	return nil
}

// buildInstallCommand builds the install command for the appropriate package manager.
func (p *PipProvider) buildInstallCommand(method catalog.InstallMethodDef, force bool) (string, []string, string, error) {
	methodName := method.Method

	packageName := method.Package
	if packageName == "" {
		packageName = extractPipPackage(method.Command)
	}
	if packageName == "" {
		return "", nil, "", fmt.Errorf("could not determine package name")
	}

	switch methodName {
	case "pipx":
		if !p.platform.IsExecutableInPath("pipx") {
			return "", nil, "", fmt.Errorf("pipx is not installed")
		}
		args := []string{"install"}
		if force {
			args = append(args, "--force")
		}
		args = append(args, packageName)
		return "pipx", args, packageName, nil

	case "uv":
		if !p.platform.IsExecutableInPath("uv") {
			return "", nil, "", fmt.Errorf("uv is not installed")
		}
		args := []string{"tool", "install"}
		if force {
			args = append(args, "--force")
		}
		args = append(args, packageName)
		return "uv", args, packageName, nil

	default: // pip
		manager := "pip3"
		if !p.platform.IsExecutableInPath("pip3") {
			manager = "pip"
		}
		if !p.platform.IsExecutableInPath(manager) {
			return "", nil, "", fmt.Errorf("pip is not installed")
		}
		args := []string{"install"}
		if force {
			args = append(args, "--force-reinstall")
		}
		args = append(args, packageName)
		return manager, args, packageName, nil
	}
}

// buildUpdateCommand builds the update command for the appropriate package manager.
func (p *PipProvider) buildUpdateCommand(method catalog.InstallMethodDef) (string, []string, string, error) {
	methodName := method.Method

	packageName := method.Package
	if packageName == "" {
		packageName = extractPipPackage(method.Command)
	}
	if packageName == "" {
		return "", nil, "", fmt.Errorf("could not determine package name")
	}

	switch methodName {
	case "pipx":
		return "pipx", []string{"upgrade", packageName}, packageName, nil

	case "uv":
		return "uv", []string{"tool", "upgrade", packageName}, packageName, nil

	default: // pip
		manager := "pip3"
		if !p.platform.IsExecutableInPath("pip3") {
			manager = "pip"
		}
		return manager, []string{"install", "--upgrade", packageName}, packageName, nil
	}
}

// buildUninstallCommand builds the uninstall command.
func (p *PipProvider) buildUninstallCommand(method catalog.InstallMethodDef) (string, []string, string, error) {
	methodName := method.Method

	packageName := method.Package
	if packageName == "" {
		packageName = extractPipPackage(method.Command)
	}
	if packageName == "" {
		return "", nil, "", fmt.Errorf("could not determine package name")
	}

	switch methodName {
	case "pipx":
		return "pipx", []string{"uninstall", packageName}, packageName, nil

	case "uv":
		return "uv", []string{"tool", "uninstall", packageName}, packageName, nil

	default: // pip
		manager := "pip3"
		if !p.platform.IsExecutableInPath("pip3") {
			manager = "pip"
		}
		return manager, []string{"uninstall", "-y", packageName}, packageName, nil
	}
}

// getInstalledVersion gets the installed version of a package.
func (p *PipProvider) getInstalledVersion(ctx context.Context, manager, packageName string) agent.Version {
	var cmd *exec.Cmd

	switch manager {
	case "pipx":
		cmd = exec.CommandContext(ctx, "pipx", "list", "--json")
	case "uv":
		cmd = exec.CommandContext(ctx, "uv", "tool", "list")
	default:
		cmd = exec.CommandContext(ctx, manager, "show", packageName)
	}

	output, err := cmd.Output()
	if err != nil {
		return agent.Version{}
	}

	// Parse version from output
	versionStr := extractVersionFromPipOutput(string(output), packageName, manager)
	version, _ := agent.ParseVersion(versionStr)
	return version
}

// methodFromManager converts a manager name to an InstallMethod.
func (p *PipProvider) methodFromManager(manager string) agent.InstallMethod {
	switch manager {
	case "pipx":
		return agent.MethodPipx
	case "uv":
		return agent.MethodUV
	default:
		return agent.MethodPip
	}
}

// findExecutable attempts to find the executable for an agent.
func (p *PipProvider) findExecutable(agentDef catalog.AgentDef) string {
	for _, exec := range agentDef.Detection.Executables {
		if path, err := p.platform.FindExecutable(exec); err == nil {
			return path
		}
	}
	return ""
}

// extractPipPackage extracts the package name from a pip install command.
func extractPipPackage(command string) string {
	parts := strings.Fields(command)
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if !strings.HasPrefix(part, "-") && part != "install" && part != "pip" && part != "pip3" && part != "pipx" && part != "uv" && part != "tool" {
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

// extractVersionFromPipOutput extracts version from pip/pipx/uv output.
func extractVersionFromPipOutput(output, packageName, manager string) string {
	lines := strings.Split(output, "\n")

	switch manager {
	case "pipx":
		return ""
	case "uv":
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), strings.ToLower(packageName)) {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return strings.TrimPrefix(parts[1], "v")
				}
			}
		}
	default:
		for _, line := range lines {
			if strings.HasPrefix(line, "Version:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			}
		}
	}

	return ""
}
