package detector

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

func TestPluginStrategy_Name(t *testing.T) {
	cfg := PluginConfig{
		Name:    "test-plugin",
		Method:  "custom",
		Enabled: true,
	}
	strategy := NewPluginStrategy(cfg, &mockPlatform{})

	if got := strategy.Name(); got != "test-plugin" {
		t.Errorf("Name() = %q, want %q", got, "test-plugin")
	}
}

func TestPluginStrategy_Method(t *testing.T) {
	cfg := PluginConfig{
		Name:    "test-plugin",
		Method:  "docker",
		Enabled: true,
	}
	strategy := NewPluginStrategy(cfg, &mockPlatform{})

	if got := strategy.Method(); got != "docker" {
		t.Errorf("Method() = %q, want %q", got, "docker")
	}
}

func TestPluginStrategy_IsApplicable(t *testing.T) {
	tests := []struct {
		name       string
		cfg        PluginConfig
		platformID platform.ID
		want       bool
	}{
		{
			name: "enabled with no platform filter",
			cfg: PluginConfig{
				Name:    "test",
				Method:  "custom",
				Enabled: true,
			},
			platformID: platform.Darwin,
			want:       true,
		},
		{
			name: "disabled plugin",
			cfg: PluginConfig{
				Name:    "test",
				Method:  "custom",
				Enabled: false,
			},
			platformID: platform.Darwin,
			want:       false,
		},
		{
			name: "platform filter matches",
			cfg: PluginConfig{
				Name:      "test",
				Method:    "custom",
				Enabled:   true,
				Platforms: []string{"darwin", "linux"},
			},
			platformID: platform.Darwin,
			want:       true,
		},
		{
			name: "platform filter does not match",
			cfg: PluginConfig{
				Name:      "test",
				Method:    "custom",
				Enabled:   true,
				Platforms: []string{"linux"},
			},
			platformID: platform.Darwin,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewPluginStrategy(tt.cfg, &mockPlatform{id: tt.platformID})
			if got := strategy.IsApplicable(&mockPlatform{id: tt.platformID}); got != tt.want {
				t.Errorf("IsApplicable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluginStrategy_Detect_WithScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows - requires /bin/sh")
	}
	cfg := PluginConfig{
		Name:         "echo-plugin",
		Method:       "custom",
		DetectScript: `echo '{"agents":[{"agent_id":"test-agent","version":"1.0.0","executable_path":"/usr/bin/test"}]}'`,
		Enabled:      true,
	}

	agents := []catalog.AgentDef{
		{
			ID:   "test-agent",
			Name: "Test Agent",
		},
	}

	strategy := NewPluginStrategy(cfg, &mockPlatform{})
	installations, err := strategy.Detect(context.Background(), agents)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if len(installations) != 1 {
		t.Fatalf("Detect() returned %d installations, want 1", len(installations))
	}

	inst := installations[0]
	if inst.AgentID != "test-agent" {
		t.Errorf("AgentID = %q, want %q", inst.AgentID, "test-agent")
	}
	if inst.InstalledVersion.String() != "1.0.0" {
		t.Errorf("Version = %q, want %q", inst.InstalledVersion.String(), "1.0.0")
	}
	if inst.ExecutablePath != "/usr/bin/test" {
		t.Errorf("ExecutablePath = %q, want %q", inst.ExecutablePath, "/usr/bin/test")
	}
}

func TestPluginStrategy_Detect_WithAgentFilter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows - requires /bin/sh")
	}
	cfg := PluginConfig{
		Name:         "filter-plugin",
		Method:       "custom",
		DetectScript: `echo '{"agents":[]}'`,
		AgentFilter:  []string{"specific-agent"},
		Enabled:      true,
	}

	agents := []catalog.AgentDef{
		{ID: "other-agent", Name: "Other Agent"},
		{ID: "specific-agent", Name: "Specific Agent"},
	}

	strategy := NewPluginStrategy(cfg, &mockPlatform{})

	// Should filter to only specific-agent
	installations, err := strategy.Detect(context.Background(), agents)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	// Script returns empty, so 0 installations expected
	if len(installations) != 0 {
		t.Errorf("Detect() returned %d installations, want 0", len(installations))
	}
}

func TestPluginStrategy_Detect_NoCommand(t *testing.T) {
	cfg := PluginConfig{
		Name:    "empty-plugin",
		Method:  "custom",
		Enabled: true,
	}

	strategy := NewPluginStrategy(cfg, &mockPlatform{})
	_, err := strategy.Detect(context.Background(), []catalog.AgentDef{{ID: "test"}})
	if err == nil {
		t.Error("Detect() should error with no command or script")
	}
}

func TestPluginRegistry_Register(t *testing.T) {
	registry := NewPluginRegistry(&mockPlatform{})

	cfg := PluginConfig{
		Name:         "test-plugin",
		Method:       "custom",
		DetectScript: "echo '{}'",
		Enabled:      true,
	}

	if err := registry.Register(cfg); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, ok := registry.Get("test-plugin")
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if got.Name != "test-plugin" {
		t.Errorf("Get().Name = %q, want %q", got.Name, "test-plugin")
	}
}

func TestPluginRegistry_Register_Validation(t *testing.T) {
	registry := NewPluginRegistry(&mockPlatform{})

	tests := []struct {
		name    string
		cfg     PluginConfig
		wantErr bool
	}{
		{
			name:    "missing name",
			cfg:     PluginConfig{Method: "custom", DetectScript: "echo"},
			wantErr: true,
		},
		{
			name:    "missing method",
			cfg:     PluginConfig{Name: "test", DetectScript: "echo"},
			wantErr: true,
		},
		{
			name:    "missing command and script",
			cfg:     PluginConfig{Name: "test", Method: "custom"},
			wantErr: true,
		},
		{
			name:    "valid config",
			cfg:     PluginConfig{Name: "test", Method: "custom", DetectScript: "echo"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Register(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPluginRegistry_Unregister(t *testing.T) {
	registry := NewPluginRegistry(&mockPlatform{})

	cfg := PluginConfig{
		Name:         "test-plugin",
		Method:       "custom",
		DetectScript: "echo",
		Enabled:      true,
	}

	_ = registry.Register(cfg)
	registry.Unregister("test-plugin")

	_, ok := registry.Get("test-plugin")
	if ok {
		t.Error("Get() returned true after Unregister(), want false")
	}
}

func TestPluginRegistry_List(t *testing.T) {
	registry := NewPluginRegistry(&mockPlatform{})

	configs := []PluginConfig{
		{Name: "plugin-a", Method: "custom", DetectScript: "echo"},
		{Name: "plugin-b", Method: "docker", DetectScript: "echo"},
	}

	for _, cfg := range configs {
		_ = registry.Register(cfg)
	}

	list := registry.List()
	if len(list) != 2 {
		t.Errorf("List() returned %d plugins, want 2", len(list))
	}
}

func TestPluginRegistry_GetStrategies(t *testing.T) {
	registry := NewPluginRegistry(&mockPlatform{})

	configs := []PluginConfig{
		{Name: "enabled", Method: "custom", DetectScript: "echo", Enabled: true},
		{Name: "disabled", Method: "docker", DetectScript: "echo", Enabled: false},
	}

	for _, cfg := range configs {
		_ = registry.Register(cfg)
	}

	strategies := registry.GetStrategies()
	if len(strategies) != 1 {
		t.Errorf("GetStrategies() returned %d strategies, want 1 (only enabled)", len(strategies))
	}
}

func TestPluginRegistry_LoadPluginsFromDir(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create a valid plugin file
	validPlugin := PluginConfig{
		Name:         "file-plugin",
		Description:  "Test plugin loaded from file",
		Method:       "custom",
		DetectScript: "echo '{\"agents\":[]}'",
		Enabled:      true,
	}

	data, _ := json.MarshalIndent(validPlugin, "", "  ")
	pluginPath := filepath.Join(tmpDir, "file-plugin.plugin.json")
	if err := os.WriteFile(pluginPath, data, 0644); err != nil {
		t.Fatalf("Failed to write plugin file: %v", err)
	}

	// Create an invalid file (wrong extension)
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write invalid file: %v", err)
	}

	registry := NewPluginRegistry(&mockPlatform{})
	if err := registry.LoadPluginsFromDir(tmpDir); err != nil {
		t.Fatalf("LoadPluginsFromDir() error = %v", err)
	}

	list := registry.List()
	if len(list) != 1 {
		t.Errorf("LoadPluginsFromDir() loaded %d plugins, want 1", len(list))
	}

	got, ok := registry.Get("file-plugin")
	if !ok {
		t.Fatal("Get() returned false for file-plugin")
	}
	if got.Description != "Test plugin loaded from file" {
		t.Errorf("Description = %q, want %q", got.Description, "Test plugin loaded from file")
	}
}

func TestPluginRegistry_LoadPluginsFromDir_NotExists(t *testing.T) {
	registry := NewPluginRegistry(&mockPlatform{})
	err := registry.LoadPluginsFromDir("/nonexistent/path")
	if err != nil {
		t.Errorf("LoadPluginsFromDir() should not error for nonexistent dir, got %v", err)
	}
}

func TestValidatePlugin(t *testing.T) {
	tests := []struct {
		name    string
		cfg     PluginConfig
		wantErr bool
	}{
		{
			name:    "valid plugin",
			cfg:     PluginConfig{Name: "test-plugin", Method: "custom", DetectScript: "echo"},
			wantErr: false,
		},
		{
			name:    "empty name",
			cfg:     PluginConfig{Name: "", Method: "custom", DetectScript: "echo"},
			wantErr: true,
		},
		{
			name:    "invalid name format",
			cfg:     PluginConfig{Name: "Test Plugin", Method: "custom", DetectScript: "echo"},
			wantErr: true,
		},
		{
			name:    "empty method",
			cfg:     PluginConfig{Name: "test", Method: "", DetectScript: "echo"},
			wantErr: true,
		},
		{
			name:    "no command or script",
			cfg:     PluginConfig{Name: "test", Method: "custom"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePlugin(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePlugin() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// mockPlatform implements platform.Platform for testing
type mockPlatform struct {
	id platform.ID
}

func (m *mockPlatform) ID() platform.ID {
	if m.id == "" {
		return platform.Darwin
	}
	return m.id
}

func (m *mockPlatform) Architecture() string                                        { return "amd64" }
func (m *mockPlatform) Name() string                                                { return "Test" }
func (m *mockPlatform) GetDataDir() string                                          { return "/tmp/data" }
func (m *mockPlatform) GetConfigDir() string                                        { return "/tmp/config" }
func (m *mockPlatform) GetCacheDir() string                                         { return "/tmp/cache" }
func (m *mockPlatform) GetLogDir() string                                           { return "/tmp/log" }
func (m *mockPlatform) GetIPCSocketPath() string                                    { return "/tmp/agentmgr.sock" }
func (m *mockPlatform) EnableAutoStart(ctx context.Context) error                   { return nil }
func (m *mockPlatform) DisableAutoStart(ctx context.Context) error                  { return nil }
func (m *mockPlatform) IsAutoStartEnabled(ctx context.Context) (bool, error)        { return false, nil }
func (m *mockPlatform) FindExecutable(name string) (string, error)                  { return "", nil }
func (m *mockPlatform) FindExecutables(name string) ([]string, error)               { return nil, nil }
func (m *mockPlatform) IsExecutableInPath(name string) bool                         { return false }
func (m *mockPlatform) GetPathDirs() []string                                       { return nil }
func (m *mockPlatform) GetShell() string                                            { return "/bin/sh" }
func (m *mockPlatform) GetShellArg() string                                         { return "-c" }
func (m *mockPlatform) ShowNotification(title, message string) error                { return nil }
func (m *mockPlatform) ShowChangelogDialog(a, b, c, d string) platform.DialogResult { return 0 }
