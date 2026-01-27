package tui

import (
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

func TestViewConstants(t *testing.T) {
	// Verify view constants are defined correctly
	tests := []struct {
		view     View
		expected int
	}{
		{ViewDashboard, 0},
		{ViewAgentList, 1},
		{ViewAgentDetail, 2},
		{ViewCatalog, 3},
		{ViewSettings, 4},
	}

	for _, tt := range tests {
		if int(tt.view) != tt.expected {
			t.Errorf("View constant = %d, want %d", tt.view, tt.expected)
		}
	}
}

func TestDefaultKeyMap(t *testing.T) {
	keys := DefaultKeyMap()

	// Test that all key bindings are defined
	t.Run("Up binding", func(t *testing.T) {
		if len(keys.Up.Keys()) == 0 {
			t.Error("Up key binding has no keys")
		}
	})

	t.Run("Down binding", func(t *testing.T) {
		if len(keys.Down.Keys()) == 0 {
			t.Error("Down key binding has no keys")
		}
	})

	t.Run("Enter binding", func(t *testing.T) {
		if len(keys.Enter.Keys()) == 0 {
			t.Error("Enter key binding has no keys")
		}
	})

	t.Run("Back binding", func(t *testing.T) {
		if len(keys.Back.Keys()) == 0 {
			t.Error("Back key binding has no keys")
		}
	})

	t.Run("Quit binding", func(t *testing.T) {
		if len(keys.Quit.Keys()) == 0 {
			t.Error("Quit key binding has no keys")
		}
	})

	t.Run("Refresh binding", func(t *testing.T) {
		if len(keys.Refresh.Keys()) == 0 {
			t.Error("Refresh key binding has no keys")
		}
	})

	t.Run("Install binding", func(t *testing.T) {
		if len(keys.Install.Keys()) == 0 {
			t.Error("Install key binding has no keys")
		}
	})

	t.Run("Update binding", func(t *testing.T) {
		if len(keys.Update.Keys()) == 0 {
			t.Error("Update key binding has no keys")
		}
	})

	t.Run("Remove binding", func(t *testing.T) {
		if len(keys.Remove.Keys()) == 0 {
			t.Error("Remove key binding has no keys")
		}
	})

	t.Run("Help binding", func(t *testing.T) {
		if len(keys.Help.Keys()) == 0 {
			t.Error("Help key binding has no keys")
		}
	})

	t.Run("Tab binding", func(t *testing.T) {
		if len(keys.Tab.Keys()) == 0 {
			t.Error("Tab key binding has no keys")
		}
	})
}

func TestDefaultKeyMapKeys(t *testing.T) {
	keys := DefaultKeyMap()

	// Verify specific keys are bound
	tests := []struct {
		name     string
		binding  []string
		expected string
	}{
		{"Up has 'up'", keys.Up.Keys(), "up"},
		{"Up has 'k'", keys.Up.Keys(), "k"},
		{"Down has 'down'", keys.Down.Keys(), "down"},
		{"Down has 'j'", keys.Down.Keys(), "j"},
		{"Enter has 'enter'", keys.Enter.Keys(), "enter"},
		{"Quit has 'q'", keys.Quit.Keys(), "q"},
		{"Refresh has 'r'", keys.Refresh.Keys(), "r"},
		{"Install has 'i'", keys.Install.Keys(), "i"},
		{"Update has 'u'", keys.Update.Keys(), "u"},
		{"Remove has 'd'", keys.Remove.Keys(), "d"},
		{"Help has '?'", keys.Help.Keys(), "?"},
		{"Tab has 'tab'", keys.Tab.Keys(), "tab"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, k := range tt.binding {
				if k == tt.expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Key binding should contain %q, got %v", tt.expected, tt.binding)
			}
		})
	}
}

func TestAgentItem(t *testing.T) {
	version, _ := agent.ParseVersion("1.2.3")
	inst := &agent.Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		InstalledVersion: version,
	}

	item := agentItem{installation: inst}

	t.Run("Title", func(t *testing.T) {
		if item.Title() != "Claude Code" {
			t.Errorf("Title() = %q, want %q", item.Title(), "Claude Code")
		}
	})

	t.Run("Description", func(t *testing.T) {
		if item.Description() != "1.2.3" {
			t.Errorf("Description() = %q, want %q", item.Description(), "1.2.3")
		}
	})

	t.Run("FilterValue", func(t *testing.T) {
		if item.FilterValue() != "Claude Code" {
			t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), "Claude Code")
		}
	})
}

func TestAgentItemWithDifferentAgents(t *testing.T) {
	tests := []struct {
		agentID   string
		agentName string
		version   string
	}{
		{"claude-code", "Claude Code", "2.0.0"},
		{"aider", "Aider", "0.50.0"},
		{"copilot", "GitHub Copilot", "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.agentID, func(t *testing.T) {
			version, _ := agent.ParseVersion(tt.version)
			inst := &agent.Installation{
				AgentID:          tt.agentID,
				AgentName:        tt.agentName,
				InstalledVersion: version,
			}

			item := agentItem{installation: inst}

			if item.Title() != tt.agentName {
				t.Errorf("Title() = %q, want %q", item.Title(), tt.agentName)
			}
			if item.Description() != tt.version {
				t.Errorf("Description() = %q, want %q", item.Description(), tt.version)
			}
			if item.FilterValue() != tt.agentName {
				t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), tt.agentName)
			}
		})
	}
}
