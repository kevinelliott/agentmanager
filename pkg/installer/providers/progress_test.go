package providers

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestProgressWriter_DefaultIsDiscard(t *testing.T) {
	w := ProgressWriter(context.Background())
	if w == nil {
		t.Fatal("ProgressWriter returned nil")
	}
	if w != io.Discard {
		t.Errorf("ProgressWriter(empty ctx) = %T, want io.Discard", w)
	}
}

func TestProgressWriter_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	ctx := WithProgressWriter(context.Background(), &buf)

	w := ProgressWriter(ctx)
	if w == nil {
		t.Fatal("ProgressWriter returned nil after WithProgressWriter")
	}

	const want = "hello streaming world"
	if _, err := w.Write([]byte(want)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := buf.String(); got != want {
		t.Errorf("round-trip: got %q, want %q", got, want)
	}
}

func TestWithProgressWriter_NilIsNoop(t *testing.T) {
	// Attaching a nil writer should leave the context unchanged so that
	// callers passing a zero writer don't accidentally silence a previously
	// attached one.
	var buf bytes.Buffer
	ctx := WithProgressWriter(context.Background(), &buf)
	ctx = WithProgressWriter(ctx, nil)

	w := ProgressWriter(ctx)
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if buf.String() != "x" {
		t.Errorf("nil re-assignment clobbered prior writer: got %q", buf.String())
	}
}
