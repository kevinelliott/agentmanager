package providers

import (
	"context"
	"io"
)

// progressWriterKey is the context key used to attach a progress writer to
// an install/update call. Keeping it unexported prevents accidental key
// collisions across packages.
type progressWriterKey struct{}

// WithProgressWriter returns a new context that carries the given writer.
// Providers use it to tee the subprocess stdout/stderr while they run, so
// callers can surface live output for long-running installs or updates.
//
// A nil or zero writer disables streaming (provider behaves as before).
// Providers always still capture the full output into Result.Output, so
// callers that ignore the stream lose no information.
func WithProgressWriter(ctx context.Context, w io.Writer) context.Context {
	if w == nil {
		return ctx
	}
	return context.WithValue(ctx, progressWriterKey{}, w)
}

// ProgressWriter returns the writer associated with the context, or
// io.Discard if none was set. Callers should use the returned writer
// unconditionally.
func ProgressWriter(ctx context.Context) io.Writer {
	if w, ok := ctx.Value(progressWriterKey{}).(io.Writer); ok && w != nil {
		return w
	}
	return io.Discard
}
