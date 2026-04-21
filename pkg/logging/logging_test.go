package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/config"
)

func TestNew_NilConfig_ReturnsUsableLogger(t *testing.T) {
	l := New(nil)
	if l == nil {
		t.Fatal("New(nil) returned nil")
	}
	// Should not panic; log something at info to prove the handler works.
	l.Info("hello")
}

func TestNew_LevelParsing(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"":        slog.LevelInfo, // default
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"garbage": slog.LevelInfo, // fallback
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			cfg := &config.Config{Logging: config.LoggingConfig{Level: input}}
			l := New(cfg)
			if !l.Enabled(context.Background(), want) {
				t.Errorf("level %q: Enabled(%v) = false, expected true", input, want)
			}
			// One level below `want` should NOT be enabled (unless we're at debug).
			below := want - 4
			if want == slog.LevelDebug {
				return
			}
			if l.Enabled(context.Background(), below) {
				t.Errorf("level %q: Enabled(below=%v) should be false", input, below)
			}
		})
	}
}

func TestBuildHandler_JSONFormat(t *testing.T) {
	// Route through a bytes.Buffer so the test doesn't hold any file
	// handles — Windows TempDir cleanup fails if a file is still open
	// when the test ends, so never hand a file to New() inside a test.
	var buf bytes.Buffer
	l := slog.New(buildHandler(&buf, slog.LevelInfo, "json"))
	l.Info("hello world", "key", "value")

	out := buf.Bytes()
	if !bytes.Contains(out, []byte(`"msg":"hello world"`)) {
		t.Errorf("expected JSON msg field, got:\n%s", out)
	}
	if !bytes.Contains(out, []byte(`"key":"value"`)) {
		t.Errorf("expected JSON key field, got:\n%s", out)
	}
}

func TestBuildHandler_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(buildHandler(&buf, slog.LevelInfo, ""))
	l.Info("hello", "k", "v")

	if !strings.Contains(buf.String(), "msg=hello") {
		t.Errorf("expected text msg=hello, got:\n%s", buf.String())
	}
}

func TestBuildHandler_UnknownFormatDefaultsToText(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(buildHandler(&buf, slog.LevelInfo, "garbage"))
	l.Info("hello")
	if !strings.Contains(buf.String(), "msg=hello") {
		t.Errorf("unknown format should default to text handler, got:\n%s", buf.String())
	}
}

func TestNew_FileOpenFailure_FallsBackToStderr(t *testing.T) {
	// Point File at a directory path that cannot be opened as a file.
	cfg := &config.Config{Logging: config.LoggingConfig{
		Level: "info",
		File:  "/this/path/definitely/does/not/exist/" + t.Name(),
	}}
	// Must not panic, must still return a usable logger.
	l := New(cfg)
	l.Info("still works despite file-open failure")
}

func TestWithContext_RoundTrip(t *testing.T) {
	l := New(nil)
	ctx := WithContext(context.Background(), l)
	got := FromContext(ctx)
	if got != l {
		t.Errorf("FromContext returned different logger than WithContext stored")
	}
}

func TestFromContext_Defaults(t *testing.T) {
	// Nil context → slog.Default()
	// Passing a nil context is intentional — we want to prove FromContext
	// handles it without panicking.
	var nilCtx context.Context
	if FromContext(nilCtx) != slog.Default() {
		t.Error("FromContext(nil) should return slog.Default()")
	}
	// Empty context → slog.Default()
	if FromContext(context.Background()) != slog.Default() {
		t.Error("FromContext(empty) should return slog.Default()")
	}
}

func TestWithContext_NilLoggerIsNoop(t *testing.T) {
	original := New(nil)
	ctx := WithContext(context.Background(), original)
	// Re-attaching nil must not clobber the prior logger.
	ctx = WithContext(ctx, nil)
	if FromContext(ctx) != original {
		t.Error("WithContext(ctx, nil) clobbered a previously attached logger")
	}
}

func TestInstall_SwapsDefault(t *testing.T) {
	before := slog.Default()
	t.Cleanup(func() { slog.SetDefault(before) })

	mine := New(&config.Config{Logging: config.LoggingConfig{Level: "debug"}})
	Install(mine)
	if slog.Default() != mine {
		t.Error("Install did not replace slog.Default()")
	}
}

func TestInstall_NilIsNoop(t *testing.T) {
	before := slog.Default()
	t.Cleanup(func() { slog.SetDefault(before) })

	Install(nil)
	if slog.Default() != before {
		t.Error("Install(nil) should not change slog.Default()")
	}
}
