package providers

import (
	"context"
	"fmt"

	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// runPostInstall completes required runtime setup after npm or Homebrew changes.
// Commands come from the same trusted catalog as the installation commands.
func runPostInstall(ctx context.Context, plat platform.Platform, method catalog.InstallMethodDef) (string, error) {
	command := method.Metadata["post_install_cmd"]
	if specific, ok := method.Metadata["post_install_cmd_"+string(plat.ID())]; ok {
		command = specific
	}
	if command == "" {
		return "", nil
	}
	output, err := NewNativeProvider(plat).executeCommand(ctx, command)
	if err != nil {
		return output, fmt.Errorf("package installed but post-install setup failed: %w", err)
	}
	return output, nil
}
