// Package cli implements the command-line interface for AgentManager.
package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmgr/internal/cli/output"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/platform"
	"github.com/kevinelliott/agentmgr/pkg/storage"
)

// CheckResult represents the result of a health check.
type CheckResult struct {
	Name    string
	Status  CheckStatus
	Message string
	Fix     string
}

// CheckStatus represents the status of a check.
type CheckStatus int

const (
	CheckOK CheckStatus = iota
	CheckWarning
	CheckError
	CheckSkipped
)

// NewDoctorCommand creates the doctor command for system health checks.
func NewDoctorCommand(cfg *config.Config) *cobra.Command {
	var (
		verbose bool
		noColor bool
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check system health and configuration",
		Long: `Run health checks to verify system requirements, package managers,
catalog accessibility, storage health, and agent configurations.

The doctor command helps identify and diagnose common issues with
your AgentManager installation.

Examples:
  agentmgr doctor              # Run all health checks
  agentmgr doctor --verbose    # Show detailed output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := output.NewPrinter(cfg, noColor)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			printer.Println()
			printer.Print("AgentManager Doctor")
			printer.Print("==================")
			printer.Println()

			var results []CheckResult
			var hasErrors bool

			// System checks
			printer.Print("System")
			printer.Print("------")
			sysResults := runSystemChecks(ctx, verbose)
			results = append(results, sysResults...)
			printResults(printer, sysResults)
			printer.Println()

			// Package manager checks
			printer.Print("Package Managers")
			printer.Print("----------------")
			pmResults := runPackageManagerChecks(ctx, verbose)
			results = append(results, pmResults...)
			printResults(printer, pmResults)
			printer.Println()

			// Storage checks
			printer.Print("Storage")
			printer.Print("-------")
			storeResults := runStorageChecks(ctx, cfg, verbose)
			results = append(results, storeResults...)
			printResults(printer, storeResults)
			printer.Println()

			// Configuration checks
			printer.Print("Configuration")
			printer.Print("-------------")
			cfgResults := runConfigChecks(cfg, verbose)
			results = append(results, cfgResults...)
			printResults(printer, cfgResults)
			printer.Println()

			// Summary
			okCount := 0
			warnCount := 0
			errCount := 0
			skipCount := 0

			for _, r := range results {
				switch r.Status {
				case CheckOK:
					okCount++
				case CheckWarning:
					warnCount++
				case CheckError:
					errCount++
					hasErrors = true
				case CheckSkipped:
					skipCount++
				}
			}

			printer.Print("Summary")
			printer.Print("-------")
			printer.Print("  Passed:   %d", okCount)
			if warnCount > 0 {
				printer.Warning("  Warnings: %d", warnCount)
			}
			if errCount > 0 {
				printer.Error("  Errors:   %d", errCount)
			}
			if skipCount > 0 {
				printer.Print("  Skipped:  %d", skipCount)
			}
			printer.Println()

			if hasErrors {
				printer.Error("Some checks failed. See above for details.")
				return fmt.Errorf("health checks failed")
			}

			if warnCount > 0 {
				printer.Warning("All checks passed with warnings.")
			} else {
				printer.Success("All checks passed!")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show detailed output")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored output")

	return cmd
}

func printResults(printer *output.Printer, results []CheckResult) {
	for _, r := range results {
		switch r.Status {
		case CheckOK:
			printer.Success("%s: %s", r.Name, r.Message)
		case CheckWarning:
			printer.Warning("%s: %s", r.Name, r.Message)
			if r.Fix != "" {
				printer.Print("         Fix: %s", r.Fix)
			}
		case CheckError:
			printer.Error("%s: %s", r.Name, r.Message)
			if r.Fix != "" {
				printer.Print("       Fix: %s", r.Fix)
			}
		case CheckSkipped:
			printer.Info("%s: %s (skipped)", r.Name, r.Message)
		}
	}
}

func runSystemChecks(_ context.Context, _ bool) []CheckResult {
	var results []CheckResult

	// Check Go version
	goVer := runtime.Version()
	results = append(results, CheckResult{
		Name:    "Go Runtime",
		Status:  CheckOK,
		Message: goVer,
	})

	// Check platform
	plat := platform.Current()
	results = append(results, CheckResult{
		Name:    "Platform",
		Status:  CheckOK,
		Message: fmt.Sprintf("%s/%s", plat.ID(), plat.Architecture()),
	})

	// Check shell
	shell := plat.GetShell()
	if shell != "" {
		results = append(results, CheckResult{
			Name:    "Shell",
			Status:  CheckOK,
			Message: shell,
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Shell",
			Status:  CheckWarning,
			Message: "Could not detect shell",
		})
	}

	return results
}

func runPackageManagerChecks(ctx context.Context, _ bool) []CheckResult {
	var results []CheckResult

	packageManagers := []struct {
		name    string
		cmd     string
		args    []string
		install string
	}{
		{"npm", "npm", []string{"--version"}, "Install Node.js from https://nodejs.org/"},
		{"pip", "pip3", []string{"--version"}, "Install Python from https://python.org/"},
		{"pipx", "pipx", []string{"--version"}, "pip install pipx"},
		{"uv", "uv", []string{"--version"}, "curl -LsSf https://astral.sh/uv/install.sh | sh"},
		{"brew", "brew", []string{"--version"}, "Install Homebrew from https://brew.sh/"},
		{"go", "go", []string{"version"}, "Install Go from https://go.dev/"},
		{"cargo", "cargo", []string{"--version"}, "Install Rust from https://rustup.rs/"},
	}

	// Add Windows-specific package managers
	if platform.IsWindows() {
		windowsPMs := []struct {
			name    string
			cmd     string
			args    []string
			install string
		}{
			{"chocolatey", "choco", []string{"--version"}, "Install from https://chocolatey.org/"},
			{"winget", "winget", []string{"--version"}, "Built into Windows 11 or download from Microsoft Store"},
		}
		packageManagers = append(packageManagers, windowsPMs...)
	}

	for _, pm := range packageManagers {
		result := checkCommand(ctx, pm.name, pm.cmd, pm.args, pm.install)
		results = append(results, result)
	}

	return results
}

func checkCommand(ctx context.Context, name, cmd string, args []string, installHint string) CheckResult {
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	execCmd := exec.CommandContext(cmdCtx, cmd, args...)
	out, err := execCmd.CombinedOutput()

	if err != nil {
		return CheckResult{
			Name:    name,
			Status:  CheckWarning,
			Message: "not installed",
			Fix:     installHint,
		}
	}

	version := strings.TrimSpace(string(out))
	// Take first line only
	if idx := strings.Index(version, "\n"); idx != -1 {
		version = version[:idx]
	}
	// Truncate long versions
	if len(version) > 50 {
		version = version[:50] + "..."
	}

	return CheckResult{
		Name:    name,
		Status:  CheckOK,
		Message: version,
	}
}

func runStorageChecks(ctx context.Context, cfg *config.Config, _ bool) []CheckResult {
	var results []CheckResult

	plat := platform.Current()
	dataDir := plat.GetDataDir()

	// Check data directory
	results = append(results, CheckResult{
		Name:    "Data Directory",
		Status:  CheckOK,
		Message: dataDir,
	})

	// Check database
	dbPath := dataDir + "/agentmgr.db"
	store, err := storage.NewSQLiteStore(dataDir)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Database",
			Status:  CheckError,
			Message: fmt.Sprintf("failed to open: %v", err),
			Fix:     "Check file permissions or delete " + dbPath + " to recreate",
		})
		return results
	}
	defer store.Close()

	if err := store.Initialize(ctx); err != nil {
		results = append(results, CheckResult{
			Name:    "Database",
			Status:  CheckError,
			Message: fmt.Sprintf("failed to initialize: %v", err),
			Fix:     "Delete " + dbPath + " and try again",
		})
		return results
	}

	results = append(results, CheckResult{
		Name:    "Database",
		Status:  CheckOK,
		Message: "connected and initialized",
	})

	// Check detection cache
	_, cacheTime, err := store.GetDetectionCache(ctx)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Detection Cache",
			Status:  CheckWarning,
			Message: "no cached data",
			Fix:     "Run 'agentmgr agent list' to populate cache",
		})
	} else if cacheTime.IsZero() {
		results = append(results, CheckResult{
			Name:    "Detection Cache",
			Status:  CheckWarning,
			Message: "cache is empty",
			Fix:     "Run 'agentmgr agent list' to populate cache",
		})
	} else {
		age := time.Since(cacheTime)
		if age > cfg.Detection.CacheDuration {
			results = append(results, CheckResult{
				Name:    "Detection Cache",
				Status:  CheckWarning,
				Message: fmt.Sprintf("stale (age: %s)", age.Round(time.Minute)),
				Fix:     "Run 'agentmgr agent refresh' to update",
			})
		} else {
			results = append(results, CheckResult{
				Name:    "Detection Cache",
				Status:  CheckOK,
				Message: fmt.Sprintf("valid (age: %s)", age.Round(time.Minute)),
			})
		}
	}

	return results
}

func runConfigChecks(cfg *config.Config, _ bool) []CheckResult {
	var results []CheckResult

	// Check config file path
	plat := platform.Current()
	configDir := plat.GetConfigDir()
	results = append(results, CheckResult{
		Name:    "Config Directory",
		Status:  CheckOK,
		Message: configDir,
	})

	// Check catalog URL
	if cfg.Catalog.SourceURL != "" {
		results = append(results, CheckResult{
			Name:    "Catalog URL",
			Status:  CheckOK,
			Message: cfg.Catalog.SourceURL,
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Catalog URL",
			Status:  CheckOK,
			Message: "using embedded catalog",
		})
	}

	// Check cache settings
	if cfg.Detection.CacheEnabled {
		results = append(results, CheckResult{
			Name:    "Detection Cache",
			Status:  CheckOK,
			Message: fmt.Sprintf("enabled (TTL: %s)", cfg.Detection.CacheDuration),
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Detection Cache",
			Status:  CheckWarning,
			Message: "disabled",
			Fix:     "Enable caching for faster agent detection",
		})
	}

	// Check update settings
	if cfg.Updates.AutoCheck {
		results = append(results, CheckResult{
			Name:    "Auto Update Check",
			Status:  CheckOK,
			Message: fmt.Sprintf("enabled (interval: %s)", cfg.Updates.CheckInterval),
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Auto Update Check",
			Status:  CheckWarning,
			Message: "disabled",
		})
	}

	// Check color settings
	if cfg.UI.UseColors {
		results = append(results, CheckResult{
			Name:    "Colors",
			Status:  CheckOK,
			Message: "enabled",
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Colors",
			Status:  CheckOK,
			Message: "disabled",
		})
	}

	return results
}
