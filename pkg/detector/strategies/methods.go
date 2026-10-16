package strategies

import "github.com/kevinelliott/agentmanager/pkg/catalog"

// hasInstallMethod avoids inventory subprocesses for unrelated agents.
func hasInstallMethod(agents []catalog.AgentDef, methods ...string) bool {
	for _, def := range agents {
		for _, method := range methods {
			if _, ok := def.InstallMethods[method]; ok {
				return true
			}
		}
	}
	return false
}
