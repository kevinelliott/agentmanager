// Package cli implements the command-line interface for AgentManager.
package cli

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmanager/internal/cli/output"
	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/config"
)

// githubReleasesURL is a variable so tests can point it at a local server.
var githubReleasesURL = "https://api.github.com/repos/kevinelliott/agentmanager/releases/latest"

const (
	checksumsAssetName = "checksums.txt"
	// maxBinarySize bounds what is extracted from a release archive.
	maxBinarySize = 512 << 20
)

// gitDescribeSuffix matches what "git describe" appends to a build made
// after a tag, such as "-34-gb10cffc" or "-34-gb10cffc-dirty".
var gitDescribeSuffix = regexp.MustCompile(`-\d+-g[0-9a-f]+(-dirty)?$`)

// releaseVersion reduces a build version to the release it was built from
// or after. A build made 34 commits after v1.5.0 is not older than 1.5.0,
// even though "1.5.0-34-gabc" reads as a prerelease of it.
func releaseVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimSuffix(v, "-dirty")
	return gitDescribeSuffix.ReplaceAllString(v, "")
}

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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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
			current, err := agent.ParseVersion(releaseVersion(currentVer))
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

			// Get current executable path
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}
			execPath, err = filepath.EvalSymlinks(execPath)
			if err != nil {
				return fmt.Errorf("failed to resolve executable path: %w", err)
			}
			if hint := managedInstallHint(execPath); hint != "" {
				return errors.New(hint)
			}

			tmpFile, err := fetchReleaseBinary(ctx, printer, release, latestVer)
			if err != nil {
				return err
			}
			defer os.Remove(tmpFile)

			printer.Info("Installing to %s...", execPath)

			// Replace the binary
			if err := replaceExecutable(tmpFile, execPath); err != nil {
				if errors.Is(err, os.ErrPermission) {
					return fmt.Errorf("no permission to write %s; run 'sudo agentmgr upgrade'", filepath.Dir(execPath))
				}
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

// fetchReleaseBinary downloads this platform's archive from the release,
// verifies it against the release checksums, and returns the path of the
// extracted CLI binary. The caller removes that file.
func fetchReleaseBinary(ctx context.Context, printer *output.Printer, release *GitHubRelease, latestVer string) (string, error) {
	// Find the archive for this platform, and the checksum list.
	assetName := getAssetName(latestVer)
	var downloadURL, checksumsURL string
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			downloadURL = asset.BrowserDownloadURL
		case checksumsAssetName:
			checksumsURL = asset.BrowserDownloadURL
		}
	}
	if downloadURL == "" {
		return "", fmt.Errorf("release %s has no build for %s/%s (looked for %s); see %s",
			release.TagName, runtime.GOOS, runtime.GOARCH, assetName, release.HTMLURL)
	}
	if checksumsURL == "" {
		return "", fmt.Errorf("release %s has no %s, so the download cannot be verified", release.TagName, checksumsAssetName)
	}

	printer.Info("Downloading %s...", assetName)

	archivePath, err := downloadFile(ctx, downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download update: %w", err)
	}
	defer os.Remove(archivePath)

	checksumsPath, err := downloadFile(ctx, checksumsURL)
	if err != nil {
		return "", fmt.Errorf("failed to download checksums: %w", err)
	}
	defer os.Remove(checksumsPath)

	if err := verifyChecksum(archivePath, checksumsPath, assetName); err != nil {
		return "", fmt.Errorf("failed to verify update: %w", err)
	}
	printer.Info("Checksum verified")

	tmpFile, err := extractBinary(archivePath, assetName)
	if err != nil {
		return "", fmt.Errorf("failed to unpack update: %w", err)
	}
	return tmpFile, nil
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

// getAssetName is the release archive for this platform, as named by the
// archives section of .goreleaser.yml.
func getAssetName(version string) string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("agentmanager_%s_%s_%s%s", version, runtime.GOOS, runtime.GOARCH, ext)
}

// binaryName is the CLI's file name inside a release archive.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "agentmgr.exe"
	}
	return "agentmgr"
}

// managedInstallHint returns advice when the running binary belongs to
// something else that should do the upgrade, or "" when it can be replaced.
func managedInstallHint(execPath string) string {
	slashed := filepath.ToSlash(execPath)
	switch {
	case strings.Contains(slashed, "/Cellar/"):
		return "this agentmgr was installed with Homebrew; run 'brew upgrade agentmanager' instead"
	case strings.Contains(slashed, ".app/Contents/MacOS/"):
		return "this agentmgr is part of AgentManager.app; download the new app from https://github.com/kevinelliott/agentmanager/releases/latest instead"
	}
	return ""
}

// verifyChecksum checks the archive's SHA-256 against the release's
// checksums file, which has "<hex digest>  <file name>" lines.
func verifyChecksum(archivePath, checksumsPath, assetName string) error {
	// #nosec G304 -- Private temporary file produced by the upgrade flow.
	sums, err := os.Open(checksumsPath)
	if err != nil {
		return err
	}
	defer sums.Close()

	want := ""
	scanner := bufio.NewScanner(sums)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == assetName {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if want == "" {
		return fmt.Errorf("%s is not listed in %s", assetName, checksumsAssetName)
	}

	// #nosec G304 -- Private temporary file produced by the upgrade flow.
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, archive); err != nil {
		return err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", assetName, got, want)
	}
	return nil
}

// extractBinary pulls the CLI out of a release archive into a temporary
// file and returns its path.
func extractBinary(archivePath, assetName string) (string, error) {
	var (
		src     io.Reader
		closers []io.Closer
	)
	defer func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}()

	if strings.HasSuffix(assetName, ".zip") {
		zr, err := zip.OpenReader(archivePath)
		if err != nil {
			return "", err
		}
		closers = append(closers, zr)
		for _, f := range zr.File {
			if f.Name == binaryName() && f.FileInfo().Mode().IsRegular() {
				rc, err := f.Open()
				if err != nil {
					return "", err
				}
				closers = append(closers, rc)
				src = rc
				break
			}
		}
	} else {
		// #nosec G304 -- Private temporary file produced by the upgrade flow.
		f, err := os.Open(archivePath)
		if err != nil {
			return "", err
		}
		closers = append(closers, f)
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", err
		}
		closers = append(closers, gz)
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return "", err
			}
			if hdr.Name == binaryName() && hdr.Typeflag == tar.TypeReg {
				src = tr
				break
			}
		}
	}
	if src == nil {
		return "", fmt.Errorf("%s not found in %s", binaryName(), assetName)
	}

	out, err := os.CreateTemp("", "agentmgr-upgrade-bin-*")
	if err != nil {
		return "", err
	}
	n, copyErr := io.Copy(out, io.LimitReader(src, maxBinarySize+1))
	if n > maxBinarySize {
		copyErr = errors.Join(copyErr, fmt.Errorf("%s in %s is larger than expected", binaryName(), assetName))
	}
	if err := errors.Join(copyErr, out.Close()); err != nil {
		return "", errors.Join(err, os.Remove(out.Name()))
	}
	return out.Name(), nil
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
