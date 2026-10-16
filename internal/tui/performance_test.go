package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

func catalogModel(t testing.TB) Model {
	t.Helper()
	var cat catalog.Catalog
	if err := json.Unmarshal(catalog.EmbeddedJSON(), &cat); err != nil {
		t.Fatal(err)
	}
	m := New(config.Default(), platform.Current(), nil)
	t.Cleanup(m.cancel)
	m.catalog = &cat
	m.loading = false
	m.currentView = ViewCatalog
	m.width = 120
	m.height = 40
	m.rebuildCatalogRows()
	return m
}

func TestIdleSpinnerStopsAndRefreshRestarts(t *testing.T) {
	m := catalogModel(t)
	updated, cmd := m.Update(spinner.TickMsg{})
	if cmd != nil {
		t.Fatal("idle model scheduled another spinner tick")
	}
	m = updated.(Model)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("refresh failed to restart loading work")
	}
}

func TestLocalSnapshotIsVisibleBeforeVersions(t *testing.T) {
	m := New(config.Default(), platform.Current(), nil)
	defer m.cancel()
	inst := &agent.Installation{AgentID: "a", AgentName: "A", InstalledVersion: agent.MustParseVersion("1.0.0")}
	next, cmd := m.Update(dataLoadedMsg{agents: []*agent.Installation{inst}, checkVersions: true})
	m = next.(Model)
	if m.loading || !m.versionsLoading || len(m.agents) != 1 || cmd == nil {
		t.Fatal("local results wait for version metadata")
	}
	if inst.LatestVersion != nil {
		t.Fatal("visible snapshot was mutated")
	}
	next, _ = m.Update(versionsLoadedMsg{agents: []*agent.Installation{inst}})
	if next.(Model).versionsLoading {
		t.Fatal("version loading did not finish")
	}
}

func TestCatalogScrollAndResizeKeepRowsVisible(t *testing.T) {
	m := catalogModel(t)
	first := m.catalogView()
	if strings.Contains(first, m.catalogRows[len(m.catalogRows)-1]) {
		t.Fatal("catalog rendered off-screen rows")
	}
	for range len(m.catalogRows) {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
	}
	if !strings.Contains(m.catalogView(), m.catalogRows[len(m.catalogRows)-1]) {
		t.Fatal("last catalog row is unreachable")
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	m = next.(Model)
	if m.catalogOffset+m.catalogPageSize() > len(m.catalogRows) {
		t.Fatal("resize left an invalid viewport")
	}
}

func BenchmarkCatalogView(b *testing.B) {
	m := catalogModel(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
