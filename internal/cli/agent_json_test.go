package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// whatever fn wrote there. We need this for emitAgentBatchJSON because it
// writes to os.Stdout directly — that's the production behavior we want
// to keep, so the test mirrors it rather than threading an io.Writer.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
	}()

	fnErr := fn()
	if cerr := w.Close(); cerr != nil {
		t.Fatalf("close pipe: %v", cerr)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy: %v", err)
	}
	return buf.String(), fnErr
}

func TestEmitAgentBatchJSON_AllSuccess(t *testing.T) {
	entries := []agentBatchEntry{
		{Agent: "claude", Status: batchStatusSuccess, Method: "brew", Version: "1.0.0"},
		{Agent: "codex", Status: batchStatusSuccess, Method: "npm", Version: "2.1.0"},
	}

	out, err := captureStdout(t, func() error {
		return emitAgentBatchJSON("install", entries)
	})
	if err != nil {
		t.Fatalf("emit returned error on all-success: %v", err)
	}

	var got agentBatchResult
	if jerr := json.Unmarshal([]byte(out), &got); jerr != nil {
		t.Fatalf("emitted output is not valid JSON: %v\noutput: %s", jerr, out)
	}
	if got.Command != "install" {
		t.Errorf("command = %q, want install", got.Command)
	}
	if len(got.Results) != 2 {
		t.Fatalf("results len = %d, want 2", len(got.Results))
	}
	if got.Summary.Total != 2 || got.Summary.Succeeded != 2 || got.Summary.Failed != 0 {
		t.Errorf("summary = %+v, want total=2 succeeded=2 failed=0", got.Summary)
	}
	// Skipped is omitempty: must be absent from output when zero so scripts
	// don't see a misleading "0" they have to special-case.
	if strings.Contains(out, `"skipped"`) {
		t.Errorf("output should omit skipped when zero, got:\n%s", out)
	}
}

func TestEmitAgentBatchJSON_MixedOutcomes(t *testing.T) {
	entries := []agentBatchEntry{
		{Agent: "ok", Status: batchStatusSuccess, Method: "brew", Version: "1.0.0"},
		{Agent: "boom", Status: batchStatusError, Method: "npm", Error: "exit 1"},
		{Agent: "noop", Status: batchStatusNoop, Reason: "already up to date"},
		{Agent: "skip", Status: batchStatusSkipped, Reason: "user canceled"},
	}

	out, err := captureStdout(t, func() error {
		return emitAgentBatchJSON("update", entries)
	})
	if err == nil {
		t.Fatal("emit should return error when any entry failed")
	}
	if !errors.Is(err, ErrSilent) {
		t.Errorf("error should wrap ErrSilent so main.go suppresses its stderr Print, got: %v", err)
	}
	if !strings.Contains(err.Error(), "1 of 4 agent(s) failed") {
		t.Errorf("error message = %q, want it to mention 1 of 4 failed", err.Error())
	}

	var got agentBatchResult
	if jerr := json.Unmarshal([]byte(out), &got); jerr != nil {
		t.Fatalf("output is not valid JSON: %v", jerr)
	}
	if got.Summary.Total != 4 {
		t.Errorf("total = %d, want 4", got.Summary.Total)
	}
	if got.Summary.Succeeded != 1 {
		t.Errorf("succeeded = %d, want 1", got.Summary.Succeeded)
	}
	if got.Summary.Failed != 1 {
		t.Errorf("failed = %d, want 1", got.Summary.Failed)
	}
	// skipped + noop both count toward skipped in the summary, since they
	// share the "we did not change this agent" meaning for scripts.
	if got.Summary.Skipped != 2 {
		t.Errorf("skipped = %d, want 2 (noop + skipped)", got.Summary.Skipped)
	}
}

func TestEmitAgentBatchJSON_EmptyEntries(t *testing.T) {
	out, err := captureStdout(t, func() error {
		return emitAgentBatchJSON("remove", nil)
	})
	if err != nil {
		t.Fatalf("emit on empty should not error: %v", err)
	}
	var got agentBatchResult
	if jerr := json.Unmarshal([]byte(out), &got); jerr != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", jerr, out)
	}
	if got.Command != "remove" {
		t.Errorf("command = %q, want remove", got.Command)
	}
	if got.Summary.Total != 0 {
		t.Errorf("total = %d, want 0", got.Summary.Total)
	}
	if got.Results == nil {
		// json.Unmarshal turns null/[] into a nil slice; either is OK for
		// scripts but we want the output to actually use [] not null so
		// jq idioms like `.results[]` work without nullguarding.
		if !strings.Contains(out, `"results": []`) && !strings.Contains(out, `"results":[]`) {
			t.Errorf("results should serialize as [] (empty array), not null. output:\n%s", out)
		}
	}
}
