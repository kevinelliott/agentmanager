package catalog

import (
	"testing"
)

// BenchmarkCatalogSearch benchmarks the catalog search functionality.
func BenchmarkCatalogSearch(b *testing.B) {
	cat := benchmarkCatalog()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cat.Search("claude")
	}
}

// BenchmarkCatalogGetAgents benchmarks listing all agents.
func BenchmarkCatalogGetAgents(b *testing.B) {
	cat := benchmarkCatalog()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cat.GetAgents()
	}
}

// BenchmarkCatalogGetAgent benchmarks getting a single agent by ID.
func BenchmarkCatalogGetAgent(b *testing.B) {
	cat := benchmarkCatalog()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cat.GetAgent("claude-code")
	}
}

// BenchmarkCatalogGetAgentsByPlatform benchmarks filtering agents by platform.
func BenchmarkCatalogGetAgentsByPlatform(b *testing.B) {
	cat := benchmarkCatalog()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cat.GetAgentsByPlatform("darwin")
	}
}

// BenchmarkCatalogGetAgentsWithManyAgents benchmarks listing with many agents.
func BenchmarkCatalogGetAgentsWithManyAgents(b *testing.B) {
	cat := createLargeCatalog(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cat.GetAgents()
	}
}

// BenchmarkCatalogSearchWithManyAgents benchmarks search with many agents.
func BenchmarkCatalogSearchWithManyAgents(b *testing.B) {
	cat := createLargeCatalog(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cat.Search("agent")
	}
}

// benchmarkCatalog creates a catalog with a few test agents for benchmarking.
func benchmarkCatalog() *Catalog {
	return &Catalog{
		Version:       "1.0.0",
		SchemaVersion: 1,
		Agents: map[string]AgentDef{
			"claude-code": {
				ID:          "claude-code",
				Name:        "Claude Code",
				Description: "Anthropic's CLI for Claude",
				InstallMethods: map[string]InstallMethodDef{
					"npm": {
						Method:    "npm",
						Package:   "@anthropic/claude-code",
						Command:   "npm install -g @anthropic/claude-code",
						Platforms: []string{"darwin", "linux", "windows"},
					},
				},
				Detection: DetectionDef{
					Executables: []string{"claude"},
					VersionCmd:  "claude --version",
				},
			},
			"aider": {
				ID:          "aider",
				Name:        "Aider",
				Description: "AI pair programming",
				InstallMethods: map[string]InstallMethodDef{
					"pip": {
						Method:    "pip",
						Package:   "aider-chat",
						Command:   "pip install aider-chat",
						Platforms: []string{"darwin", "linux", "windows"},
					},
				},
				Detection: DetectionDef{
					Executables: []string{"aider"},
					VersionCmd:  "aider --version",
				},
			},
			"amp": {
				ID:          "amp",
				Name:        "Amp",
				Description: "Coding agent by Sourcegraph",
				InstallMethods: map[string]InstallMethodDef{
					"npm": {
						Method:    "npm",
						Package:   "@anthropic/amp",
						Command:   "npm install -g @anthropic/amp",
						Platforms: []string{"darwin", "linux"},
					},
				},
				Detection: DetectionDef{
					Executables: []string{"amp"},
					VersionCmd:  "amp --version",
				},
			},
		},
	}
}

// createLargeCatalog creates a catalog with many agents for benchmarking.
func createLargeCatalog(count int) *Catalog {
	agents := make(map[string]AgentDef, count)
	for i := 0; i < count; i++ {
		id := "agent-" + string(rune('a'+i%26)) + "-" + string(rune('0'+i/26))
		agents[id] = AgentDef{
			ID:          id,
			Name:        "Agent " + id,
			Description: "Test agent number " + string(rune('0'+i)),
			InstallMethods: map[string]InstallMethodDef{
				"npm": {
					Method:    "npm",
					Package:   "test-" + id,
					Command:   "npm install -g test-" + id,
					Platforms: []string{"darwin", "linux", "windows"},
				},
			},
			Detection: DetectionDef{
				Executables: []string{id},
				VersionCmd:  id + " --version",
			},
		}
	}
	return &Catalog{
		Version:       "1.0.0",
		SchemaVersion: 1,
		Agents:        agents,
	}
}
