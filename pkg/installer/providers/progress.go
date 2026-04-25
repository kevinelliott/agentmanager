package providers

import (
	"context"
	"io"
	"sync"
)

// progressWriterKey is the context key used to attach a progress writer to
// an install/update call. Keeping it unexported prevents accidental key
// collisions across packages.
type progressWriterKey struct{}

// WithProgressWriter returns a new context that carries the given writer.
// Providers use it to tee the subprocess stdout/stderr while they run, so
// callers can surface live output for long-running installs or updates.
//
// Passing nil is a no-op: the returned context is the input context, so a
// previously attached writer is NOT cleared. To actually silence streaming
// after one was attached, start from a context that never had one. This
// asymmetry lets middleware attach a writer safely without worrying about
// downstream code accidentally clobbering it with nil.
//
// Providers always still capture the full output into Result.Output, so
// callers that ignore the stream lose no information.
//
// The returned writer is safe for concurrent calls to Write — providers
// use it to tee BOTH cmd.Stdout and cmd.Stderr, which os/exec writes to
// from separate goroutines. Internally Write calls serialize on a mutex
// so concurrent writes produce non-interleaved byte sequences per call.
func WithProgressWriter(ctx context.Context, w io.Writer) context.Context {
	if w == nil {
		return ctx
	}
	return context.WithValue(ctx, progressWriterKey{}, &lockedWriter{w: w})
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

// lockedWriter serializes concurrent Write calls so teeing a single
// progress writer from cmd.Stdout and cmd.Stderr (os/exec writes those
// from separate goroutines) doesn't interleave bytes mid-call.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
