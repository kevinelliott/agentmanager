// Package providers contains installation provider implementations.
package providers

import (
	"strings"
)

// FormatInstallError formats an installation error with helpful hints based on the error content.
func FormatInstallError(manager, operation, stderr string) string {
	var hints []string

	// Check for common error patterns and add helpful hints
	switch manager {
	case "npm":
		hints = append(hints, getNPMHints(stderr)...)
	case "pip", "pip3":
		hints = append(hints, getPipHints(stderr)...)
	case "pipx":
		hints = append(hints, getPipxHints(stderr)...)
	case "uv":
		hints = append(hints, getUVHints(stderr)...)
	case "brew":
		hints = append(hints, getBrewHints(stderr)...)
	case "go":
		hints = append(hints, getGoHints(stderr)...)
	case "cargo":
		hints = append(hints, getCargoHints(stderr)...)
	}

	// Check for generic errors
	hints = append(hints, getGenericHints(stderr)...)

	if len(hints) == 0 {
		return ""
	}

	return "\n\nSuggested fixes:\n" + strings.Join(hints, "\n")
}

func getNPMHints(stderr string) []string {
	var hints []string

	if strings.Contains(stderr, "EACCES") {
		hints = append(hints, `• Permission denied. Configure npm to use a local directory:
    mkdir -p ~/.npm-global
    npm config set prefix '~/.npm-global'
    echo 'export PATH=~/.npm-global/bin:$PATH' >> ~/.bashrc
    source ~/.bashrc`)
	}

	if strings.Contains(stderr, "ENOENT") {
		hints = append(hints, "• Package or file not found. Verify the package name is correct.")
	}

	if strings.Contains(stderr, "network") || strings.Contains(stderr, "ETIMEDOUT") {
		hints = append(hints, "• Network error. Check your internet connection and try again.")
	}

	if strings.Contains(stderr, "E404") || strings.Contains(stderr, "404 Not Found") {
		hints = append(hints, "• Package not found in npm registry. Verify the package name.")
	}

	return hints
}

func getPipHints(stderr string) []string {
	var hints []string

	if strings.Contains(stderr, "Permission denied") || strings.Contains(stderr, "PermissionError") {
		hints = append(hints, `• Permission denied. Use pipx instead for isolated installations:
    pipx install <package>
  Or use a virtual environment:
    python -m venv .venv && source .venv/bin/activate`)
	}

	if strings.Contains(stderr, "externally-managed-environment") {
		hints = append(hints, `• This Python installation is managed by your system.
  Use pipx for isolated installations:
    pipx install <package>
  Or use uv for fast package management:
    uv tool install <package>`)
	}

	if strings.Contains(stderr, "No matching distribution") {
		hints = append(hints, "• Package not found. Verify the package name and try again.")
	}

	if strings.Contains(stderr, "Could not find a version") {
		hints = append(hints, "• Version not found. Try without specifying a version.")
	}

	return hints
}

func getPipxHints(stderr string) []string {
	var hints []string

	if strings.Contains(stderr, "not found") || strings.Contains(stderr, "No such file") {
		hints = append(hints, `• pipx not found. Install it first:
    pip install --user pipx
    python -m pipx ensurepath`)
	}

	if strings.Contains(stderr, "already installed") {
		hints = append(hints, "• Package already installed. Use 'pipx upgrade <package>' to update.")
	}

	return hints
}

func getUVHints(stderr string) []string {
	var hints []string

	if strings.Contains(stderr, "not found") {
		hints = append(hints, `• uv not found. Install it:
    curl -LsSf https://astral.sh/uv/install.sh | sh`)
	}

	return hints
}

func getBrewHints(stderr string) []string {
	var hints []string

	if strings.Contains(stderr, "No available formula") || strings.Contains(stderr, "Error: No formulae found") {
		hints = append(hints, `• Formula not found. Check if you need to tap a repository first:
    brew tap <user>/<repo>
  Then try the install again.`)
	}

	if strings.Contains(stderr, "Permission denied") {
		hints = append(hints, `• Permission denied. Fix Homebrew permissions:
    sudo chown -R $(whoami) $(brew --prefix)/*`)
	}

	if strings.Contains(stderr, "Please update Homebrew") {
		hints = append(hints, `• Homebrew is outdated. Update it first:
    brew update`)
	}

	if strings.Contains(stderr, "already installed") {
		hints = append(hints, "• Package already installed. Use 'brew upgrade <package>' to update.")
	}

	return hints
}

func getGoHints(stderr string) []string {
	var hints []string

	if strings.Contains(stderr, "go: module") && strings.Contains(stderr, "not found") {
		hints = append(hints, "• Module not found. Verify the package path is correct.")
	}

	if strings.Contains(stderr, "GOPATH") || strings.Contains(stderr, "GOBIN") {
		hints = append(hints, `• GOPATH issue. Ensure Go is properly configured:
    export GOPATH=$HOME/go
    export PATH=$PATH:$GOPATH/bin`)
	}

	return hints
}

func getCargoHints(stderr string) []string {
	var hints []string

	if strings.Contains(stderr, "could not find") {
		hints = append(hints, "• Crate not found. Verify the package name on crates.io.")
	}

	if strings.Contains(stderr, "Permission denied") {
		hints = append(hints, `• Permission denied. Check cargo installation:
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh`)
	}

	return hints
}

func getGenericHints(stderr string) []string {
	var hints []string

	if strings.Contains(stderr, "command not found") {
		hints = append(hints, "• Required command not found. Ensure it's installed and in your PATH.")
	}

	if strings.Contains(stderr, "timeout") || strings.Contains(stderr, "timed out") {
		hints = append(hints, "• Operation timed out. Check your network connection and try again.")
	}

	if strings.Contains(stderr, "SSL") || strings.Contains(stderr, "certificate") {
		hints = append(hints, "• SSL/certificate error. Check your system certificates and network proxy settings.")
	}

	return hints
}
