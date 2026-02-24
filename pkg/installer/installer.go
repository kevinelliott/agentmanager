// Package installer provides agent installation, update, and uninstall capabilities.
package installer

import (
	"context"
	"fmt"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/installer/providers"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// Manager orchestrates installation operations.
type Manager struct {
	npm    *providers.NPMProvider
	pip    *providers.PipProvider
	brew   *providers.BrewProvider
	native *providers.NativeProvider
	plat   platform.Platform
}

// NewManager creates a new installation manager.
func NewManager(p platform.Platform) *Manager {
	return &Manager{
		npm:    providers.NewNPMProvider(p),
		pip:    providers.NewPipProvider(p),
		brew:   providers.NewBrewProvider(p),
		native: providers.NewNativeProvider(p),
		plat:   p,
	}
}

// Install installs an agent using the specified method.
func (m *Manager) Install(ctx context.Context, agentDef catalog.AgentDef, method catalog.InstallMethodDef, force bool) (*providers.Result, error) {
	switch method.Method {
	case "npm":
		if !m.npm.IsAvailable() {
			return nil, fmt.Errorf("npm is not available")
		}
		return m.npm.Install(ctx, agentDef, method, force)

	case "pip", "pipx", "uv":
		if !m.pip.IsAvailable() {
			return nil, fmt.Errorf("pip/pipx/uv is not available")
		}
		return m.pip.Install(ctx, agentDef, method, force)

	case "brew", "brew-cask":
		if !m.brew.IsAvailable() {
			return nil, fmt.Errorf("brew is not available")
		}
		return m.brew.Install(ctx, agentDef, method, force)

	case "native", "curl", "binary", "bun", "bunx", "cargo", "go", "scoop", "chocolatey", "powershell", "winget", "dmg", "krew", "nix", "git":
		return m.native.Install(ctx, agentDef, method, force)

	default:
		return nil, fmt.Errorf("unsupported install method: %s", method.Method)
	}
}

// Update updates an installed agent.
func (m *Manager) Update(ctx context.Context, inst *agent.Installation, agentDef catalog.AgentDef, method catalog.InstallMethodDef) (*providers.Result, error) {
	switch method.Method {
	case "npm", "bun", "bunx":
		if method.Method == "npm" && !m.npm.IsAvailable() {
			return nil, fmt.Errorf("npm is not available")
		}
		if (method.Method == "bun" || method.Method == "bunx") && !m.plat.IsExecutableInPath("bun") {
			return nil, fmt.Errorf("bun is not available")
		}
		// bun/bunx use npm-compatible commands, but if they have explicit update_cmd, use native
		if (method.Method == "bun" || method.Method == "bunx") && method.UpdateCmd != "" {
			return m.native.Update(ctx, inst, agentDef, method)
		}
		return m.npm.Update(ctx, inst, agentDef, method)

	case "pip", "pipx", "uv":
		if !m.pip.IsAvailable() {
			return nil, fmt.Errorf("pip/pipx/uv is not available")
		}
		return m.pip.Update(ctx, inst, agentDef, method)

	case "brew", "brew-cask":
		if !m.brew.IsAvailable() {
			return nil, fmt.Errorf("brew is not available")
		}
		return m.brew.Update(ctx, inst, agentDef, method)

	case "native", "curl", "binary", "cargo", "go", "scoop", "chocolatey", "powershell", "winget", "dmg", "krew", "nix", "git":
		// All of these use their update_cmd or command directly via the native provider
		return m.native.Update(ctx, inst, agentDef, method)

	default:
		return nil, fmt.Errorf("unsupported install method: %s", method.Method)
	}
}

// Uninstall removes an installed agent.
func (m *Manager) Uninstall(ctx context.Context, inst *agent.Installation, method catalog.InstallMethodDef) error {
	switch method.Method {
	case "npm":
		if !m.npm.IsAvailable() {
			return fmt.Errorf("npm is not available")
		}
		return m.npm.Uninstall(ctx, inst, method)

	case "pip", "pipx", "uv":
		if !m.pip.IsAvailable() {
			return fmt.Errorf("pip/pipx/uv is not available")
		}
		return m.pip.Uninstall(ctx, inst, method)

	case "brew", "brew-cask":
		if !m.brew.IsAvailable() {
			return fmt.Errorf("brew is not available")
		}
		return m.brew.Uninstall(ctx, inst, method)

	case "native", "curl", "binary", "bun", "bunx", "cargo", "go", "scoop", "chocolatey", "powershell", "winget", "dmg", "krew", "nix", "git":
		return m.native.Uninstall(ctx, inst, method)

	default:
		return fmt.Errorf("unsupported install method: %s", method.Method)
	}
}

// GetAvailableMethods returns the installation methods available for an agent on this platform.
func (m *Manager) GetAvailableMethods(agentDef catalog.AgentDef) []catalog.InstallMethodDef {
	platformID := string(m.plat.ID())
	var methods []catalog.InstallMethodDef

	for _, method := range agentDef.InstallMethods {
		// Check if platform is supported
		supported := false
		for _, p := range method.Platforms {
			if p == platformID {
				supported = true
				break
			}
		}
		if !supported {
			continue
		}

		// Check if provider is available
		available := false
		switch method.Method {
		case "npm":
			available = m.npm.IsAvailable()
		case "pip", "pipx", "uv":
			available = m.pip.IsAvailable()
		case "brew", "brew-cask":
			available = m.brew.IsAvailable()
		case "native", "curl", "binary", "bun", "bunx", "cargo", "go", "scoop", "chocolatey", "powershell", "winget", "dmg", "krew", "nix", "git":
			available = true
		}

		if available {
			methods = append(methods, method)
		}
	}

	return methods
}

// IsMethodAvailable checks if a specific install method is available on this system.
func (m *Manager) IsMethodAvailable(method string) bool {
	switch method {
	case "npm":
		return m.npm.IsAvailable()
	case "pip", "pipx", "uv":
		return m.pip.IsAvailable()
	case "brew", "brew-cask":
		return m.brew.IsAvailable()
	case "native", "curl", "binary", "bun", "bunx", "cargo", "go", "scoop", "chocolatey", "powershell", "winget", "dmg", "krew", "nix", "git":
		return true
	default:
		return false
	}
}

// GetLatestVersion returns the latest version available for an agent using the specified method.
func (m *Manager) GetLatestVersion(ctx context.Context, method catalog.InstallMethodDef) (agent.Version, error) {
	switch method.Method {
	case "npm":
		if !m.npm.IsAvailable() {
			return agent.Version{}, fmt.Errorf("npm is not available")
		}
		return m.npm.GetLatestVersion(ctx, method)

	case "pip", "pipx", "uv":
		if !m.pip.IsAvailable() {
			return agent.Version{}, fmt.Errorf("pip/pipx/uv is not available")
		}
		return m.pip.GetLatestVersion(ctx, method)

	case "brew", "brew-cask":
		if !m.brew.IsAvailable() {
			return agent.Version{}, fmt.Errorf("brew is not available")
		}
		return m.brew.GetLatestVersion(ctx, method)

	case "native", "curl", "binary", "bun", "bunx", "cargo", "go", "scoop", "chocolatey", "powershell", "winget", "dmg", "krew", "nix", "git":
		// These methods don't have a standard registry to check
		return agent.Version{}, fmt.Errorf("version checking not supported for %s", method.Method)

	default:
		return agent.Version{}, fmt.Errorf("unsupported install method: %s", method.Method)
	}
}
