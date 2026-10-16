package strategies

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
)

func embeddedAgent(t *testing.T, id string) catalog.AgentDef {
	t.Helper()
	var c catalog.Catalog
	if err := json.Unmarshal(catalog.EmbeddedJSON(), &c); err != nil {
		t.Fatal(err)
	}
	a, ok := c.Agents[id]
	if !ok {
		t.Fatalf("missing catalog entry %q", id)
	}
	return a
}

func TestCatalogUnrealHomebrewCaskDetection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Homebrew fixture")
	}
	dir := t.TempDir()
	fixture := `#!/bin/sh
case "$*" in
  *--cask*) printf '%s\n' '{"casks":[{"token":"unreal-agent","installed":"0.3.1","version":"0.3.1"}]}' ;;
  *) printf '%s\n' '{"formulae":[]}' ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "brew"), []byte(fixture), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	plat := newMockPlatform()
	plat.executablePaths["unreal-agent"] = "/opt/homebrew/bin/unreal-agent"
	got, err := NewBrewStrategy(plat).Detect(context.Background(), []catalog.AgentDef{embeddedAgent(t, "unreal-agent")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Method != agent.MethodBrew || got[0].Metadata["package_type"] != "cask" {
		t.Fatalf("cask must stay on Homebrew update/remove routing: %+v", got)
	}
}

func TestCatalogLeviathanCargoDetection(t *testing.T) {
	for _, path := range []string{"/Users/test/.cargo/bin/leviathan", `C:\Users\test\.cargo\bin\leviathan.exe`} {
		t.Run(path, func(t *testing.T) {
			def := embeddedAgent(t, "leviathan")
			def.Detection.VersionCmd = "" // Exercise discovery without launching a real third-party binary.
			plat := newMockPlatform()
			plat.executablePaths["leviathan"] = path
			got, err := NewBinaryStrategy(plat).Detect(context.Background(), []catalog.AgentDef{def})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Method != agent.InstallMethod("cargo") || got[0].ExecutablePath != path {
				t.Fatalf("Cargo installation must remain discoverable: %+v", got)
			}
			method, ok := def.GetInstallMethod(string(got[0].Method))
			if !ok || method.Method != "cargo" || method.UninstallCmd != "cargo uninstall leviathan-index" {
				t.Fatalf("detected method must route to Cargo update/remove commands: %+v", method)
			}
		})
	}
}

func TestCatalogHomebrewFormulaDetection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Homebrew fixture")
	}
	dir := t.TempDir()
	// Homebrew v2 serializes installed kegs as objects, not version strings.
	fixture := `#!/bin/sh
case "$*" in
  *--cask*) printf '%s\n' '{"casks":[]}' ;;
  *) printf '%s\n' '{"formulae":[{"name":"docker-agent","full_name":"docker-agent","installed":[{"version":"1.150.0","used_options":[]}],"versions":{"stable":"1.150.0"}}]}' ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "brew"), []byte(fixture), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := NewBrewStrategy(newMockPlatform()).Detect(context.Background(), []catalog.AgentDef{embeddedAgent(t, "docker-agent")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Method != agent.MethodBrew || got[0].InstalledVersion.String() != "1.150.0" {
		t.Fatalf("installed Homebrew formula must retain its version and update method: %+v", got)
	}
}

func TestCargoPathRequiresDeclaredCargoMethod(t *testing.T) {
	def := catalog.AgentDef{
		ID: "native-only", InstallMethods: map[string]catalog.InstallMethodDef{"native": {Method: "native"}},
		Detection: catalog.DetectionDef{Executables: []string{"native-only"}},
	}
	plat := newMockPlatform()
	plat.executablePaths["native-only"] = "/Users/test/.cargo/bin/native-only"
	got, err := NewBinaryStrategy(plat).Detect(context.Background(), []catalog.AgentDef{def})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("unrelated package-manager binaries must stay excluded: %+v", got)
	}
}

func TestCargoPathOverridesNativeInstallMethod(t *testing.T) {
	def := embeddedAgent(t, "leviathan")
	def.Detection.VersionCmd = ""
	def.InstallMethods["native"] = catalog.InstallMethodDef{Method: "native"}
	plat := newMockPlatform()
	plat.executablePaths["leviathan"] = "/Users/test/.cargo/bin/leviathan"
	got, err := NewBinaryStrategy(plat).Detect(context.Background(), []catalog.AgentDef{def})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Method != agent.InstallMethod("cargo") {
		t.Fatalf("a Cargo path must route to Cargo even with a native installer available: %+v", got)
	}
}

func TestHomebrewFormulaAndCaskNamesDoNotCrossMatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Homebrew fixture")
	}
	dir := t.TempDir()
	// A formula and a cask may have the same name but different update commands.
	fixture := `#!/bin/sh
case "$*" in
  *--cask*) printf '%s\n' '{"casks":[{"token":"moltis","installed":"1.0.0"}]}' ;;
  *) printf '%s\n' '{"formulae":[{"name":"unreal-agent","full_name":"unreallabsai/tap/unreal-agent","installed":[{"version":"0.3.1"}]}]}' ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "brew"), []byte(fixture), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := NewBrewStrategy(newMockPlatform()).Detect(context.Background(), []catalog.AgentDef{
		embeddedAgent(t, "moltis"), embeddedAgent(t, "unreal-agent"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("formula and cask identities must stay separate: %+v", got)
	}
}
