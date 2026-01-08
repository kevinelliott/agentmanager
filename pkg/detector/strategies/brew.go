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

// BrewStrategy detects agents installed via Homebrew.
type BrewStrategy struct {
	platform platform.Platform
}

// NewBrewStrategy creates a new Homebrew detection strategy.
func NewBrewStrategy(p platform.Platform) *BrewStrategy {
	return &BrewStrategy{platform: p}
}

// Name returns the strategy name.
func (s *BrewStrategy) Name() string {
	return "brew"
}

// Method returns the install method this strategy detects.
func (s *BrewStrategy) Method() agent.InstallMethod {
	return agent.MethodBrew
}

// IsApplicable returns true if brew is available (macOS/Linux).
func (s *BrewStrategy) IsApplicable(p platform.Platform) bool {
	return p.ID() != platform.Windows && p.IsExecutableInPath("brew")
}

// brewFormula represents a Homebrew formula from list output.
type brewFormula struct {
	Name              string   `json:"name"`
	FullName          string   `json:"full_name"`
	InstalledVersions []string `json:"installed"`
	Versions          struct {
		Stable string `json:"stable"`
	} `json:"versions"`
}

// brewCask represents a Homebrew cask from list output.
type brewCask struct {
	Token            string   `json:"token"`
	Name             []string `json:"name"`
	InstalledVersion string   `json:"installed"`
	Version          string   `json:"version"`
}

// Detect scans for brew-installed agents.
func (s *BrewStrategy) Detect(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error) {
	var installations []*agent.Installation

	// Get installed formulae
	formulae := s.getInstalledFormulae(ctx)

	// Get installed casks
	casks := s.getInstalledCasks(ctx)

	for _, agentDef := range agents {
		// Check brew method
		brewMethod, hasBrew := agentDef.InstallMethods["brew"]
		if !hasBrew {
			continue
		}

		packageName := extractBrewPackageName(brewMethod.Package, brewMethod.Command)
		if packageName == "" {
			continue
		}

		// Check if it's a formula
		if formula, found := formulae[strings.ToLower(packageName)]; found {
			var versionStr string
			if len(formula.InstalledVersions) > 0 {
				versionStr = formula.InstalledVersions[0]
			}
			version, _ := agent.ParseVersion(versionStr)

			installations = append(installations, &agent.Installation{
				AgentID:          agentDef.ID,
				AgentName:        agentDef.Name,
				Method:           agent.MethodBrew,
				InstalledVersion: version,
				ExecutablePath:   s.findExecutable(agentDef),
				Metadata: map[string]string{
					"detected_by":  "brew",
					"package":      packageName,
					"package_type": "formula",
				},
			})
			continue
		}

		// Check if it's a cask
		if cask, found := casks[strings.ToLower(packageName)]; found {
			version, _ := agent.ParseVersion(cask.InstalledVersion)

			installations = append(installations, &agent.Installation{
				AgentID:          agentDef.ID,
				AgentName:        agentDef.Name,
				Method:           agent.MethodBrew,
				InstalledVersion: version,
				ExecutablePath:   s.findExecutable(agentDef),
				Metadata: map[string]string{
					"detected_by":  "brew",
					"package":      packageName,
					"package_type": "cask",
				},
			})
		}
	}

	return installations, nil
}

// getInstalledFormulae retrieves installed Homebrew formulae.
func (s *BrewStrategy) getInstalledFormulae(ctx context.Context) map[string]brewFormula {
	formulae := make(map[string]brewFormula)

	cmd := exec.CommandContext(ctx, "brew", "info", "--installed", "--json=v2")
	output, err := cmd.Output()
	if err != nil {
		return formulae
	}

	var result struct {
		Formulae []brewFormula `json:"formulae"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return formulae
	}

	for _, f := range result.Formulae {
		formulae[strings.ToLower(f.Name)] = f
		if f.FullName != "" && f.FullName != f.Name {
			formulae[strings.ToLower(f.FullName)] = f
		}
	}

	return formulae
}

// getInstalledCasks retrieves installed Homebrew casks.
func (s *BrewStrategy) getInstalledCasks(ctx context.Context) map[string]brewCask {
	casks := make(map[string]brewCask)

	cmd := exec.CommandContext(ctx, "brew", "info", "--cask", "--installed", "--json=v2")
	output, err := cmd.Output()
	if err != nil {
		return casks
	}

	var result struct {
		Casks []brewCask `json:"casks"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return casks
	}

	for _, c := range result.Casks {
		casks[strings.ToLower(c.Token)] = c
	}

	return casks
}

// findExecutable attempts to find the executable for an agent.
func (s *BrewStrategy) findExecutable(agentDef catalog.AgentDef) string {
	for _, exec := range agentDef.Detection.Executables {
		if path, err := s.platform.FindExecutable(exec); err == nil {
			return path
		}
	}
	return ""
}

// extractBrewPackageName extracts the package name from a brew install command.
func extractBrewPackageName(packageField, command string) string {
	if packageField != "" {
		return packageField
	}

	// Extract from command
	// Common patterns:
	// brew install package
	// brew install --cask package
	// brew install user/tap/package
	parts := strings.Fields(command)
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if !strings.HasPrefix(part, "-") && part != "install" && part != "brew" {
			// Handle tap format: user/tap/package -> package
			if strings.Contains(part, "/") {
				segments := strings.Split(part, "/")
				return segments[len(segments)-1]
			}
			return part
		}
	}

	return ""
}
