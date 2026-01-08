// Package agent provides core types and interfaces for AI development agents.
package agent

import (
	"fmt"
	"time"
)

// InstallMethod represents how an agent was installed.
type InstallMethod string

const (
	// Full form constants
	InstallMethodNPM        InstallMethod = "npm"
	InstallMethodBrew       InstallMethod = "brew"
	InstallMethodPip        InstallMethod = "pip"
	InstallMethodPipx       InstallMethod = "pipx"
	InstallMethodUV         InstallMethod = "uv"
	InstallMethodScoop      InstallMethod = "scoop"
	InstallMethodWinget     InstallMethod = "winget"
	InstallMethodChocolatey InstallMethod = "chocolatey"
	InstallMethodNative     InstallMethod = "native"
	InstallMethodCurl       InstallMethod = "curl"
	InstallMethodBinary     InstallMethod = "binary"

	// Short form aliases
	MethodNPM        = InstallMethodNPM
	MethodBrew       = InstallMethodBrew
	MethodPip        = InstallMethodPip
	MethodPipx       = InstallMethodPipx
	MethodUV         = InstallMethodUV
	MethodScoop      = InstallMethodScoop
	MethodWinget     = InstallMethodWinget
	MethodChocolatey = InstallMethodChocolatey
	MethodNative     = InstallMethodNative
	MethodCurl       = InstallMethodCurl
	MethodBinary     = InstallMethodBinary
)

// String returns the string representation of the install method.
func (m InstallMethod) String() string {
	return string(m)
}

// DisplayName returns a human-friendly name for the install method.
func (m InstallMethod) DisplayName() string {
	names := map[InstallMethod]string{
		InstallMethodNPM:        "npm",
		InstallMethodBrew:       "Homebrew",
		InstallMethodPip:        "pip",
		InstallMethodPipx:       "pipx",
		InstallMethodUV:         "uv",
		InstallMethodScoop:      "Scoop",
		InstallMethodWinget:     "winget",
		InstallMethodChocolatey: "Chocolatey",
		InstallMethodNative:     "Native Installer",
		InstallMethodCurl:       "curl",
		InstallMethodBinary:     "Binary",
	}
	if name, ok := names[m]; ok {
		return name
	}
	return string(m)
}

// Agent represents a detected AI development agent.
type Agent struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Homepage    string `json:"homepage,omitempty"`
	Repository  string `json:"repository,omitempty"`
}

// Installation represents a unique installation instance of an agent.
// The same agent can have multiple installations via different methods.
type Installation struct {
	AgentID          string            `json:"agent_id"`
	AgentName        string            `json:"agent_name"`
	Method           InstallMethod     `json:"install_method"`
	InstalledVersion Version           `json:"installed_version"`
	LatestVersion    *Version          `json:"latest_version,omitempty"`
	ExecutablePath   string            `json:"executable_path"`
	InstallPath      string            `json:"install_path,omitempty"`
	IsGlobal         bool              `json:"is_global"`
	DetectedAt       time.Time         `json:"detected_at"`
	LastChecked      time.Time         `json:"last_checked"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// Key returns a unique identifier for this installation.
func (i Installation) Key() string {
	return fmt.Sprintf("%s:%s:%s", i.AgentID, i.Method, i.ExecutablePath)
}

// HasUpdate returns true if an update is available.
func (i Installation) HasUpdate() bool {
	if i.LatestVersion == nil {
		return false
	}
	return i.LatestVersion.IsNewerThan(i.InstalledVersion)
}

// Status represents the current status of an installation.
type Status string

const (
	StatusCurrent    Status = "current"
	StatusOutdated   Status = "outdated"
	StatusUnknown    Status = "unknown"
	StatusError      Status = "error"
	StatusInstalling Status = "installing"
	StatusUpdating   Status = "updating"
)

// GetStatus returns the current status of the installation.
func (i Installation) GetStatus() Status {
	if i.LatestVersion == nil {
		return StatusUnknown
	}
	if i.HasUpdate() {
		return StatusOutdated
	}
	return StatusCurrent
}

// UpdateInfo contains information about an available update.
type UpdateInfo struct {
	Installation Installation `json:"installation"`
	FromVersion  Version      `json:"from_version"`
	ToVersion    Version      `json:"to_version"`
	Changelog    string       `json:"changelog,omitempty"`
	ReleaseNotes []Release    `json:"release_notes,omitempty"`
	PublishedAt  time.Time    `json:"published_at,omitempty"`
}

// Release represents a single release with its notes.
type Release struct {
	Version     Version   `json:"version"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Highlights  []string  `json:"highlights,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	URL         string    `json:"url,omitempty"`
}
