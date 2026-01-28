package detector

import (
	"context"
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// BenchmarkDetectorDetectAll benchmarks the full detection pipeline.
func BenchmarkDetectorDetectAll(b *testing.B) {
	plat := &benchPlatform{}
	d := New(plat)

	agents := benchAgentDefs()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.DetectAll(ctx, agents)
	}
}

// BenchmarkDetectorGetStrategies benchmarks getting strategies.
func BenchmarkDetectorGetStrategies(b *testing.B) {
	plat := &benchPlatform{}
	d := New(plat)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.GetStrategies()
	}
}

// benchAgentDefs creates test agent definitions for benchmarking.
func benchAgentDefs() []catalog.AgentDef {
	return []catalog.AgentDef{
		{
			ID:   "claude-code",
			Name: "Claude Code",
			Detection: catalog.DetectionDef{
				Executables: []string{"claude"},
				VersionCmd:  "claude --version",
			},
		},
		{
			ID:   "aider",
			Name: "Aider",
			Detection: catalog.DetectionDef{
				Executables: []string{"aider"},
				VersionCmd:  "aider --version",
			},
		},
		{
			ID:   "amp",
			Name: "Amp",
			Detection: catalog.DetectionDef{
				Executables: []string{"amp"},
				VersionCmd:  "amp --version",
			},
		},
	}
}

// benchPlatform is a simple mock for benchmarking.
type benchPlatform struct{}

func (p *benchPlatform) ID() platform.ID          { return platform.Darwin }
func (p *benchPlatform) Architecture() string     { return "arm64" }
func (p *benchPlatform) Name() string             { return "macOS" }
func (p *benchPlatform) GetDataDir() string       { return "/tmp/agentmgr" }
func (p *benchPlatform) GetConfigDir() string     { return "/tmp/agentmgr/config" }
func (p *benchPlatform) GetCacheDir() string      { return "/tmp/agentmgr/cache" }
func (p *benchPlatform) GetLogDir() string        { return "/tmp/agentmgr/logs" }
func (p *benchPlatform) GetIPCSocketPath() string { return "/tmp/agentmgr.sock" }
func (p *benchPlatform) EnableAutoStart(_ context.Context) error {
	return nil
}
func (p *benchPlatform) DisableAutoStart(_ context.Context) error {
	return nil
}
func (p *benchPlatform) IsAutoStartEnabled(_ context.Context) (bool, error) {
	return false, nil
}
func (p *benchPlatform) FindExecutable(_ string) (string, error)    { return "", nil }
func (p *benchPlatform) FindExecutables(_ string) ([]string, error) { return nil, nil }
func (p *benchPlatform) IsExecutableInPath(_ string) bool           { return false }
func (p *benchPlatform) GetPathDirs() []string                      { return nil }
func (p *benchPlatform) GetShell() string                           { return "/bin/zsh" }
func (p *benchPlatform) GetShellArg() string                        { return "-c" }
func (p *benchPlatform) ShowNotification(_, _ string) error         { return nil }
func (p *benchPlatform) ShowChangelogDialog(_, _, _, _ string) platform.DialogResult {
	return platform.DialogResultCancel
}
