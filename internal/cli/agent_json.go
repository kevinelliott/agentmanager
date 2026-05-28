package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ErrSilent is wrapped into errors returned by --json paths whose
// human-readable message has already been encoded into the JSON payload
// on stdout. main.go checks `errors.Is(err, ErrSilent)` and skips the
// "Error: ..." stderr line in that case, so stdout stays pure JSON
// while the process still exits non-zero.
var ErrSilent = errors.New("silent failure")

// agentBatchResult is the JSON envelope emitted by install/update/remove
// when --json is set. The shape is stable so scripts can rely on it:
// one entry per agent processed (or per (agent, method) for update),
// plus a final summary with counts.
type agentBatchResult struct {
	Command string            `json:"command"`
	Results []agentBatchEntry `json:"results"`
	Summary agentBatchSummary `json:"summary"`
}

// Status values used in agentBatchEntry.Status. Kept as named constants
// so any future renames are a compile-time signal, not a string-grep
// hunt across docs and tests.
const (
	batchStatusSuccess = "success"
	batchStatusError   = "error"
	batchStatusSkipped = "skipped"
	batchStatusNoop    = "noop"
)

type agentBatchEntry struct {
	Agent           string `json:"agent"`
	Status          string `json:"status"`
	Method          string `json:"method,omitempty"`
	Version         string `json:"version,omitempty"`
	PreviousVersion string `json:"previous_version,omitempty"`
	Reason          string `json:"reason,omitempty"`
	Error           string `json:"error,omitempty"`
}

type agentBatchSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped,omitempty"`
}

// emitAgentBatchJSON writes the batch result to stdout as indented JSON
// and returns a non-nil error if any entry failed. Callers should set
// cmd.SilenceErrors+SilenceUsage so cobra doesn't print anything else
// to stderr — keeping stdout strictly JSON for downstream `jq`.
func emitAgentBatchJSON(command string, entries []agentBatchEntry) error {
	// Guarantee Results serializes as [] (not null) on empty input so
	// `jq '.results[]'` in scripts works without nullguarding.
	if entries == nil {
		entries = []agentBatchEntry{}
	}
	res := agentBatchResult{Command: command, Results: entries}
	for _, e := range entries {
		res.Summary.Total++
		switch e.Status {
		case batchStatusSuccess:
			res.Summary.Succeeded++
		case batchStatusError:
			res.Summary.Failed++
		case batchStatusSkipped, batchStatusNoop:
			res.Summary.Skipped++
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(res); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	if res.Summary.Failed > 0 {
		return fmt.Errorf("%w: %d of %d agent(s) failed", ErrSilent, res.Summary.Failed, res.Summary.Total)
	}
	return nil
}
