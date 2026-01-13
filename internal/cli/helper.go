package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/ipc"
	"github.com/kevinelliott/agentmgr/pkg/platform"
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
			// Check if already running
			if isHelperRunning() {
				printInfo("Helper is already running")
				return nil
			}

			// Find the helper binary
			helperPath, err := findHelperBinary()
			if err != nil {
				return fmt.Errorf("could not find helper binary: %w", err)
			}

			if foreground {
				fmt.Println("Starting helper in foreground...")
				c := exec.Command(helperPath)
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				return c.Run()
			}

			fmt.Println("Starting helper in background...")

			// Check if this is a macOS .app bundle
			isAppBundle := strings.HasSuffix(helperPath, ".app")

			var c *exec.Cmd
			if isAppBundle {
				// Use 'open' command for .app bundles on macOS
				c = exec.Command("open", helperPath)
			} else {
				// Start helper as background process
				c = exec.Command(helperPath)
				c.Stdout = nil
				c.Stderr = nil
				c.Stdin = nil
			}

			if err := c.Start(); err != nil {
				return fmt.Errorf("failed to start helper: %w", err)
			}

			if isAppBundle {
				// For .app bundles, 'open' returns immediately
				// Wait briefly for it to complete
				_ = c.Wait() //nolint:errcheck // 'open' returns immediately for .app bundles; exit status irrelevant
				printSuccess("Helper started")
			} else {
				// Save PID before releasing (Release invalidates the Process)
				pid := c.Process.Pid

				// Detach from parent
				if err := c.Process.Release(); err != nil {
					return fmt.Errorf("failed to detach helper: %w", err)
				}

				printSuccess("Helper started (PID: %d)", pid)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&foreground, "foreground", "F", false, "run in foreground (don't daemonize)")

	return cmd
}

func newHelperStopCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the systray helper",
		Long:  `Stop the running AgentManager systray helper process.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isHelperRunning() {
				printInfo("Helper is not running")
				return nil
			}

			fmt.Println("Stopping helper...")

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Try to send shutdown message via IPC
			client := ipc.NewClient("")
			if err := client.Connect(ctx); err != nil {
				return fmt.Errorf("failed to connect to helper: %w", err)
			}
			defer func() { _ = client.Disconnect() }()

			msg, err := ipc.NewMessage(ipc.MessageTypeShutdown, nil)
			if err != nil {
				return fmt.Errorf("failed to create shutdown message: %w", err)
			}

			_, err = client.Send(ctx, msg)
			if err != nil {
				// It's okay if we don't get a response - helper may shutdown before responding
				printWarning("Helper may have shutdown before acknowledging: %v", err)
			}

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
			plat := platform.Current()

			fmt.Println("Helper Status:")

			// Check if running
			running := isHelperRunning()
			if running {
				fmt.Println("  Running: yes")

				// Try to get detailed status via IPC
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				client := ipc.NewClient("")
				if err := client.Connect(ctx); err != nil {
					fmt.Printf("  (IPC connect error: %v)\n", err)
				} else {
					defer func() { _ = client.Disconnect() }()

					msg, err := ipc.NewMessage(ipc.MessageTypeGetStatus, nil)
					if err != nil {
						fmt.Printf("  (IPC message error: %v)\n", err)
					} else {
						resp, err := client.Send(ctx, msg)
						if err != nil {
							fmt.Printf("  (IPC send error: %v)\n", err)
						} else if resp != nil {
							var status ipc.StatusResponse
							if err := resp.DecodePayload(&status); err != nil {
								fmt.Printf("  (IPC decode error: %v)\n", err)
							} else {
								fmt.Printf("  PID: %d\n", status.PID)
								fmt.Printf("  Uptime: %s\n", formatDuration(time.Duration(status.Uptime)*time.Second))
								fmt.Printf("  Agents: %d\n", status.AgentCount)
								fmt.Printf("  Updates available: %d\n", status.UpdatesAvailable)
								if !status.LastCatalogRefresh.IsZero() {
									fmt.Printf("  Last catalog refresh: %s\n", status.LastCatalogRefresh.Format(time.RFC3339))
								}
							}
						}
					}
				}
			} else {
				fmt.Println("  Running: no")
			}

			// Check auto-start
			autoStart, err := plat.IsAutoStartEnabled(context.Background())
			if err != nil {
				fmt.Println("  Auto-start: unknown")
			} else if autoStart {
				fmt.Println("  Auto-start: enabled")
			} else {
				fmt.Println("  Auto-start: disabled")
			}

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
				plat := platform.Current()

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				if err := plat.EnableAutoStart(ctx); err != nil {
					return fmt.Errorf("failed to enable auto-start: %w", err)
				}

				printSuccess("Auto-start enabled")
				return nil
			},
		},
		&cobra.Command{
			Use:   "disable",
			Short: "Disable auto-start",
			RunE: func(cmd *cobra.Command, args []string) error {
				plat := platform.Current()

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				if err := plat.DisableAutoStart(ctx); err != nil {
					return fmt.Errorf("failed to disable auto-start: %w", err)
				}

				printSuccess("Auto-start disabled")
				return nil
			},
		},
	)

	return cmd
}

// isHelperRunning checks if the helper process is running by attempting to connect via IPC.
func isHelperRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client := ipc.NewClient("")
	if err := client.Connect(ctx); err != nil {
		return false
	}
	_ = client.Disconnect()
	return true
}

// findHelperBinary locates the agentmgr-helper binary or .app bundle.
// On macOS, it prefers the .app bundle for proper systray support.
func findHelperBinary() (string, error) {
	plat := platform.Current()
	isMacOS := plat.ID() == platform.Darwin

	// First, check in the same directory as the current executable
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)

		// On macOS, prefer .app bundle
		if isMacOS {
			appPath := filepath.Join(dir, "AgentManager Helper.app")
			if _, err := os.Stat(appPath); err == nil {
				return appPath, nil
			}
		}

		helperPath := filepath.Join(dir, "agentmgr-helper")
		if _, err := os.Stat(helperPath); err == nil {
			return helperPath, nil
		}
	}

	// Check common paths
	var paths []string

	if isMacOS {
		// Check for .app bundle in Applications and common locations
		paths = append(paths,
			"/Applications/AgentManager Helper.app",
			filepath.Join(os.Getenv("HOME"), "Applications", "AgentManager Helper.app"),
		)
	}

	paths = append(paths,
		"/usr/local/bin/agentmgr-helper",
		"/usr/bin/agentmgr-helper",
	)

	// Add home directory paths
	if home, err := os.UserHomeDir(); err == nil {
		if isMacOS {
			paths = append(paths,
				filepath.Join(home, ".local", "bin", "AgentManager Helper.app"),
			)
		}
		paths = append(paths,
			filepath.Join(home, ".local", "bin", "agentmgr-helper"),
			filepath.Join(home, "go", "bin", "agentmgr-helper"),
		)
	}

	// Check each path
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Check PATH for bare binary
	if path, err := exec.LookPath("agentmgr-helper"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("agentmgr-helper not found in PATH or common locations")
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}
