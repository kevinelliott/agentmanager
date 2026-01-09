package output

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Spinner displays a loading animation in the terminal.
type Spinner struct {
	mu       sync.Mutex
	active   bool
	message  string
	frames   []string
	interval time.Duration
	done     chan struct{}
	out      io.Writer
	noColor  bool
	style    lipgloss.Style
}

// SpinnerOption is a functional option for configuring a Spinner.
type SpinnerOption func(*Spinner)

// WithMessage sets the spinner message.
func WithMessage(msg string) SpinnerOption {
	return func(s *Spinner) {
		s.message = msg
	}
}

// WithFrames sets custom spinner frames.
func WithFrames(frames []string) SpinnerOption {
	return func(s *Spinner) {
		s.frames = frames
	}
}

// WithInterval sets the spinner animation interval.
func WithInterval(interval time.Duration) SpinnerOption {
	return func(s *Spinner) {
		s.interval = interval
	}
}

// WithOutput sets the spinner output writer.
func WithOutput(w io.Writer) SpinnerOption {
	return func(s *Spinner) {
		s.out = w
	}
}

// WithNoColor disables color output.
func WithNoColor(noColor bool) SpinnerOption {
	return func(s *Spinner) {
		s.noColor = noColor
	}
}

// NewSpinner creates a new spinner instance.
func NewSpinner(opts ...SpinnerOption) *Spinner {
	s := &Spinner{
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		interval: 80 * time.Millisecond,
		out:      os.Stdout,
		done:     make(chan struct{}),
	}

	for _, opt := range opts {
		opt(s)
	}

	// Setup style
	r := lipgloss.NewRenderer(s.out)
	if s.noColor || os.Getenv("NO_COLOR") != "" {
		r.SetColorProfile(termenv.Ascii)
		s.style = r.NewStyle()
	} else {
		s.style = r.NewStyle().Foreground(lipgloss.Color("#BD93F9"))
	}

	return s
}

// Start starts the spinner animation.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.done = make(chan struct{})
	s.mu.Unlock()

	go s.animate()
}

// Stop stops the spinner animation.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	s.mu.Unlock()

	close(s.done)

	// Clear the line
	fmt.Fprintf(s.out, "\r%s\r", clearLine())
}

// UpdateMessage updates the spinner message while it's running.
func (s *Spinner) UpdateMessage(msg string) {
	s.mu.Lock()
	s.message = msg
	s.mu.Unlock()
}

// Success stops the spinner and shows a success message.
func (s *Spinner) Success(msg string) {
	s.Stop()

	var icon string
	if s.noColor || os.Getenv("NO_COLOR") != "" {
		icon = "✓"
	} else {
		r := lipgloss.NewRenderer(s.out)
		successStyle := r.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Bold(true)
		icon = successStyle.Render("✓")
	}

	fmt.Fprintf(s.out, "%s %s\n", icon, msg)
}

// Error stops the spinner and shows an error message.
func (s *Spinner) Error(msg string) {
	s.Stop()

	var icon string
	if s.noColor || os.Getenv("NO_COLOR") != "" {
		icon = "✗"
	} else {
		r := lipgloss.NewRenderer(s.out)
		errorStyle := r.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true)
		icon = errorStyle.Render("✗")
	}

	fmt.Fprintf(s.out, "%s %s\n", icon, msg)
}

// Warning stops the spinner and shows a warning message.
func (s *Spinner) Warning(msg string) {
	s.Stop()

	var icon string
	if s.noColor || os.Getenv("NO_COLOR") != "" {
		icon = "⚠"
	} else {
		r := lipgloss.NewRenderer(s.out)
		warningStyle := r.NewStyle().Foreground(lipgloss.Color("#FFB86C")).Bold(true)
		icon = warningStyle.Render("⚠")
	}

	fmt.Fprintf(s.out, "%s %s\n", icon, msg)
}

// Info stops the spinner and shows an info message.
func (s *Spinner) Info(msg string) {
	s.Stop()

	var icon string
	if s.noColor || os.Getenv("NO_COLOR") != "" {
		icon = "ℹ"
	} else {
		r := lipgloss.NewRenderer(s.out)
		infoStyle := r.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))
		icon = infoStyle.Render("ℹ")
	}

	fmt.Fprintf(s.out, "%s %s\n", icon, msg)
}

// animate runs the spinner animation loop.
func (s *Spinner) animate() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	frameIdx := 0

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			frame := s.frames[frameIdx%len(s.frames)]
			msg := s.message
			s.mu.Unlock()

			// Render frame
			frameStr := s.style.Render(frame)
			fmt.Fprintf(s.out, "\r%s %s", frameStr, msg)

			frameIdx++
		}
	}
}

// clearLine returns ANSI escape sequence to clear the current line.
func clearLine() string {
	return "\033[2K"
}

// SpinnerFrames contains predefined spinner frame sets.
var SpinnerFrames = struct {
	Dots     []string
	Line     []string
	Arrow    []string
	Pulse    []string
	Binary   []string
	Circle   []string
}{
	Dots:     []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	Line:     []string{"-", "\\", "|", "/"},
	Arrow:    []string{"←", "↖", "↑", "↗", "→", "↘", "↓", "↙"},
	Pulse:    []string{"◐", "◓", "◑", "◒"},
	Binary:   []string{"010010", "001001", "100100", "010010", "001001"},
	Circle:   []string{"◜", "◠", "◝", "◞", "◡", "◟"},
}
