package styles

import (
	"strings"
	"testing"
)

func TestColorConstants(t *testing.T) {
	// Verify color constants are defined
	colors := map[string]string{
		"Purple":    string(Purple),
		"Green":     string(Green),
		"Orange":    string(Orange),
		"Red":       string(Red),
		"Cyan":      string(Cyan),
		"Pink":      string(Pink),
		"Yellow":    string(Yellow),
		"White":     string(White),
		"Gray":      string(Gray),
		"DarkGray":  string(DarkGray),
		"BG":        string(BG),
		"CurrentBG": string(CurrentBG),
	}

	for name, value := range colors {
		if value == "" {
			t.Errorf("%s color is empty", name)
		}
		if !strings.HasPrefix(value, "#") {
			t.Errorf("%s color = %q, should start with #", name, value)
		}
	}
}

func TestDimensionConstants(t *testing.T) {
	if MinWidth <= 0 {
		t.Errorf("MinWidth = %d, should be positive", MinWidth)
	}
	if MinHeight <= 0 {
		t.Errorf("MinHeight = %d, should be positive", MinHeight)
	}
	if SidebarWidth <= 0 {
		t.Errorf("SidebarWidth = %d, should be positive", SidebarWidth)
	}
	if MaxWidth <= MinWidth {
		t.Errorf("MaxWidth = %d should be greater than MinWidth = %d", MaxWidth, MinWidth)
	}
}

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		status        string
		shouldContain string
	}{
		{"installed", "Installed"},
		{"update", "Update Available"},
		{"not_installed", "Not Installed"},
		{"error", "Error"},
		{"unknown", "unknown"}, // default case
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := FormatStatus(tt.status)
			if result == "" {
				t.Error("FormatStatus returned empty string")
			}
			// The result contains ANSI escape codes, so we check if the text is somewhere in there
			if !strings.Contains(result, tt.shouldContain) {
				t.Errorf("FormatStatus(%q) = %q, should contain %q", tt.status, result, tt.shouldContain)
			}
		})
	}
}

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		version   string
		hasUpdate bool
	}{
		{"1.0.0", false},
		{"1.0.0", true},
		{"2.5.3", false},
		{"0.0.1", true},
	}

	for _, tt := range tests {
		name := tt.version
		if tt.hasUpdate {
			name += "_with_update"
		}
		t.Run(name, func(t *testing.T) {
			result := FormatVersion(tt.version, tt.hasUpdate)
			if result == "" {
				t.Error("FormatVersion returned empty string")
			}
			// The version string should be in the result
			if !strings.Contains(result, tt.version) {
				t.Errorf("FormatVersion(%q, %v) = %q, should contain version", tt.version, tt.hasUpdate, result)
			}
		})
	}
}

func TestFormatBadge(t *testing.T) {
	tests := []struct {
		text    string
		variant string
	}{
		{"OK", "success"},
		{"Warning", "warning"},
		{"Error", "error"},
		{"Info", "default"},
		{"Custom", "unknown"}, // default case
	}

	for _, tt := range tests {
		t.Run(tt.variant, func(t *testing.T) {
			result := FormatBadge(tt.text, tt.variant)
			if result == "" {
				t.Error("FormatBadge returned empty string")
			}
			// The text should be in the result
			if !strings.Contains(result, tt.text) {
				t.Errorf("FormatBadge(%q, %q) = %q, should contain text", tt.text, tt.variant, result)
			}
		})
	}
}

func TestStylesAreDefined(t *testing.T) {
	// Test that all style variables are properly defined and don't panic
	// We just access them to ensure they're initialized

	styles := []struct {
		name  string
		style interface{}
	}{
		{"App", App},
		{"Title", Title},
		{"TitleBar", TitleBar},
		{"Subtitle", Subtitle},
		{"StatusBar", StatusBar},
		{"Help", Help},
		{"HelpKey", HelpKey},
		{"ListItem", ListItem},
		{"SelectedItem", SelectedItem},
		{"StatusInstalled", StatusInstalled},
		{"StatusUpdateAvailable", StatusUpdateAvailable},
		{"StatusNotInstalled", StatusNotInstalled},
		{"StatusError", StatusError},
		{"Version", Version},
		{"VersionOld", VersionOld},
		{"VersionNew", VersionNew},
		{"TableHeader", TableHeader},
		{"TableRow", TableRow},
		{"TableRowSelected", TableRowSelected},
		{"Box", Box},
		{"BoxFocused", BoxFocused},
		{"Button", Button},
		{"ButtonActive", ButtonActive},
		{"ButtonDanger", ButtonDanger},
		{"Badge", Badge},
		{"BadgeSuccess", BadgeSuccess},
		{"BadgeWarning", BadgeWarning},
		{"BadgeError", BadgeError},
		{"Input", Input},
		{"InputFocused", InputFocused},
		{"Tab", Tab},
		{"TabActive", TabActive},
		{"Spinner", Spinner},
		{"ErrorMessage", ErrorMessage},
		{"SuccessMessage", SuccessMessage},
		{"InfoMessage", InfoMessage},
		{"WarningMessage", WarningMessage},
	}

	for _, s := range styles {
		t.Run(s.name, func(t *testing.T) {
			if s.style == nil {
				t.Errorf("%s style is nil", s.name)
			}
		})
	}
}

func TestStylesRenderWithoutPanic(t *testing.T) {
	// Test that all styles can render text without panicking
	testText := "test"

	t.Run("App", func(t *testing.T) { _ = App.Render(testText) })
	t.Run("Title", func(t *testing.T) { _ = Title.Render(testText) })
	t.Run("TitleBar", func(t *testing.T) { _ = TitleBar.Render(testText) })
	t.Run("Subtitle", func(t *testing.T) { _ = Subtitle.Render(testText) })
	t.Run("StatusBar", func(t *testing.T) { _ = StatusBar.Render(testText) })
	t.Run("Help", func(t *testing.T) { _ = Help.Render(testText) })
	t.Run("HelpKey", func(t *testing.T) { _ = HelpKey.Render(testText) })
	t.Run("ListItem", func(t *testing.T) { _ = ListItem.Render(testText) })
	t.Run("SelectedItem", func(t *testing.T) { _ = SelectedItem.Render(testText) })
	t.Run("StatusInstalled", func(t *testing.T) { _ = StatusInstalled.Render(testText) })
	t.Run("StatusUpdateAvailable", func(t *testing.T) { _ = StatusUpdateAvailable.Render(testText) })
	t.Run("StatusNotInstalled", func(t *testing.T) { _ = StatusNotInstalled.Render(testText) })
	t.Run("StatusError", func(t *testing.T) { _ = StatusError.Render(testText) })
	t.Run("Version", func(t *testing.T) { _ = Version.Render(testText) })
	t.Run("VersionOld", func(t *testing.T) { _ = VersionOld.Render(testText) })
	t.Run("VersionNew", func(t *testing.T) { _ = VersionNew.Render(testText) })
	t.Run("TableHeader", func(t *testing.T) { _ = TableHeader.Render(testText) })
	t.Run("TableRow", func(t *testing.T) { _ = TableRow.Render(testText) })
	t.Run("TableRowSelected", func(t *testing.T) { _ = TableRowSelected.Render(testText) })
	t.Run("Box", func(t *testing.T) { _ = Box.Render(testText) })
	t.Run("BoxFocused", func(t *testing.T) { _ = BoxFocused.Render(testText) })
	t.Run("Button", func(t *testing.T) { _ = Button.Render(testText) })
	t.Run("ButtonActive", func(t *testing.T) { _ = ButtonActive.Render(testText) })
	t.Run("ButtonDanger", func(t *testing.T) { _ = ButtonDanger.Render(testText) })
	t.Run("Badge", func(t *testing.T) { _ = Badge.Render(testText) })
	t.Run("BadgeSuccess", func(t *testing.T) { _ = BadgeSuccess.Render(testText) })
	t.Run("BadgeWarning", func(t *testing.T) { _ = BadgeWarning.Render(testText) })
	t.Run("BadgeError", func(t *testing.T) { _ = BadgeError.Render(testText) })
	t.Run("Input", func(t *testing.T) { _ = Input.Render(testText) })
	t.Run("InputFocused", func(t *testing.T) { _ = InputFocused.Render(testText) })
	t.Run("Tab", func(t *testing.T) { _ = Tab.Render(testText) })
	t.Run("TabActive", func(t *testing.T) { _ = TabActive.Render(testText) })
	t.Run("Spinner", func(t *testing.T) { _ = Spinner.Render(testText) })
	t.Run("ErrorMessage", func(t *testing.T) { _ = ErrorMessage.Render(testText) })
	t.Run("SuccessMessage", func(t *testing.T) { _ = SuccessMessage.Render(testText) })
	t.Run("InfoMessage", func(t *testing.T) { _ = InfoMessage.Render(testText) })
	t.Run("WarningMessage", func(t *testing.T) { _ = WarningMessage.Render(testText) })
}
