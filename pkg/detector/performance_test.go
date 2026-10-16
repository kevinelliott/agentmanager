package detector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/catalog"
)

type inventoryPlatform struct {
	benchPlatform
	bin string
}

func (p *inventoryPlatform) FindExecutable(string) (string, error) { return "", exec.ErrNotFound }
func (p inventoryPlatform) IsExecutableInPath(name string) bool {
	_, err := os.Stat(filepath.Join(p.bin, name))
	return err == nil
}

func inventoryFixture(t testing.TB) (*inventoryPlatform, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX subprocess fixture")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	t.Setenv("PATH", dir)
	t.Setenv("AGENTMGR_TEST_COMMAND_LOG", log)
	for name, output := range map[string]string{
		"npm":  `{"dependencies":{"test-agent":{"version":"1.0.0"}}}`,
		"pip3": `[]`, "pipx": `{"venvs":{}}`, "uv": "test-agent v1.0.0", "brew": `{"formulae":[],"casks":[]}`,
	} {
		script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '%s' >> \"$AGENTMGR_TEST_COMMAND_LOG\"\nprintf '%%s\\n' '%s'\n", name, output)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return &inventoryPlatform{bin: dir}, log
}

func TestTargetedDetectionSkipsUnrelatedInventories(t *testing.T) {
	plat, log := inventoryFixture(t)
	def := catalog.AgentDef{ID: "test", Name: "Test", InstallMethods: map[string]catalog.InstallMethodDef{"npm": {Method: "npm", Package: "test-agent"}}}
	result, err := New(plat).DetectAgent(t.Context(), def)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "npm" || len(result) != 1 {
		t.Fatalf("commands=%q installations=%d", data, len(result))
	}
}

func BenchmarkDetectorInventory(b *testing.B) {
	for _, mode := range []string{"empty", "npm-only", "full-catalog"} {
		b.Run(mode, func(b *testing.B) {
			plat, _ := inventoryFixture(b)
			d := New(plat)
			var defs []catalog.AgentDef
			if mode == "npm-only" {
				defs = []catalog.AgentDef{{ID: "test", Name: "Test", InstallMethods: map[string]catalog.InstallMethodDef{"npm": {Method: "npm", Package: "test-agent"}}}}
			}
			if mode == "full-catalog" {
				var cat catalog.Catalog
				if err := json.Unmarshal(catalog.EmbeddedJSON(), &cat); err != nil {
					b.Fatal(err)
				}
				defs = cat.GetAgents()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := d.DetectAll(context.Background(), defs); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
