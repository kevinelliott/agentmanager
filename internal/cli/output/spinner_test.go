package output

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

// nonTTYBuf is a *bytes.Buffer wrapper that guarantees isTerminal() returns
// false (which it already does for non-*os.File writers, but the explicit
// type makes the test intent obvious).
type nonTTYBuf struct {
	*bytes.Buffer
}

func newNonTTYBuf() *nonTTYBuf { return &nonTTYBuf{&bytes.Buffer{}} }

func TestSpinner_NonTTY_StartIsNoop(t *testing.T) {
	buf := newNonTTYBuf()
	s := NewSpinner(
		WithOutput(buf),
		WithMessage("loading..."),
		WithInterval(5*time.Millisecond),
	)
	s.Start()
	// Start() is a no-op on non-TTY output: no goroutine is spawned and
	// no frames are written. We don't need to wait — if Start had spawned
	// anything, s.isTTY would be true here and the structure of the test
	// fails fast rather than relying on sleep-based timing.
	if s.isTTY {
		t.Fatalf("isTTY = true for bytes.Buffer output; non-TTY path not exercised")
	}
	s.Stop()

	if got := buf.String(); got != "" {
		t.Errorf("non-TTY spinner emitted output: %q", got)
	}
}

func TestSpinner_NonTTY_Success(t *testing.T) {
	buf := newNonTTYBuf()
	s := NewSpinner(WithOutput(buf), WithNoColor(true))
	s.Success("done")

	got := buf.String()
	if !strings.Contains(got, "done") {
		t.Errorf("Success() output missing message: %q", got)
	}
	if !strings.Contains(got, "✓") {
		t.Errorf("Success() output missing check icon: %q", got)
	}
}

func TestSpinner_Variants(t *testing.T) {
	cases := []struct {
		name string
		call func(*Spinner, string)
		icon string
	}{
		{"Success", (*Spinner).Success, "✓"},
		{"Error", (*Spinner).Error, "✗"},
		{"Warning", (*Spinner).Warning, "⚠"},
		{"Info", (*Spinner).Info, "ℹ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := newNonTTYBuf()
			s := NewSpinner(WithOutput(buf), WithNoColor(true))
			tc.call(s, "message here")

			got := buf.String()
			if !strings.Contains(got, tc.icon) {
				t.Errorf("%s missing icon %q: %q", tc.name, tc.icon, got)
			}
			if !strings.Contains(got, "message here") {
				t.Errorf("%s missing message: %q", tc.name, got)
			}
		})
	}
}

func TestSpinner_UpdateMessage(t *testing.T) {
	buf := newNonTTYBuf()
	s := NewSpinner(WithOutput(buf), WithMessage("first"))
	s.UpdateMessage("second")
	// Non-TTY: no output but mutation should be safe under lock.
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.message != "second" {
		t.Errorf("message not updated: %q", s.message)
	}
}

func TestSpinner_NO_COLOR_Env(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	buf := newNonTTYBuf()
	s := NewSpinner(WithOutput(buf))
	if !s.noColor {
		t.Error("NewSpinner should honor NO_COLOR env var")
	}
}

func TestSpinner_DoubleStopIsSafe(t *testing.T) {
	buf := newNonTTYBuf()
	s := NewSpinner(WithOutput(buf))
	// Never started. Stopping twice must not panic.
	s.Stop()
	s.Stop()
}

func TestIsTerminal_Buffer(t *testing.T) {
	// Plain bytes.Buffer is not a file → always false.
	if isTerminal(&bytes.Buffer{}) {
		t.Error("bytes.Buffer should not be reported as TTY")
	}
	// io.Discard is not a file → false.
	if isTerminal(io.Discard) {
		t.Error("io.Discard should not be reported as TTY")
	}
}

func TestSpinner_CustomFrames(t *testing.T) {
	custom := []string{"a", "b", "c"}
	s := NewSpinner(WithFrames(custom))
	if len(s.frames) != 3 || s.frames[0] != "a" {
		t.Errorf("WithFrames did not apply: got %v", s.frames)
	}
}

func TestSpinner_CustomInterval(t *testing.T) {
	s := NewSpinner(WithInterval(250 * time.Millisecond))
	if s.interval != 250*time.Millisecond {
		t.Errorf("WithInterval did not apply: got %v", s.interval)
	}
}
