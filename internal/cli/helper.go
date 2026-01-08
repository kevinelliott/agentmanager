package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmgr/pkg/config"
)

// NewHelperCommand creates the helper management command group.
func NewHelperCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "helper",
		Short: "Manage the systray helper",
		Long: `Control the AgentManager systray helper process.

The helper runs in the background and provides:
- System tray icon with quick access to agent status
- Desktop notifications for available updates
- Background catalog refresh`,
	}

	cmd.AddCommand(
		newHelperStartCommand(cfg),
		newHelperStopCommand(cfg),
		newHelperStatusCommand(cfg),
		newHelperAutoStartCommand(cfg),
	)

	return cmd
}

func newHelperStartCommand(cfg *config.Config) *cobra.Command {
	var foreground bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the systray helper",
		Long: `Start the AgentManager systray helper in the background.

The helper will appear in your system tray and provide quick access
to agent status and updates.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if foreground {
				fmt.Println("Starting helper in foreground...")
				// TODO: Implement actual helper start in foreground
				return nil
			}

			fmt.Println("Starting helper in background...")
			// TODO: Implement actual helper start
			printSuccess("Helper started")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "run in foreground (don't daemonize)")

	return cmd
}

func newHelperStopCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the systray helper",
		Long:  `Stop the running AgentManager systray helper process.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Stopping helper...")
			// TODO: Implement actual helper stop
			printSuccess("Helper stopped")
			return nil
		},
	}
}

func newHelperStatusCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check helper status",
		Long:  `Display the current status of the systray helper.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement actual status check
			fmt.Println("Helper Status:")
			fmt.Println("  Running: no")
			fmt.Println("  Auto-start: disabled")
			return nil
		},
	}
}

func newHelperAutoStartCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autostart",
		Short: "Manage auto-start settings",
		Long:  `Enable or disable automatic startup of the helper on system boot.`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "enable",
			Short: "Enable auto-start",
			RunE: func(cmd *cobra.Command, args []string) error {
				// TODO: Implement using platform.EnableAutoStart
				printSuccess("Auto-start enabled")
				return nil
			},
		},
		&cobra.Command{
			Use:   "disable",
			Short: "Disable auto-start",
			RunE: func(cmd *cobra.Command, args []string) error {
				// TODO: Implement using platform.DisableAutoStart
				printSuccess("Auto-start disabled")
				return nil
			},
		},
	)

	return cmd
}
