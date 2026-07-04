package providers

import (
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

// TestCaskVersionParsesCleanly guards the end-to-end symptom that reached the
// user: a raw Homebrew cask version string (which carries a ",<build-id>"
// suffix) must parse to the clean human version once normalized, not a
// zero/garbage version. The brew provider now normalizes with
// agent.CleanCaskVersion before parsing at every cask read site.
func TestCaskVersionParsesCleanly(t *testing.T) {
	v, err := agent.ParseVersion(agent.CleanCaskVersion("1.0.16,4893150192467968"))
	if err != nil {
		t.Fatalf("ParseVersion returned error: %v", err)
	}
	if got := v.String(); got != "1.0.16" {
		t.Errorf("version string = %q, want %q", got, "1.0.16")
	}
	if v.IsZero() {
		t.Errorf("version unexpectedly zero")
	}
}
