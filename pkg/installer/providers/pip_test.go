package providers

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/catalog"
)

// FastMCP's metapackage has no entry point of its own. Its dependency's CLI
// must be exposed without losing the main package identity used for updates.
func TestFastMCPUVLifecycleCommands(t *testing.T) {
	var c catalog.Catalog
	if err := json.Unmarshal(catalog.EmbeddedJSON(), &c); err != nil {
		t.Fatal(err)
	}
	method, ok := c.Agents["fastmcp"].InstallMethods["uv"]
	if !ok || method.Package != "fastmcp" {
		t.Fatal("FastMCP must retain its uv package identity")
	}
	plat := newMockPlatform()
	plat.executables["uv"] = "/usr/bin/uv"
	p := NewPipProvider(plat)
	for _, force := range []bool{false, true} {
		manager, args, pkg, err := p.buildInstallCommand(method, force)
		want := []string{"tool", "install"}
		if force {
			want = append(want, "--force")
		}
		want = append(want, "--with-executables-from", "fastmcp-slim", "fastmcp")
		if err != nil || manager != "uv" || pkg != "fastmcp" || !slices.Equal(args, want) {
			t.Fatalf("install(force=%v) = %q %v, package %q, err %v; want uv %v", force, manager, args, pkg, err, want)
		}
	}
	for _, operation := range []string{"upgrade", "uninstall"} {
		build := p.buildUpdateCommand
		if operation == "uninstall" {
			build = p.buildUninstallCommand
		}
		manager, args, pkg, err := build(method)
		if err != nil || manager != "uv" || pkg != "fastmcp" || !slices.Equal(args, []string{"tool", operation, "fastmcp"}) {
			t.Fatalf("%s lost the uv package identity: %q %v, package %q, err %v", operation, manager, args, pkg, err)
		}
	}
}
