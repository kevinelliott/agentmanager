package cli

import (
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmgr/pkg/config"
)

// findSubcommand returns a subcommand by name, or nil if not found.
func findSubcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}

// assertSubcommandExists checks that a subcommand exists.
func assertSubcommandExists(t *testing.T, cmd *cobra.Command, name string) *cobra.Command {
	t.Helper()
	sub := findSubcommand(cmd, name)
	if sub == nil {
		t.Errorf("expected subcommand %q to exist, but it was not found", name)
	}
	return sub
}

// assertFlagExists checks that a flag exists on the command.
func assertFlagExists(t *testing.T, cmd *cobra.Command, name string) {
	t.Helper()
	if cmd.Flags().Lookup(name) == nil && cmd.PersistentFlags().Lookup(name) == nil {
		t.Errorf("expected flag %q to exist on command %q", name, cmd.Name())
	}
}

func TestNewRootCommand(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewRootCommand(cfg, "1.0.0", "abc123", "2024-01-01")

	// Verify Use and Short
	if cmd.Use != "agentmgr" {
		t.Errorf("Use = %q, want %q", cmd.Use, "agentmgr")
	}
	if cmd.Short != "AI Development Agent Manager" {
		t.Errorf("Short = %q, want %q", cmd.Short, "AI Development Agent Manager")
	}

	// Verify SilenceUsage and SilenceErrors
	if !cmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
	if !cmd.SilenceErrors {
		t.Error("SilenceErrors should be true")
	}

	// Check expected subcommands
	expectedSubcommands := []string{"agent", "catalog", "completion", "config", "doctor", "helper", "tui", "upgrade", "version"}
	subCmds := cmd.Commands()
	if len(subCmds) < len(expectedSubcommands) {
		t.Errorf("Expected at least %d subcommands, got %d", len(expectedSubcommands), len(subCmds))
	}

	for _, name := range expectedSubcommands {
		assertSubcommandExists(t, cmd, name)
	}

	// Check persistent flags
	assertFlagExists(t, cmd, "config")
	assertFlagExists(t, cmd, "verbose")
	assertFlagExists(t, cmd, "format")
	assertFlagExists(t, cmd, "no-color")

	// Verify flag shorthand
	if flag := cmd.PersistentFlags().ShorthandLookup("c"); flag == nil {
		t.Error("expected -c shorthand for --config flag")
	}
	if flag := cmd.PersistentFlags().ShorthandLookup("v"); flag == nil {
		t.Error("expected -v shorthand for --verbose flag")
	}
	if flag := cmd.PersistentFlags().ShorthandLookup("f"); flag == nil {
		t.Error("expected -f shorthand for --format flag")
	}
}

func TestNewAgentCommand(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewAgentCommand(cfg)

	// Verify Use and Short
	if cmd.Use != "agent" {
		t.Errorf("Use = %q, want %q", cmd.Use, "agent")
	}
	if cmd.Short != "Manage AI development agents" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Manage AI development agents")
	}

	// Verify alias
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "agents" {
		t.Errorf("Aliases = %v, want [agents]", cmd.Aliases)
	}

	// Check expected subcommands
	expectedSubcommands := []string{"list", "install", "update", "info", "remove", "refresh"}
	for _, name := range expectedSubcommands {
		assertSubcommandExists(t, cmd, name)
	}

	// Verify list subcommand has expected flags
	listCmd := findSubcommand(cmd, "list")
	if listCmd != nil {
		assertFlagExists(t, listCmd, "all")
		assertFlagExists(t, listCmd, "hidden")
		assertFlagExists(t, listCmd, "format")
		assertFlagExists(t, listCmd, "updates")
		assertFlagExists(t, listCmd, "refresh")

		// Check alias
		if len(listCmd.Aliases) == 0 || listCmd.Aliases[0] != "ls" {
			t.Errorf("list command Aliases = %v, want [ls]", listCmd.Aliases)
		}
	}

	// Verify install subcommand
	installCmd := findSubcommand(cmd, "install")
	if installCmd != nil {
		assertFlagExists(t, installCmd, "method")
	}

	// Verify update subcommand
	updateCmd := findSubcommand(cmd, "update")
	if updateCmd != nil {
		assertFlagExists(t, updateCmd, "all")
		assertFlagExists(t, updateCmd, "force")
	}

	// Verify remove subcommand
	removeCmd := findSubcommand(cmd, "remove")
	if removeCmd != nil {
		assertFlagExists(t, removeCmd, "force")
		assertFlagExists(t, removeCmd, "method")
	}
}

func TestNewCatalogCommand(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewCatalogCommand(cfg)

	// Verify Use and Short
	if cmd.Use != "catalog" {
		t.Errorf("Use = %q, want %q", cmd.Use, "catalog")
	}
	if cmd.Short != "Manage the agent catalog" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Manage the agent catalog")
	}

	// Check expected subcommands
	expectedSubcommands := []string{"list", "refresh", "search", "show"}
	for _, name := range expectedSubcommands {
		assertSubcommandExists(t, cmd, name)
	}

	// Verify list subcommand has expected flags
	listCmd := findSubcommand(cmd, "list")
	if listCmd != nil {
		assertFlagExists(t, listCmd, "format")
		assertFlagExists(t, listCmd, "platform")

		// Check alias
		if len(listCmd.Aliases) == 0 || listCmd.Aliases[0] != "ls" {
			t.Errorf("list command Aliases = %v, want [ls]", listCmd.Aliases)
		}
	}

	// Verify refresh subcommand
	refreshCmd := findSubcommand(cmd, "refresh")
	if refreshCmd != nil {
		assertFlagExists(t, refreshCmd, "force")
	}

	// Verify search subcommand requires exactly 1 arg
	searchCmd := findSubcommand(cmd, "search")
	if searchCmd != nil {
		assertFlagExists(t, searchCmd, "format")
	}

	// Verify show subcommand
	showCmd := findSubcommand(cmd, "show")
	if showCmd != nil {
		assertFlagExists(t, showCmd, "format")
	}
}

func TestNewConfigCommand(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewConfigCommand(cfg)

	// Verify Use and Short
	if cmd.Use != "config" {
		t.Errorf("Use = %q, want %q", cmd.Use, "config")
	}
	if cmd.Short != "Manage configuration" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Manage configuration")
	}

	// Check expected subcommands
	expectedSubcommands := []string{"show", "get", "set", "path", "init"}
	for _, name := range expectedSubcommands {
		assertSubcommandExists(t, cmd, name)
	}

	// Verify init subcommand has force flag
	initCmd := findSubcommand(cmd, "init")
	if initCmd != nil {
		assertFlagExists(t, initCmd, "force")
	}
}

func TestNewHelperCommand(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewHelperCommand(cfg)

	// Verify Use and Short
	if cmd.Use != "helper" {
		t.Errorf("Use = %q, want %q", cmd.Use, "helper")
	}
	if cmd.Short != "Manage the systray helper" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Manage the systray helper")
	}

	// Check expected subcommands
	expectedSubcommands := []string{"start", "stop", "status", "autostart"}
	for _, name := range expectedSubcommands {
		assertSubcommandExists(t, cmd, name)
	}

	// Verify start subcommand has foreground flag
	startCmd := findSubcommand(cmd, "start")
	if startCmd != nil {
		assertFlagExists(t, startCmd, "foreground")
	}

	// Verify autostart has enable/disable subcommands
	autoStartCmd := findSubcommand(cmd, "autostart")
	if autoStartCmd != nil {
		assertSubcommandExists(t, autoStartCmd, "enable")
		assertSubcommandExists(t, autoStartCmd, "disable")
	}
}

func TestNewVersionCommand(t *testing.T) {
	version := "1.2.3"
	commit := "abc123def"
	date := "2024-06-15"

	cmd := NewVersionCommand(version, commit, date)

	// Verify Use and Short
	if cmd.Use != "version" {
		t.Errorf("Use = %q, want %q", cmd.Use, "version")
	}
	if cmd.Short != "Show version information" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Show version information")
	}

	// Verify json flag exists
	assertFlagExists(t, cmd, "json")

	// Verify command has a Run function (not RunE)
	if cmd.Run == nil {
		t.Error("expected Run function to be set")
	}
}

func TestNewCompletionCommand(t *testing.T) {
	cmd := NewCompletionCommand()

	// Verify Use and Short
	if cmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("Use = %q, want %q", cmd.Use, "completion [bash|zsh|fish|powershell]")
	}
	if cmd.Short != "Generate shell completion scripts" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Generate shell completion scripts")
	}

	// Verify ValidArgs
	expectedValidArgs := []string{"bash", "zsh", "fish", "powershell"}
	if len(cmd.ValidArgs) != len(expectedValidArgs) {
		t.Errorf("ValidArgs length = %d, want %d", len(cmd.ValidArgs), len(expectedValidArgs))
	}
	for i, arg := range expectedValidArgs {
		if i < len(cmd.ValidArgs) && cmd.ValidArgs[i] != arg {
			t.Errorf("ValidArgs[%d] = %q, want %q", i, cmd.ValidArgs[i], arg)
		}
	}

	// Verify DisableFlagsInUseLine
	if !cmd.DisableFlagsInUseLine {
		t.Error("DisableFlagsInUseLine should be true")
	}

	// Verify command has a RunE function
	if cmd.RunE == nil {
		t.Error("expected RunE function to be set")
	}
}

func TestNewTUICommand(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewTUICommand(cfg)

	// Verify Use and Short
	if cmd.Use != "tui" {
		t.Errorf("Use = %q, want %q", cmd.Use, "tui")
	}
	if cmd.Short != "Launch the terminal user interface" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Launch the terminal user interface")
	}

	// Verify command has Long description
	if cmd.Long == "" {
		t.Error("expected Long description to be set")
	}

	// Verify command has a RunE function
	if cmd.RunE == nil {
		t.Error("expected RunE function to be set")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"zero", 0, "0s"},
		{"seconds", 45 * time.Second, "45s"},
		{"one minute", time.Minute, "1m 0s"},
		{"minutes and seconds", 5*time.Minute + 30*time.Second, "5m 30s"},
		{"one hour", time.Hour, "1h 0m"},
		{"hours and minutes", 3*time.Hour + 15*time.Minute, "3h 15m"},
		{"one day", 24 * time.Hour, "1d 0h"},
		{"days and hours", 2*24*time.Hour + 5*time.Hour, "2d 5h"},
		{"many days", 10*24*time.Hour + 12*time.Hour, "10d 12h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestParseConfigValue(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected interface{}
	}{
		// Boolean keys
		{"bool true lowercase", "catalog.refresh_on_start", "true", true},
		{"bool true uppercase", "catalog.refresh_on_start", "TRUE", true},
		{"bool true yes", "updates.auto_check", "yes", true},
		{"bool true 1", "updates.notify", "1", true},
		{"bool false lowercase", "updates.auto_update", "false", false},
		{"bool false no", "ui.show_hidden", "no", false},
		{"bool false 0", "ui.use_colors", "0", false},
		{"bool false random", "ui.compact_mode", "random", false},
		{"bool api enable grpc", "api.enable_grpc", "true", true},
		{"bool api enable rest", "api.enable_rest", "false", false},
		{"bool api require auth", "api.require_auth", "yes", true},

		// Integer keys
		{"int ui page size", "ui.page_size", "50", 50},
		{"int grpc port", "api.grpc_port", "50051", 50051},
		{"int rest port", "api.rest_port", "8080", 8080},
		{"int logging max size", "logging.max_size", "100", 100},
		{"int logging max age", "logging.max_age", "30", 30},
		{"int invalid returns string", "ui.page_size", "invalid", "invalid"},

		// Duration keys
		{"duration refresh interval", "catalog.refresh_interval", "1h", time.Hour},
		{"duration check interval", "updates.check_interval", "30m", 30 * time.Minute},
		{"duration with seconds", "catalog.refresh_interval", "1h30m45s", time.Hour + 30*time.Minute + 45*time.Second},
		{"duration invalid returns string", "catalog.refresh_interval", "invalid", "invalid"},

		// String keys (default)
		{"string unknown key", "unknown.key", "some value", "some value"},
		{"string catalog url", "catalog.source_url", "https://example.com", "https://example.com"},
		{"string with spaces", "some.key", "value with spaces", "value with spaces"},

		// Case insensitive keys
		{"case insensitive bool", "CATALOG.REFRESH_ON_START", "true", true},
		{"case insensitive int", "UI.PAGE_SIZE", "25", 25},
		{"case insensitive duration", "CATALOG.REFRESH_INTERVAL", "2h", 2 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseConfigValue(tt.key, tt.value)
			if result != tt.expected {
				t.Errorf("parseConfigValue(%q, %q) = %v (%T), want %v (%T)",
					tt.key, tt.value, result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestParseConfigValueAllBoolKeys(t *testing.T) {
	boolKeys := []string{
		"catalog.refresh_on_start",
		"updates.auto_check",
		"updates.notify",
		"updates.auto_update",
		"ui.show_hidden",
		"ui.use_colors",
		"ui.compact_mode",
		"api.enable_grpc",
		"api.enable_rest",
		"api.require_auth",
	}

	for _, key := range boolKeys {
		t.Run(key, func(t *testing.T) {
			// Test that "true" returns true
			result := parseConfigValue(key, "true")
			if result != true {
				t.Errorf("parseConfigValue(%q, %q) = %v, want true", key, "true", result)
			}

			// Test that "false" returns false
			result = parseConfigValue(key, "false")
			if result != false {
				t.Errorf("parseConfigValue(%q, %q) = %v, want false", key, "false", result)
			}
		})
	}
}

func TestParseConfigValueAllIntKeys(t *testing.T) {
	intKeys := []string{
		"ui.page_size",
		"api.grpc_port",
		"api.rest_port",
		"logging.max_size",
		"logging.max_age",
	}

	for _, key := range intKeys {
		t.Run(key, func(t *testing.T) {
			// Test that a valid integer is parsed
			result := parseConfigValue(key, "42")
			if result != 42 {
				t.Errorf("parseConfigValue(%q, %q) = %v (%T), want 42 (int)", key, "42", result, result)
			}
		})
	}
}

func TestParseConfigValueAllDurationKeys(t *testing.T) {
	durationKeys := []string{
		"catalog.refresh_interval",
		"updates.check_interval",
	}

	for _, key := range durationKeys {
		t.Run(key, func(t *testing.T) {
			// Test that a valid duration is parsed
			result := parseConfigValue(key, "1h")
			if result != time.Hour {
				t.Errorf("parseConfigValue(%q, %q) = %v (%T), want 1h (time.Duration)", key, "1h", result, result)
			}
		})
	}
}

func TestRootCommandSubcommandCount(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewRootCommand(cfg, "1.0.0", "abc123", "2024-01-01")

	// Verify we have exactly the expected number of subcommands
	// This helps catch if subcommands are accidentally removed
	expectedCount := 11 // agent, api, catalog, completion, config, doctor, helper, plugin, tui, upgrade, version
	actualCount := len(cmd.Commands())

	if actualCount != expectedCount {
		t.Errorf("subcommand count = %d, want %d", actualCount, expectedCount)
		t.Log("Subcommands found:")
		for _, sub := range cmd.Commands() {
			t.Logf("  - %s", sub.Name())
		}
	}
}

func TestAgentCommandSubcommandCount(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewAgentCommand(cfg)

	expectedCount := 6 // list, install, update, info, remove, refresh
	actualCount := len(cmd.Commands())

	if actualCount != expectedCount {
		t.Errorf("subcommand count = %d, want %d", actualCount, expectedCount)
		t.Log("Subcommands found:")
		for _, sub := range cmd.Commands() {
			t.Logf("  - %s", sub.Name())
		}
	}
}

func TestCatalogCommandSubcommandCount(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewCatalogCommand(cfg)

	expectedCount := 4 // list, refresh, search, show
	actualCount := len(cmd.Commands())

	if actualCount != expectedCount {
		t.Errorf("subcommand count = %d, want %d", actualCount, expectedCount)
		t.Log("Subcommands found:")
		for _, sub := range cmd.Commands() {
			t.Logf("  - %s", sub.Name())
		}
	}
}

func TestConfigCommandSubcommandCount(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewConfigCommand(cfg)

	expectedCount := 7 // show, get, set, path, init, export, import
	actualCount := len(cmd.Commands())

	if actualCount != expectedCount {
		t.Errorf("subcommand count = %d, want %d", actualCount, expectedCount)
		t.Log("Subcommands found:")
		for _, sub := range cmd.Commands() {
			t.Logf("  - %s", sub.Name())
		}
	}
}

func TestHelperCommandSubcommandCount(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewHelperCommand(cfg)

	expectedCount := 4 // start, stop, status, autostart
	actualCount := len(cmd.Commands())

	if actualCount != expectedCount {
		t.Errorf("subcommand count = %d, want %d", actualCount, expectedCount)
		t.Log("Subcommands found:")
		for _, sub := range cmd.Commands() {
			t.Logf("  - %s", sub.Name())
		}
	}
}

func TestHelperAutoStartSubcommandCount(t *testing.T) {
	cfg := &config.Config{}
	cmd := NewHelperCommand(cfg)
	autoStartCmd := findSubcommand(cmd, "autostart")

	if autoStartCmd == nil {
		t.Fatal("autostart subcommand not found")
	}

	expectedCount := 2 // enable, disable
	actualCount := len(autoStartCmd.Commands())

	if actualCount != expectedCount {
		t.Errorf("autostart subcommand count = %d, want %d", actualCount, expectedCount)
	}
}
