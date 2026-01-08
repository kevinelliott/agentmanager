// Package styles provides Lip Gloss styles for the TUI.
package styles

import "github.com/charmbracelet/lipgloss"

// Colors - Dracula inspired palette
var (
	Purple    = lipgloss.Color("#BD93F9")
	Green     = lipgloss.Color("#50FA7B")
	Orange    = lipgloss.Color("#FFB86C")
	Red       = lipgloss.Color("#FF5555")
	Cyan      = lipgloss.Color("#8BE9FD")
	Pink      = lipgloss.Color("#FF79C6")
	Yellow    = lipgloss.Color("#F1FA8C")
	White     = lipgloss.Color("#F8F8F2")
	Gray      = lipgloss.Color("#6272A4")
	DarkGray  = lipgloss.Color("#44475A")
	BG        = lipgloss.Color("#282A36")
	CurrentBG = lipgloss.Color("#44475A")
)

// Base styles
var (
	// App container
	App = lipgloss.NewStyle().
		Background(BG)

	// Title styles
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Purple).
		MarginBottom(1)

	TitleBar = lipgloss.NewStyle().
			Bold(true).
			Foreground(White).
			Background(Purple).
			Padding(0, 1)

	// Subtitle
	Subtitle = lipgloss.NewStyle().
			Foreground(Gray).
			MarginBottom(1)

	// Status bar
	StatusBar = lipgloss.NewStyle().
			Foreground(White).
			Background(DarkGray).
			Padding(0, 1)

	// Help text
	Help = lipgloss.NewStyle().
		Foreground(Gray)

	HelpKey = lipgloss.NewStyle().
		Foreground(Purple).
		Bold(true)

	// List item styles
	ListItem = lipgloss.NewStyle().
			PaddingLeft(2)

	SelectedItem = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(White).
			Background(CurrentBG).
			Bold(true)

	// Agent status styles
	StatusInstalled = lipgloss.NewStyle().
			Foreground(Green)

	StatusUpdateAvailable = lipgloss.NewStyle().
				Foreground(Orange)

	StatusNotInstalled = lipgloss.NewStyle().
				Foreground(Gray)

	StatusError = lipgloss.NewStyle().
			Foreground(Red)

	// Version styles
	Version = lipgloss.NewStyle().
		Foreground(Cyan)

	VersionOld = lipgloss.NewStyle().
			Foreground(Orange)

	VersionNew = lipgloss.NewStyle().
			Foreground(Green)

	// Table styles
	TableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(Purple).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(DarkGray)

	TableRow = lipgloss.NewStyle().
			Foreground(White)

	TableRowSelected = lipgloss.NewStyle().
				Foreground(White).
				Background(CurrentBG)

	// Box styles
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Purple).
		Padding(1, 2)

	BoxFocused = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Cyan).
			Padding(1, 2)

	// Button styles
	Button = lipgloss.NewStyle().
		Foreground(White).
		Background(DarkGray).
		Padding(0, 2).
		MarginRight(1)

	ButtonActive = lipgloss.NewStyle().
			Foreground(BG).
			Background(Purple).
			Padding(0, 2).
			MarginRight(1).
			Bold(true)

	ButtonDanger = lipgloss.NewStyle().
			Foreground(White).
			Background(Red).
			Padding(0, 2).
			MarginRight(1)

	// Badge styles
	Badge = lipgloss.NewStyle().
		Foreground(White).
		Background(DarkGray).
		Padding(0, 1)

	BadgeSuccess = lipgloss.NewStyle().
			Foreground(BG).
			Background(Green).
			Padding(0, 1)

	BadgeWarning = lipgloss.NewStyle().
			Foreground(BG).
			Background(Orange).
			Padding(0, 1)

	BadgeError = lipgloss.NewStyle().
			Foreground(White).
			Background(Red).
			Padding(0, 1)

	// Input styles
	Input = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(DarkGray).
		Padding(0, 1)

	InputFocused = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(Purple).
			Padding(0, 1)

	// Tab styles
	Tab = lipgloss.NewStyle().
		Foreground(Gray).
		Padding(0, 2)

	TabActive = lipgloss.NewStyle().
			Foreground(White).
			Background(Purple).
			Padding(0, 2).
			Bold(true)

	// Spinner
	Spinner = lipgloss.NewStyle().
		Foreground(Purple)

	// Error message
	ErrorMessage = lipgloss.NewStyle().
			Foreground(Red).
			Bold(true)

	// Success message
	SuccessMessage = lipgloss.NewStyle().
			Foreground(Green).
			Bold(true)

	// Info message
	InfoMessage = lipgloss.NewStyle().
			Foreground(Cyan)

	// Warning message
	WarningMessage = lipgloss.NewStyle().
			Foreground(Orange)
)

// Dimensions
const (
	MinWidth     = 80
	MinHeight    = 24
	SidebarWidth = 30
	MaxWidth     = 120
)

// FormatStatus returns a styled status string.
func FormatStatus(status string) string {
	switch status {
	case "installed":
		return StatusInstalled.Render("● Installed")
	case "update":
		return StatusUpdateAvailable.Render("↑ Update Available")
	case "not_installed":
		return StatusNotInstalled.Render("○ Not Installed")
	case "error":
		return StatusError.Render("✗ Error")
	default:
		return StatusNotInstalled.Render(status)
	}
}

// FormatVersion returns a styled version string.
func FormatVersion(version string, hasUpdate bool) string {
	if hasUpdate {
		return VersionOld.Render(version)
	}
	return Version.Render(version)
}

// FormatBadge returns a styled badge.
func FormatBadge(text string, variant string) string {
	switch variant {
	case "success":
		return BadgeSuccess.Render(text)
	case "warning":
		return BadgeWarning.Render(text)
	case "error":
		return BadgeError.Render(text)
	default:
		return Badge.Render(text)
	}
}
