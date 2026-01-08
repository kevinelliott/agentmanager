package detector

import (
	"context"
	"testing"
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

// mockStrategy is a test strategy that returns predetermined results.
type mockStrategy struct {
	name          string
	method        agent.InstallMethod
	applicable    bool
	installations []*agent.Installation
	err           error
}

func (m *mockStrategy) Name() string {
	return m.name
}

func (m *mockStrategy) Method() agent.InstallMethod {
	return m.method
}

func (m *mockStrategy) IsApplicable(p platform.Platform) bool {
	return m.applicable
}

func (m *mockStrategy) Detect(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.installations, nil
}

func TestNewDetector(t *testing.T) {
	p := platform.Current()
	d := New(p)

	if d == nil {
		t.Fatal("New() returned nil")
	}

	// Should have default strategies registered
	strategies := d.GetStrategies()
	if len(strategies) == 0 {
		t.Error("New() should register default strategies")
	}
}

func TestDetectorRegisterStrategy(t *testing.T) {
	p := platform.Current()
	d := &Detector{
		platform:   p,
		strategies: make([]Strategy, 0),
	}

	mock := &mockStrategy{
		name:       "test",
		method:     agent.InstallMethodNPM,
		applicable: true,
	}

	d.RegisterStrategy(mock)

	strategies := d.GetStrategies()
	if len(strategies) != 1 {
		t.Errorf("GetStrategies() returned %d strategies, want 1", len(strategies))
	}
}

func TestDetectorGetStrategies(t *testing.T) {
	p := platform.Current()
	d := &Detector{
		platform:   p,
		strategies: make([]Strategy, 0),
	}

	// Initially empty
	if len(d.GetStrategies()) != 0 {
		t.Error("GetStrategies() should return empty slice initially")
	}

	// Register multiple strategies
	d.RegisterStrategy(&mockStrategy{name: "s1", method: agent.InstallMethodNPM, applicable: true})
	d.RegisterStrategy(&mockStrategy{name: "s2", method: agent.InstallMethodPip, applicable: true})
	d.RegisterStrategy(&mockStrategy{name: "s3", method: agent.InstallMethodBrew, applicable: false})

	strategies := d.GetStrategies()
	if len(strategies) != 3 {
		t.Errorf("GetStrategies() returned %d strategies, want 3", len(strategies))
	}
}

func TestDetectorDetectAll(t *testing.T) {
	p := platform.Current()
	d := &Detector{
		platform:   p,
		strategies: make([]Strategy, 0),
	}

	now := time.Now()
	mockInstallations := []*agent.Installation{
		{
			AgentID:          "claude-code",
			AgentName:        "Claude Code",
			Method:           agent.InstallMethodNPM,
			InstalledVersion: agent.MustParseVersion("1.0.0"),
			ExecutablePath:   "/usr/local/bin/claude",
			DetectedAt:       now,
		},
		{
			AgentID:          "aider",
			AgentName:        "Aider",
			Method:           agent.InstallMethodPip,
			InstalledVersion: agent.MustParseVersion("0.50.0"),
			ExecutablePath:   "/home/user/.local/bin/aider",
		},
	}

	// Register strategies with predetermined results
	d.RegisterStrategy(&mockStrategy{
		name:          "npm",
		method:        agent.InstallMethodNPM,
		applicable:    true,
		installations: mockInstallations[:1],
	})
	d.RegisterStrategy(&mockStrategy{
		name:          "pip",
		method:        agent.InstallMethodPip,
		applicable:    true,
		installations: mockInstallations[1:],
	})
	d.RegisterStrategy(&mockStrategy{
		name:       "brew",
		method:     agent.InstallMethodBrew,
		applicable: false, // Not applicable
	})

	ctx := context.Background()
	installations, err := d.DetectAll(ctx, nil)

	if err != nil {
		t.Fatalf("DetectAll() error = %v", err)
	}

	if len(installations) != 2 {
		t.Errorf("DetectAll() returned %d installations, want 2", len(installations))
	}

	// Verify timestamps are set
	for _, inst := range installations {
		if inst.LastChecked.IsZero() {
			t.Error("LastChecked should be set")
		}
	}
}

func TestDetectorDetectAllDeduplication(t *testing.T) {
	p := platform.Current()
	d := &Detector{
		platform:   p,
		strategies: make([]Strategy, 0),
	}

	// Both strategies return the same installation (should be deduplicated)
	sameInstallation := &agent.Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		Method:           agent.InstallMethodNPM,
		InstalledVersion: agent.MustParseVersion("1.0.0"),
		ExecutablePath:   "/usr/local/bin/claude",
	}

	d.RegisterStrategy(&mockStrategy{
		name:          "s1",
		method:        agent.InstallMethodNPM,
		applicable:    true,
		installations: []*agent.Installation{sameInstallation},
	})
	d.RegisterStrategy(&mockStrategy{
		name:          "s2",
		method:        agent.InstallMethodNPM,
		applicable:    true,
		installations: []*agent.Installation{sameInstallation},
	})

	ctx := context.Background()
	installations, err := d.DetectAll(ctx, nil)

	if err != nil {
		t.Fatalf("DetectAll() error = %v", err)
	}

	if len(installations) != 1 {
		t.Errorf("DetectAll() returned %d installations after deduplication, want 1", len(installations))
	}
}

func TestDetectorDetectByMethod(t *testing.T) {
	p := platform.Current()
	d := &Detector{
		platform:   p,
		strategies: make([]Strategy, 0),
	}

	npmInst := &agent.Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		Method:           agent.InstallMethodNPM,
		InstalledVersion: agent.MustParseVersion("1.0.0"),
		ExecutablePath:   "/usr/local/bin/claude",
	}
	pipInst := &agent.Installation{
		AgentID:          "aider",
		AgentName:        "Aider",
		Method:           agent.InstallMethodPip,
		InstalledVersion: agent.MustParseVersion("0.50.0"),
		ExecutablePath:   "/home/user/.local/bin/aider",
	}

	d.RegisterStrategy(&mockStrategy{
		name:          "npm",
		method:        agent.InstallMethodNPM,
		applicable:    true,
		installations: []*agent.Installation{npmInst},
	})
	d.RegisterStrategy(&mockStrategy{
		name:          "pip",
		method:        agent.InstallMethodPip,
		applicable:    true,
		installations: []*agent.Installation{pipInst},
	})

	ctx := context.Background()

	// Detect by npm method
	npmResults, err := d.DetectByMethod(ctx, agent.InstallMethodNPM, nil)
	if err != nil {
		t.Fatalf("DetectByMethod(npm) error = %v", err)
	}
	if len(npmResults) != 1 {
		t.Errorf("DetectByMethod(npm) returned %d, want 1", len(npmResults))
	}
	if npmResults[0].AgentID != "claude-code" {
		t.Errorf("Expected claude-code, got %s", npmResults[0].AgentID)
	}

	// Detect by pip method
	pipResults, err := d.DetectByMethod(ctx, agent.InstallMethodPip, nil)
	if err != nil {
		t.Fatalf("DetectByMethod(pip) error = %v", err)
	}
	if len(pipResults) != 1 {
		t.Errorf("DetectByMethod(pip) returned %d, want 1", len(pipResults))
	}
	if pipResults[0].AgentID != "aider" {
		t.Errorf("Expected aider, got %s", pipResults[0].AgentID)
	}
}

func TestDetectorDetectByMethodNotFound(t *testing.T) {
	p := platform.Current()
	d := &Detector{
		platform:   p,
		strategies: make([]Strategy, 0),
	}

	// No strategies registered
	ctx := context.Background()
	_, err := d.DetectByMethod(ctx, agent.InstallMethodBrew, nil)

	if err == nil {
		t.Error("DetectByMethod() should return error for unavailable method")
	}
}

func TestDeduplicateInstallations(t *testing.T) {
	inst1 := &agent.Installation{
		AgentID:        "claude-code",
		Method:         agent.InstallMethodNPM,
		ExecutablePath: "/usr/local/bin/claude",
	}
	inst2 := &agent.Installation{
		AgentID:        "aider",
		Method:         agent.InstallMethodPip,
		ExecutablePath: "/home/user/.local/bin/aider",
	}
	inst3 := &agent.Installation{ // Duplicate of inst1
		AgentID:        "claude-code",
		Method:         agent.InstallMethodNPM,
		ExecutablePath: "/usr/local/bin/claude",
	}

	installations := []*agent.Installation{inst1, inst2, inst3}
	result := deduplicateInstallations(installations)

	if len(result) != 2 {
		t.Errorf("deduplicateInstallations() returned %d, want 2", len(result))
	}
}

func TestResultNewInstallations(t *testing.T) {
	existing := []*agent.Installation{
		{
			AgentID:        "claude-code",
			Method:         agent.InstallMethodNPM,
			ExecutablePath: "/usr/local/bin/claude",
		},
	}

	detected := []*agent.Installation{
		{
			AgentID:        "claude-code",
			Method:         agent.InstallMethodNPM,
			ExecutablePath: "/usr/local/bin/claude",
		},
		{
			AgentID:        "aider",
			Method:         agent.InstallMethodPip,
			ExecutablePath: "/home/user/.local/bin/aider",
		},
	}

	result := &Result{
		Installations: detected,
	}

	newInsts := result.NewInstallations(existing)
	if len(newInsts) != 1 {
		t.Errorf("NewInstallations() returned %d, want 1", len(newInsts))
	}
	if newInsts[0].AgentID != "aider" {
		t.Errorf("NewInstallations() returned wrong agent: %s", newInsts[0].AgentID)
	}
}

func TestResultRemovedInstallations(t *testing.T) {
	existing := []*agent.Installation{
		{
			AgentID:        "claude-code",
			Method:         agent.InstallMethodNPM,
			ExecutablePath: "/usr/local/bin/claude",
		},
		{
			AgentID:        "aider",
			Method:         agent.InstallMethodPip,
			ExecutablePath: "/home/user/.local/bin/aider",
		},
	}

	detected := []*agent.Installation{
		{
			AgentID:        "claude-code",
			Method:         agent.InstallMethodNPM,
			ExecutablePath: "/usr/local/bin/claude",
		},
	}

	result := &Result{
		Installations: detected,
	}

	removed := result.RemovedInstallations(existing)
	if len(removed) != 1 {
		t.Errorf("RemovedInstallations() returned %d, want 1", len(removed))
	}
	if removed[0].AgentID != "aider" {
		t.Errorf("RemovedInstallations() returned wrong agent: %s", removed[0].AgentID)
	}
}

func TestResultNewInstallationsEmpty(t *testing.T) {
	existing := []*agent.Installation{
		{
			AgentID:        "claude-code",
			Method:         agent.InstallMethodNPM,
			ExecutablePath: "/usr/local/bin/claude",
		},
	}

	detected := []*agent.Installation{
		{
			AgentID:        "claude-code",
			Method:         agent.InstallMethodNPM,
			ExecutablePath: "/usr/local/bin/claude",
		},
	}

	result := &Result{
		Installations: detected,
	}

	newInsts := result.NewInstallations(existing)
	if len(newInsts) != 0 {
		t.Errorf("NewInstallations() returned %d, want 0", len(newInsts))
	}
}

func TestResultRemovedInstallationsEmpty(t *testing.T) {
	existing := []*agent.Installation{
		{
			AgentID:        "claude-code",
			Method:         agent.InstallMethodNPM,
			ExecutablePath: "/usr/local/bin/claude",
		},
	}

	detected := []*agent.Installation{
		{
			AgentID:        "claude-code",
			Method:         agent.InstallMethodNPM,
			ExecutablePath: "/usr/local/bin/claude",
		},
	}

	result := &Result{
		Installations: detected,
	}

	removed := result.RemovedInstallations(existing)
	if len(removed) != 0 {
		t.Errorf("RemovedInstallations() returned %d, want 0", len(removed))
	}
}

func TestResultStruct(t *testing.T) {
	result := &Result{
		Installations: []*agent.Installation{
			{AgentID: "test"},
		},
		Errors:   []error{},
		Duration: 5 * time.Second,
	}

	if len(result.Installations) != 1 {
		t.Errorf("Installations count = %d, want 1", len(result.Installations))
	}
	if result.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want %v", result.Duration, 5*time.Second)
	}
}
