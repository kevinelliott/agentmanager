package cli

import (
	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmgr/internal/tui"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

// NewTUICommand creates the TUI launch command.
func NewTUICommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the terminal user interface",
		Long: `Launch the interactive Terminal User Interface (TUI) for AgentManager.

The TUI provides a full-screen interface for browsing agents, viewing
details, installing, and updating agents with a visual changelog display.

Navigation:
  Tab / Shift+Tab   Switch between tabs
  j/k or arrows     Move up/down in lists
  Enter             Select / Confirm
  ?                 Show help
  q                 Quit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			plat := platform.Current()
			return tui.Run(cfg, plat)
		},
	}
}
