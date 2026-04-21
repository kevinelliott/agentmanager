package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/config"
)

func TestNoColor(t *testing.T) {
	cases := []struct {
		name     string
		cfg      *config.Config
		explicit bool
		env      string
		want     bool
	}{
		{"nil cfg, no env, no flag", nil, false, "", false},
		{"explicit flag true", nil, true, "", true},
		{"NO_COLOR env set", nil, false, "1", true},
		{"cfg disables colors", &config.Config{UI: config.UIConfig{UseColors: false}}, false, "", true},
		{"cfg enables colors", &config.Config{UI: config.UIConfig{UseColors: true}}, false, "", false},
		{"flag overrides cfg", &config.Config{UI: config.UIConfig{UseColors: true}}, true, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tc.env)
			if got := NoColor(tc.cfg, tc.explicit); got != tc.want {
				t.Errorf("NoColor(%+v, %v) with NO_COLOR=%q = %v, want %v",
					tc.cfg, tc.explicit, tc.env, got, tc.want)
			}
		})
	}
}

func TestNewPrinter_NoColorDetection(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	p := NewPrinter(&config.Config{UI: config.UIConfig{UseColors: true}}, false)
	if p.NoColor() {
		t.Error("expected NoColor()=false when flag false + env unset + cfg true")
	}

	// NO_COLOR env forces noColor even when flag + cfg say otherwise.
	t.Setenv("NO_COLOR", "1")
	p = NewPrinter(&config.Config{UI: config.UIConfig{UseColors: true}}, false)
	if !p.NoColor() {
		t.Error("expected NoColor()=true when NO_COLOR is set")
	}

	// Cfg false forces noColor.
	t.Setenv("NO_COLOR", "")
	p = NewPrinter(&config.Config{UI: config.UIConfig{UseColors: false}}, false)
	if !p.NoColor() {
		t.Error("expected NoColor()=true when cfg.UI.UseColors=false")
	}
}

func TestPrinter_MessageMethods(t *testing.T) {
	var out, errOut bytes.Buffer
	p := NewPrinter(nil, true) // noColor=true for predictable output
	p.SetOutput(&out)
	p.SetErrorOutput(&errOut)

	p.Success("installed %s", "foo")
	p.Info("caching %s", "bar")
	p.Warning("slow %s", "baz")
	p.Error("failed %s", "qux")
	p.Print("plain %d", 42)
	p.Printf("noln %s", "quux")
	p.Println("one", "two")

	gotOut := out.String()
	gotErr := errOut.String()

	// stdout receives Success, Info, Print, Printf, Println
	for _, want := range []string{"installed foo", "caching bar", "plain 42", "noln quux", "one two"} {
		if !strings.Contains(gotOut, want) {
			t.Errorf("stdout missing %q\n---\n%s", want, gotOut)
		}
	}

	// stderr receives Warning, Error
	for _, want := range []string{"slow baz", "failed qux"} {
		if !strings.Contains(gotErr, want) {
			t.Errorf("stderr missing %q\n---\n%s", want, gotErr)
		}
	}

	// Sanity: Warning/Error should NOT appear on stdout.
	for _, notWant := range []string{"slow baz", "failed qux"} {
		if strings.Contains(gotOut, notWant) {
			t.Errorf("stdout should not contain %q (belongs on stderr)", notWant)
		}
	}
}

func TestStyles_NoColorVariant(t *testing.T) {
	nc := noColorStyles()
	// Icons render without ANSI when noColor is in effect.
	if got := nc.SuccessIcon(); got != "✓" {
		t.Errorf("SuccessIcon() = %q, want bare ✓", got)
	}
	if got := nc.ErrorIcon(); got != "✗" {
		t.Errorf("ErrorIcon() = %q, want bare ✗", got)
	}
	if got := nc.WarningIcon(); got != "⚠" {
		t.Errorf("WarningIcon() = %q, want bare ⚠", got)
	}
	if got := nc.InfoIcon(); got != "ℹ" {
		t.Errorf("InfoIcon() = %q, want bare ℹ", got)
	}
}

func TestStyles_FormatStatus(t *testing.T) {
	s := noColorStyles()
	cases := map[string]string{
		"installed":     "● Installed",
		"update":        "↑ Update Available",
		"not_installed": "○ Not Installed",
		"error":         "✗ Error",
		"something":     "something", // default fallthrough
	}
	for input, want := range cases {
		if got := s.FormatStatus(input); got != want {
			t.Errorf("FormatStatus(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStyles_FormatBadge(t *testing.T) {
	s := noColorStyles()
	// In the no-color path, all variants render the literal text without
	// ANSI, so each should just contain the input string.
	for _, variant := range []string{"success", "warning", "error", "info", "other"} {
		got := s.FormatBadge("HELLO", variant)
		if !strings.Contains(got, "HELLO") {
			t.Errorf("FormatBadge(\"HELLO\", %q) = %q, missing text", variant, got)
		}
	}
}

func TestStyles_FormatAgentAndMethod(t *testing.T) {
	s := noColorStyles()
	if got := s.FormatAgentName("aider"); !strings.Contains(got, "aider") {
		t.Errorf("FormatAgentName missing name: %q", got)
	}
	if got := s.FormatMethod("brew"); !strings.Contains(got, "brew") {
		t.Errorf("FormatMethod missing method: %q", got)
	}
	if got := s.FormatHeader("NAME"); !strings.Contains(got, "NAME") {
		t.Errorf("FormatHeader missing text: %q", got)
	}
}

func TestStyles_FormatVersion(t *testing.T) {
	s := noColorStyles()
	if got := s.FormatVersion("1.2.3", false); !strings.Contains(got, "1.2.3") {
		t.Errorf("FormatVersion(_, false) missing version: %q", got)
	}
	if got := s.FormatVersion("1.2.3", true); !strings.Contains(got, "1.2.3") {
		t.Errorf("FormatVersion(_, true) missing version: %q", got)
	}
}

func TestPrinter_Styles_ReturnsConfiguredPalette(t *testing.T) {
	// With noColor=true, Styles() should return the no-color variant.
	p := NewPrinter(nil, true)
	s := p.Styles()
	if s == nil {
		t.Fatal("Styles() returned nil")
	}
	// No-color icons are the raw chars (covered above); here we just ensure
	// Styles doesn't panic and returns a non-nil renderer.
	if s.renderer == nil {
		t.Error("Styles().renderer is nil")
	}

	// With noColor=false, Styles() should return the colored variant.
	t.Setenv("NO_COLOR", "")
	p = NewPrinter(&config.Config{UI: config.UIConfig{UseColors: true}}, false)
	s = p.Styles()
	if s == nil || s.renderer == nil {
		t.Fatal("Styles() returned nil or unrendered variant")
	}
	// Purple etc. are populated in the default variant; ensure at least one
	// color was assigned.
	if s.Purple == "" {
		t.Error("default Styles.Purple is empty")
	}
}
