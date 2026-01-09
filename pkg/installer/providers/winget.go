package providers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

// semverRegex matches semantic version strings including pre-release and build metadata.
// This pattern is designed to extract versions from within text (e.g., winget output).
// Unlike pkg/agent/version.go's pattern, this doesn't use anchors (^$) to allow extraction.
// Matches: 1.2.3, 1.2.3-beta, 1.2.3-rc.1, 1.2.3+build.123, 1.2.3-beta+build
var semverRegex = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?(?:-[0-9A-Za-z.\-]+)?(?:\+[0-9A-Za-z.\-]+)?`)

// WingetProvider handles winget-based installations on Windows.
type WingetProvider struct {
	platform platform.Platform
}

// NewWingetProvider creates a new winget provider.
func NewWingetProvider(p platform.Platform) *WingetProvider {
	return &WingetProvider{platform: p}
}

// Name returns the provider name.
func (p *WingetProvider) Name() string {
	return "winget"
}

// Method returns the install method this provider handles.
func (p *WingetProvider) Method() agent.InstallMethod {
	return agent.MethodWinget
}

// IsAvailable returns true if winget is available.
func (p *WingetProvider) IsAvailable() bool {
	return p.platform.ID() == platform.Windows && p.platform.IsExecutableInPath("winget")
}

// Install installs an agent via winget.
func (p *WingetProvider) Install(ctx context.Context, agentDef catalog.AgentDef, method catalog.InstallMethodDef, force bool) (*Result, error) {
	start := time.Now()

	packageName := method.Package
	if packageName == "" {
		packageName = extractWingetPackageFromCommand(method.Command)
	}
	if packageName == "" {
		return nil, fmt.Errorf("could not determine winget package name")
	}

	// Build install command
	args := []string{"install", packageName, "--accept-package-agreements", "--accept-source-agreements"}
	if force {
		args = append(args, "--force")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "winget", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("winget install failed: %w\n%s", err, stderr.String())
	}

	// Get installed version
	version := p.getInstalledVersion(ctx, packageName)

	// Find executable (platform's FindExecutable now searches winget packages too)
	execPath := p.findExecutable(agentDef)

	return &Result{
		AgentID:        agentDef.ID,
		AgentName:      agentDef.Name,
		Method:         agent.MethodWinget,
		Version:        version,
		ExecutablePath: execPath,
		Duration:       time.Since(start),
		Output:         stdout.String(),
	}, nil
}

// Update updates a winget-installed agent.
func (p *WingetProvider) Update(ctx context.Context, inst *agent.Installation, agentDef catalog.AgentDef, method catalog.InstallMethodDef) (*Result, error) {
	start := time.Now()

	packageName := method.Package
	if packageName == "" {
		packageName = extractWingetPackageFromCommand(method.Command)
	}
	if packageName == "" {
		return nil, fmt.Errorf("could not determine winget package name")
	}

	fromVersion := inst.InstalledVersion

	// Run upgrade command
	args := []string{"upgrade", packageName, "--accept-package-agreements", "--accept-source-agreements"}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "winget", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// winget upgrade returns error if no updates available
		if !strings.Contains(stdout.String(), "No applicable update found") &&
			!strings.Contains(stderr.String(), "No applicable update found") {
			return nil, fmt.Errorf("winget upgrade failed: %w\n%s", err, stderr.String())
		}
	}

	// Get new version
	toVersion := p.getInstalledVersion(ctx, packageName)

	return &Result{
		AgentID:        agentDef.ID,
		AgentName:      agentDef.Name,
		Method:         agent.MethodWinget,
		FromVersion:    fromVersion,
		Version:        toVersion,
		Duration:       time.Since(start),
		Output:         stdout.String(),
		WasUpdated:     toVersion.IsNewerThan(fromVersion),
		ExecutablePath: inst.ExecutablePath,
	}, nil
}

// Uninstall removes a winget-installed agent.
func (p *WingetProvider) Uninstall(ctx context.Context, inst *agent.Installation, method catalog.InstallMethodDef) error {
	packageName := method.Package
	if packageName == "" {
		packageName = extractWingetPackageFromCommand(method.Command)
	}
	if packageName == "" {
		return fmt.Errorf("could not determine winget package name")
	}

	args := []string{"uninstall", packageName}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "winget", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("winget uninstall failed: %w\n%s", err, stderr.String())
	}

	return nil
}

// getInstalledVersion gets the installed version of a winget package.
func (p *WingetProvider) getInstalledVersion(ctx context.Context, packageName string) agent.Version {
	cmd := exec.CommandContext(ctx, "winget", "list", packageName)
	output, err := cmd.Output()
	if err != nil {
		return agent.Version{}
	}

	// Parse winget list output to find version
	// Output format is typically: Name Id Version Available Source
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, packageName) {
			// Extract version using semantic version regex
			matches := semverRegex.FindAllString(line, -1)
			if len(matches) > 0 {
				// First version match is typically the installed version
				version, _ := agent.ParseVersion(matches[0])
				return version
			}
		}
	}

	return agent.Version{}
}

// findExecutable attempts to find the executable for an agent.
func (p *WingetProvider) findExecutable(agentDef catalog.AgentDef) string {
	for _, exec := range agentDef.Detection.Executables {
		if path, err := p.platform.FindExecutable(exec); err == nil {
			return path
		}
	}
	return ""
}

// extractWingetPackageFromCommand extracts package name from command.
func extractWingetPackageFromCommand(command string) string {
	parts := strings.Fields(command)

	// Look for package name after "install" or "upgrade" keyword
	foundKeyword := false
	for _, part := range parts {
		if foundKeyword && !strings.HasPrefix(part, "-") {
			return part
		}
		if part == "install" || part == "upgrade" || part == "uninstall" {
			foundKeyword = true
		}
	}

	return ""
}
