package providers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

// NativeProvider handles native binary installations (shell scripts, direct downloads).
type NativeProvider struct {
	platform platform.Platform
}

// NewNativeProvider creates a new native provider.
func NewNativeProvider(p platform.Platform) *NativeProvider {
	return &NativeProvider{platform: p}
}

// Name returns the provider name.
func (p *NativeProvider) Name() string {
	return "native"
}

// Method returns the install method this provider handles.
func (p *NativeProvider) Method() agent.InstallMethod {
	return agent.MethodNative
}

// IsAvailable returns true - native install is always available.
func (p *NativeProvider) IsAvailable() bool {
	return true
}

// Install installs an agent via native method.
func (p *NativeProvider) Install(ctx context.Context, agentDef catalog.AgentDef, method catalog.InstallMethodDef, force bool) (*Result, error) {
	start := time.Now()

	command := method.Command
	if command == "" {
		return nil, fmt.Errorf("no install command specified")
	}

	// Execute the install command
	output, err := p.executeCommand(ctx, command)
	if err != nil {
		return nil, fmt.Errorf("native install failed: %w", err)
	}

	// Get installed version
	version := p.getInstalledVersion(ctx, agentDef)

	// Find executable
	execPath := p.findExecutable(agentDef)

	return &Result{
		AgentID:        agentDef.ID,
		AgentName:      agentDef.Name,
		Method:         agent.MethodNative,
		Version:        version,
		ExecutablePath: execPath,
		Duration:       time.Since(start),
		Output:         output,
	}, nil
}

// Update updates a native-installed agent.
func (p *NativeProvider) Update(ctx context.Context, inst *agent.Installation, agentDef catalog.AgentDef, method catalog.InstallMethodDef) (*Result, error) {
	start := time.Now()

	command := method.UpdateCmd
	if command == "" {
		// Fall back to running the install command again
		command = method.Command
	}
	if command == "" {
		return nil, fmt.Errorf("no update command specified")
	}

	fromVersion := inst.InstalledVersion

	// Execute the update command
	output, err := p.executeCommand(ctx, command)
	if err != nil {
		return nil, fmt.Errorf("native update failed: %w", err)
	}

	// Get new version
	toVersion := p.getInstalledVersion(ctx, agentDef)

	return &Result{
		AgentID:        agentDef.ID,
		AgentName:      agentDef.Name,
		Method:         agent.MethodNative,
		FromVersion:    fromVersion,
		Version:        toVersion,
		Duration:       time.Since(start),
		Output:         output,
		WasUpdated:     toVersion.IsNewerThan(fromVersion),
		ExecutablePath: inst.ExecutablePath,
	}, nil
}

// Uninstall removes a native-installed agent.
func (p *NativeProvider) Uninstall(ctx context.Context, inst *agent.Installation, method catalog.InstallMethodDef) error {
	command := method.UninstallCmd
	if command == "" {
		// Try to find and remove the executable
		execPath := inst.ExecutablePath
		if execPath == "" {
			return fmt.Errorf("no uninstall command and no executable path")
		}

		// Remove the executable
		if err := os.Remove(execPath); err != nil {
			return fmt.Errorf("failed to remove executable: %w", err)
		}
		return nil
	}

	// Execute the uninstall command
	_, err := p.executeCommand(ctx, command)
	if err != nil {
		return fmt.Errorf("native uninstall failed: %w", err)
	}

	return nil
}

// executeCommand runs a shell command.
func (p *NativeProvider) executeCommand(ctx context.Context, command string) (string, error) {
	shell := p.platform.GetShell()
	shellArg := p.platform.GetShellArg()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, shell, shellArg, command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w\n%s", err, stderr.String())
	}

	return stdout.String(), nil
}

// getInstalledVersion gets the installed version of an agent.
func (p *NativeProvider) getInstalledVersion(ctx context.Context, agentDef catalog.AgentDef) agent.Version {
	if agentDef.Detection.VersionCmd == "" {
		return agent.Version{}
	}

	shell := p.platform.GetShell()
	shellArg := p.platform.GetShellArg()

	cmd := exec.CommandContext(ctx, shell, shellArg, agentDef.Detection.VersionCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return agent.Version{}
	}

	versionStr := strings.TrimSpace(string(output))

	// Try common patterns
	versionStr = extractVersionString(versionStr)

	version, _ := agent.ParseVersion(versionStr)
	return version
}

// findExecutable attempts to find the executable for an agent.
func (p *NativeProvider) findExecutable(agentDef catalog.AgentDef) string {
	for _, exec := range agentDef.Detection.Executables {
		if path, err := p.platform.FindExecutable(exec); err == nil {
			return path
		}
	}
	return ""
}

// extractVersionString extracts a version string from output.
func extractVersionString(output string) string {
	// Look for common version patterns
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for version-like patterns
		if strings.Contains(strings.ToLower(line), "version") {
			// Extract the version number
			parts := strings.Fields(line)
			for _, part := range parts {
				if len(part) > 0 && (part[0] >= '0' && part[0] <= '9' || part[0] == 'v') {
					return strings.TrimPrefix(part, "v")
				}
			}
		}
	}

	// Just try to find any version-like string
	parts := strings.Fields(output)
	for _, part := range parts {
		if len(part) > 0 && (part[0] >= '0' && part[0] <= '9') {
			return part
		}
	}

	return ""
}
