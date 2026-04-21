// Package logging provides a thin wrapper around log/slog that reads its
// level, format, and destination from the AgentManager config.
//
// The stdlib's log/slog is the primary API — callers should use
// FromContext(ctx) to obtain a *slog.Logger and call its methods directly
// (Debug/Info/Warn/Error, With, ...). This package only adds:
//
//   - New(cfg): construct a logger configured from cfg.Logging.
//   - WithContext/FromContext: plumb the logger through a context.
//   - Install: swap slog.Default() to the configured logger, useful in main
//     so libraries that reach for slog.Default also see the configuration.
//
// No custom Info/Warn/Error wrappers — those add no value over slog and
// make it harder to grep for callers.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/kevinelliott/agentmanager/pkg/config"
)

// contextKey is the unexported key type for storing a logger in a context.
// Making it unexported prevents accidental overlap with other packages.
type contextKey struct{}

// New constructs a *slog.Logger configured from cfg.Logging.
//
//   - Level: "debug" | "info" | "warn" | "error" (case-insensitive).
//     Invalid values fall back to info — never error, so a misconfigured
//     user config does not prevent startup.
//   - Format: "json" | "text" (case-insensitive). Default is text.
//   - File: if non-empty, logs are appended to the file. Failure to open
//     the file falls back to stderr and an early warn is emitted so the
//     operator can see it.
//
// A nil cfg or nil cfg.Logging returns a sensible default (info, text,
// stderr) rather than panicking — handy in tests and early startup.
func New(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	format := "text"
	var out io.Writer = os.Stderr

	if cfg != nil {
		switch strings.ToLower(cfg.Logging.Level) {
		case "debug":
			level = slog.LevelDebug
		case "info", "":
			level = slog.LevelInfo
		case "warn", "warning":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
		if f := strings.ToLower(cfg.Logging.Format); f != "" {
			format = f
		}
		if cfg.Logging.File != "" {
			f, err := os.OpenFile(cfg.Logging.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err == nil {
				out = f
			} else {
				// Fall back to stderr and emit a warning so the operator
				// notices. Using the default slog here is intentional —
				// we haven't installed our own yet.
				slog.Warn("logging: could not open log file, falling back to stderr",
					"file", cfg.Logging.File, "err", err)
			}
		}
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	switch format {
	case "json":
		h = slog.NewJSONHandler(out, opts)
	default:
		h = slog.NewTextHandler(out, opts)
	}
	return slog.New(h)
}

// WithContext returns ctx carrying logger. A nil logger is a no-op to keep
// middleware chains safe.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext returns the logger attached to ctx, or slog.Default() if none
// is set. Callers should never see a nil logger.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// Install swaps slog.Default() to logger so libraries that reach for the
// default logger (rather than taking one via DI or context) also benefit
// from the configured level/format. Typical use is once at program start
// after New.
func Install(logger *slog.Logger) {
	if logger == nil {
		return
	}
	slog.SetDefault(logger)
}
