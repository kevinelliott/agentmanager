package providers

import (
	"testing"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

func TestResult(t *testing.T) {
	result := &Result{
		AgentID:        "claude-code",
		AgentName:      "Claude Code",
		Method:         agent.InstallMethodNPM,
		Version:        agent.MustParseVersion("2.0.0"),
		FromVersion:    agent.MustParseVersion("1.0.0"),
		InstallPath:    "/usr/local/lib/node_modules/@anthropic-ai/claude-code",
		ExecutablePath: "/usr/local/bin/claude",
		Duration:       5 * time.Second,
		Output:         "Install successful",
		WasUpdated:     true,
	}

	if result.AgentID != "claude-code" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "claude-code")
	}
	if result.AgentName != "Claude Code" {
		t.Errorf("AgentName = %q, want %q", result.AgentName, "Claude Code")
	}
	if result.Method != agent.InstallMethodNPM {
		t.Errorf("Method = %q, want %q", result.Method, agent.InstallMethodNPM)
	}
	if result.Version.String() != "2.0.0" {
		t.Errorf("Version = %q, want %q", result.Version.String(), "2.0.0")
	}
	if result.FromVersion.String() != "1.0.0" {
		t.Errorf("FromVersion = %q, want %q", result.FromVersion.String(), "1.0.0")
	}
	if result.InstallPath != "/usr/local/lib/node_modules/@anthropic-ai/claude-code" {
		t.Errorf("InstallPath = %q", result.InstallPath)
	}
	if result.ExecutablePath != "/usr/local/bin/claude" {
		t.Errorf("ExecutablePath = %q, want %q", result.ExecutablePath, "/usr/local/bin/claude")
	}
	if result.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want %v", result.Duration, 5*time.Second)
	}
	if result.Output != "Install successful" {
		t.Errorf("Output = %q, want %q", result.Output, "Install successful")
	}
	if !result.WasUpdated {
		t.Error("WasUpdated should be true")
	}
}

func TestResultInstall(t *testing.T) {
	// Result from install (not update)
	result := &Result{
		AgentID:        "aider",
		AgentName:      "Aider",
		Method:         agent.InstallMethodPipx,
		Version:        agent.MustParseVersion("0.50.0"),
		InstallPath:    "/home/user/.local/pipx/venvs/aider-chat",
		ExecutablePath: "/home/user/.local/bin/aider",
		Duration:       10 * time.Second,
		Output:         "pipx install complete",
		WasUpdated:     false,
	}

	if result.AgentID != "aider" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "aider")
	}
	if result.Method != agent.InstallMethodPipx {
		t.Errorf("Method = %q, want %q", result.Method, agent.InstallMethodPipx)
	}
	if result.WasUpdated {
		t.Error("WasUpdated should be false for install")
	}
	if result.FromVersion.String() != "0.0.0" {
		// FromVersion should be zero value for install
		t.Log("FromVersion is zero for install as expected")
	}
}

func TestResultEmpty(t *testing.T) {
	result := &Result{}

	if result.AgentID != "" {
		t.Errorf("AgentID should be empty, got %q", result.AgentID)
	}
	if result.WasUpdated {
		t.Error("WasUpdated should be false by default")
	}
	if result.Duration != 0 {
		t.Errorf("Duration should be 0, got %v", result.Duration)
	}
}
