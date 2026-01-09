// Package output provides CLI output utilities with color support.
package output

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/kevinelliott/agentmgr/pkg/config"
)

// Printer handles colorized output for CLI commands.
type Printer struct {
	cfg      *config.Config
	noColor  bool
	out      io.Writer
	errOut   io.Writer
	renderer *lipgloss.Renderer
}

// NewPrinter creates a new Printer instance.
func NewPrinter(cfg *config.Config, noColor bool) *Printer {
	p := &Printer{
		cfg:     cfg,
		noColor: noColor,
		out:     os.Stdout,
		errOut:  os.Stderr,
	}

	// Check NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		p.noColor = true
	}

	// Override with config setting if colors are disabled
	if cfg != nil && !cfg.UI.UseColors {
		p.noColor = true
	}

	// Create renderer
	p.renderer = lipgloss.NewRenderer(p.out)
	if p.noColor {
		p.renderer.SetColorProfile(termenv.Ascii)
	}

	return p
}

// SetOutput sets the output writer.
func (p *Printer) SetOutput(w io.Writer) {
	p.out = w
	p.renderer = lipgloss.NewRenderer(w)
	if p.noColor {
		p.renderer.SetColorProfile(termenv.Ascii)
	}
}

// SetErrorOutput sets the error output writer.
func (p *Printer) SetErrorOutput(w io.Writer) {
	p.errOut = w
}

// Styles returns the style definitions.
func (p *Printer) Styles() *Styles {
	if p.noColor {
		return noColorStyles()
	}
	return defaultStyles(p.renderer)
}

// Success prints a success message.
func (p *Printer) Success(format string, args ...interface{}) {
	s := p.Styles()
	fmt.Fprintf(p.out, "%s %s\n", s.SuccessIcon(), fmt.Sprintf(format, args...))
}

// Info prints an info message.
func (p *Printer) Info(format string, args ...interface{}) {
	s := p.Styles()
	fmt.Fprintf(p.out, "%s %s\n", s.InfoIcon(), fmt.Sprintf(format, args...))
}

// Warning prints a warning message.
func (p *Printer) Warning(format string, args ...interface{}) {
	s := p.Styles()
	fmt.Fprintf(p.errOut, "%s %s\n", s.WarningIcon(), fmt.Sprintf(format, args...))
}

// Error prints an error message.
func (p *Printer) Error(format string, args ...interface{}) {
	s := p.Styles()
	fmt.Fprintf(p.errOut, "%s %s\n", s.ErrorIcon(), fmt.Sprintf(format, args...))
}

// Print prints a plain message.
func (p *Printer) Print(format string, args ...interface{}) {
	fmt.Fprintf(p.out, format+"\n", args...)
}

// Printf prints a formatted message without a newline.
func (p *Printer) Printf(format string, args ...interface{}) {
	fmt.Fprintf(p.out, format, args...)
}

// Println prints a message with a newline.
func (p *Printer) Println(args ...interface{}) {
	fmt.Fprintln(p.out, args...)
}

// Styles holds the visual styles for CLI output.
type Styles struct {
	renderer *lipgloss.Renderer

	// Colors
	Purple   lipgloss.Color
	Green    lipgloss.Color
	Orange   lipgloss.Color
	Red      lipgloss.Color
	Cyan     lipgloss.Color
	Yellow   lipgloss.Color
	Gray     lipgloss.Color
	White    lipgloss.Color

	// Text styles
	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style
	Muted   lipgloss.Style
	Bold    lipgloss.Style
	Header  lipgloss.Style

	// Status styles
	StatusInstalled lipgloss.Style
	StatusUpdate    lipgloss.Style
	StatusNotFound  lipgloss.Style
	StatusError     lipgloss.Style

	// Badge styles
	Badge        lipgloss.Style
	BadgeSuccess lipgloss.Style
	BadgeWarning lipgloss.Style
	BadgeError   lipgloss.Style
	BadgeInfo    lipgloss.Style
}

// defaultStyles returns the default color styles.
func defaultStyles(r *lipgloss.Renderer) *Styles {
	s := &Styles{
		renderer: r,
		Purple:   lipgloss.Color("#BD93F9"),
		Green:    lipgloss.Color("#50FA7B"),
		Orange:   lipgloss.Color("#FFB86C"),
		Red:      lipgloss.Color("#FF5555"),
		Cyan:     lipgloss.Color("#8BE9FD"),
		Yellow:   lipgloss.Color("#F1FA8C"),
		Gray:     lipgloss.Color("#6272A4"),
		White:    lipgloss.Color("#F8F8F2"),
	}

	s.Success = r.NewStyle().Foreground(s.Green).Bold(true)
	s.Error = r.NewStyle().Foreground(s.Red).Bold(true)
	s.Warning = r.NewStyle().Foreground(s.Orange).Bold(true)
	s.Info = r.NewStyle().Foreground(s.Cyan)
	s.Muted = r.NewStyle().Foreground(s.Gray)
	s.Bold = r.NewStyle().Bold(true)
	s.Header = r.NewStyle().Foreground(s.Purple).Bold(true)

	s.StatusInstalled = r.NewStyle().Foreground(s.Green)
	s.StatusUpdate = r.NewStyle().Foreground(s.Orange)
	s.StatusNotFound = r.NewStyle().Foreground(s.Gray)
	s.StatusError = r.NewStyle().Foreground(s.Red)

	s.Badge = r.NewStyle().Foreground(s.White).Background(s.Gray).Padding(0, 1)
	s.BadgeSuccess = r.NewStyle().Foreground(lipgloss.Color("#282A36")).Background(s.Green).Padding(0, 1)
	s.BadgeWarning = r.NewStyle().Foreground(lipgloss.Color("#282A36")).Background(s.Orange).Padding(0, 1)
	s.BadgeError = r.NewStyle().Foreground(s.White).Background(s.Red).Padding(0, 1)
	s.BadgeInfo = r.NewStyle().Foreground(lipgloss.Color("#282A36")).Background(s.Cyan).Padding(0, 1)

	return s
}

// noColorStyles returns styles with no color formatting.
func noColorStyles() *Styles {
	r := lipgloss.NewRenderer(os.Stdout)
	r.SetColorProfile(termenv.Ascii)

	s := &Styles{
		renderer: r,
	}

	s.Success = r.NewStyle()
	s.Error = r.NewStyle()
	s.Warning = r.NewStyle()
	s.Info = r.NewStyle()
	s.Muted = r.NewStyle()
	s.Bold = r.NewStyle().Bold(true)
	s.Header = r.NewStyle().Bold(true)

	s.StatusInstalled = r.NewStyle()
	s.StatusUpdate = r.NewStyle()
	s.StatusNotFound = r.NewStyle()
	s.StatusError = r.NewStyle()

	s.Badge = r.NewStyle()
	s.BadgeSuccess = r.NewStyle()
	s.BadgeWarning = r.NewStyle()
	s.BadgeError = r.NewStyle()
	s.BadgeInfo = r.NewStyle()

	return s
}

// SuccessIcon returns the success icon.
func (s *Styles) SuccessIcon() string {
	return s.Success.Render("✓")
}

// ErrorIcon returns the error icon.
func (s *Styles) ErrorIcon() string {
	return s.Error.Render("✗")
}

// WarningIcon returns the warning icon.
func (s *Styles) WarningIcon() string {
	return s.Warning.Render("⚠")
}

// InfoIcon returns the info icon.
func (s *Styles) InfoIcon() string {
	return s.Info.Render("ℹ")
}

// UpdateIcon returns the update available icon.
func (s *Styles) UpdateIcon() string {
	return s.StatusUpdate.Render("⬆")
}

// InstalledIcon returns the installed icon.
func (s *Styles) InstalledIcon() string {
	return s.StatusInstalled.Render("●")
}

// NotInstalledIcon returns the not installed icon.
func (s *Styles) NotInstalledIcon() string {
	return s.StatusNotFound.Render("○")
}

// FormatStatus formats a status indicator with color.
func (s *Styles) FormatStatus(status string) string {
	switch status {
	case "installed":
		return s.StatusInstalled.Render("● Installed")
	case "update":
		return s.StatusUpdate.Render("↑ Update Available")
	case "not_installed":
		return s.StatusNotFound.Render("○ Not Installed")
	case "error":
		return s.StatusError.Render("✗ Error")
	default:
		return status
	}
}

// FormatVersion formats a version string with color.
func (s *Styles) FormatVersion(version string, hasUpdate bool) string {
	if hasUpdate {
		return s.StatusUpdate.Render(version)
	}
	return s.Info.Render(version)
}

// FormatAgentName formats an agent name.
func (s *Styles) FormatAgentName(name string) string {
	return s.Bold.Render(name)
}

// FormatHeader formats a table header.
func (s *Styles) FormatHeader(text string) string {
	return s.Header.Render(text)
}

// FormatMethod formats an installation method.
func (s *Styles) FormatMethod(method string) string {
	return s.Muted.Render(method)
}

// FormatBadge formats a badge with the given variant.
func (s *Styles) FormatBadge(text, variant string) string {
	switch variant {
	case "success":
		return s.BadgeSuccess.Render(text)
	case "warning":
		return s.BadgeWarning.Render(text)
	case "error":
		return s.BadgeError.Render(text)
	case "info":
		return s.BadgeInfo.Render(text)
	default:
		return s.Badge.Render(text)
	}
}
