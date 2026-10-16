// Package cli implements the command-line interface for AgentManager.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmanager/internal/cli/output"
	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/config"
)

const (
	githubReleasesURL = "https://api.github.com/repos/kevinelliott/agentmanager/releases/latest"
	downloadBaseURL   = "https://github.com/kevinelliott/agentmanager/releases/download"
)

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
	HTMLURL     string        `json:"html_url"`
}

// GitHubAsset represents a release asset.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// NewUpgradeCommand creates the upgrade command for self-updating.
func NewUpgradeCommand(cfg *config.Config, currentVersion string) *cobra.Command {
	var (
		checkOnly bool
		force     bool
		noColor   bool
	)

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade AgentManager to the latest version",
		Long: `Check for and install the latest version of AgentManager.

This command queries GitHub releases to find the latest version and
can download and install the update automatically.

Examples:
  agentmgr upgrade           # Check and install latest version
  agentmgr upgrade --check   # Check for updates only
  agentmgr upgrade --force   # Force reinstall even if up to date`,
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := output.NewPrinter(cfg, output.NoColor(cfg, noColor))
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// Get current version
			currentVer := strings.TrimPrefix(currentVersion, "v")
			if currentVer == "" || currentVer == "dev" {
				printer.Warning("Running development version, cannot check for updates")
				if !force {
					return nil
				}
			}

			printer.Info("Checking for updates...")

			// Fetch latest release
			release, err := fetchLatestRelease(ctx)
			if err != nil {
				return fmt.Errorf("failed to check for updates: %w", err)
			}

			latestVer := strings.TrimPrefix(release.TagName, "v")

			// Parse and compare versions
			current, err := agent.ParseVersion(currentVer)
			if err != nil && !force {
				printer.Warning("Could not parse current version: %s", currentVer)
			}

			latest, err := agent.ParseVersion(latestVer)
			if err != nil {
				return fmt.Errorf("could not parse latest version: %s", latestVer)
			}

			printer.Print("Current version: %s", currentVer)
			printer.Print("Latest version:  %s", latestVer)
			printer.Println()

			// Check if update is needed
			if !force && current.Compare(latest) >= 0 {
				printer.Success("You are running the latest version!")
				return nil
			}

			if force {
				printer.Info("Force flag set, proceeding with reinstall")
			} else {
				printer.Info("Update available: %s → %s", currentVer, latestVer)
			}

			if checkOnly {
				printer.Println()
				printer.Print("Release notes:")
				printer.Print("--------------")
				// Print first 500 chars of release notes
				notes := release.Body
				if len(notes) > 500 {
					notes = notes[:500] + "..."
				}
				printer.Print(notes)
				printer.Println()
				printer.Info("Run 'agentmgr upgrade' to install")
				return nil
			}

			// Find the right asset for this platform
			assetName := getAssetName()
			var downloadURL string
			for _, asset := range release.Assets {
				if asset.Name == assetName {
					downloadURL = asset.BrowserDownloadURL
					break
				}
			}

			if downloadURL == "" {
				// Try constructed URL
				downloadURL = fmt.Sprintf("%s/%s/%s", downloadBaseURL, release.TagName, assetName)
				printer.Warning("Asset not found in release, trying: %s", assetName)
			}

			printer.Info("Downloading %s...", assetName)

			// Download the new binary
			tmpFile, err := downloadFile(ctx, downloadURL)
			if err != nil {
				return fmt.Errorf("failed to download update: %w", err)
			}
			defer os.Remove(tmpFile)

			// Get current executable path
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}
			execPath, err = filepath.EvalSymlinks(execPath)
			if err != nil {
				return fmt.Errorf("failed to resolve executable path: %w", err)
			}

			printer.Info("Installing to %s...", execPath)

			// Replace the binary
			if err := replaceExecutable(tmpFile, execPath); err != nil {
				return fmt.Errorf("failed to install update: %w", err)
			}

			printer.Success("Successfully upgraded to v%s!", latestVer)
			printer.Info("Release URL: %s", release.HTMLURL)

			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "check for updates only, don't install")
	cmd.Flags().BoolVar(&force, "force", false, "force reinstall even if up to date")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored output")

	return cmd
}

func fetchLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "agentmgr")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

func getAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	ext := ""
	if os == "windows" {
		ext = ".exe"
	}

	return fmt.Sprintf("agentmgr_%s_%s%s", os, arch, ext)
}

func downloadFile(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "agentmgr")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "agentmgr-upgrade-*")
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(tmpFile, resp.Body)
	if err := errors.Join(copyErr, tmpFile.Close()); err != nil {
		return "", errors.Join(err, os.Remove(tmpFile.Name()))
	}

	return tmpFile.Name(), nil
}

func replaceExecutable(src, dst string) (err error) {
	// #nosec G304 -- Reads the private download temporary file produced by the upgrade flow.
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to read new binary: %w", err)
	}
	defer func() { err = errors.Join(err, source.Close()) }()

	// Get permissions of old binary
	info, err := os.Lstat(dst)
	if err != nil {
		return fmt.Errorf("failed to stat old binary: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to replace non-regular executable %s", dst)
	}

	// Keep staging and rollback files in a private, unpredictable directory on
	// the same filesystem. A pre-existing .new symlink must never be followed.
	stagingDir, err := os.MkdirTemp(filepath.Dir(dst), ".agentmgr-upgrade-*")
	if err != nil {
		return fmt.Errorf("create upgrade staging directory: %w", err)
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			if cleanupErr := os.RemoveAll(stagingDir); cleanupErr != nil {
				if err == nil && runtime.GOOS == "windows" {
					// A running Windows executable can be renamed but remain
					// locked against deletion until this process exits.
					slog.Warn("update installed; previous executable retained", "directory", stagingDir, "error", cleanupErr)
				} else {
					err = errors.Join(err, cleanupErr)
				}
			}
		}
	}()
	tmpDst := filepath.Join(stagingDir, "new")
	// #nosec G304 -- Fixed filename inside a newly created owner-only temporary directory.
	target, err := os.OpenFile(tmpDst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create staged executable: %w", err)
	}
	_, copyErr := io.Copy(target, source)
	if err := errors.Join(copyErr, target.Chmod(info.Mode().Perm()), target.Sync(), target.Close()); err != nil {
		return fmt.Errorf("write staged executable: %w", err)
	}

	// Backup old binary
	backupPath := filepath.Join(stagingDir, "previous")
	if err := os.Rename(dst, backupPath); err != nil {
		return fmt.Errorf("failed to backup old binary: %w", err)
	}

	// Move new binary into place
	if err := os.Rename(tmpDst, dst); err != nil {
		if restoreErr := os.Rename(backupPath, dst); restoreErr != nil {
			keepBackup = true
			return fmt.Errorf("install failed: %w; restore failed: %w; old executable retained at %s", err, restoreErr, backupPath)
		}
		return fmt.Errorf("failed to install new binary: %w", err)
	}
	return nil
}
