package providers

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

func TestPackageProvidersCompleteRuntimeSetup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("package-manager fixtures use POSIX shell scripts")
	}
	for _, manager := range []string{"npm", "brew"} {
		for _, operation := range []string{"install", "update"} {
			for _, failure := range []string{"none", "package", "setup"} {
				t.Run(manager+"/"+operation+"/"+failure, func(t *testing.T) {
					dir := t.TempDir()
					logPath := filepath.Join(dir, "operations")
					t.Setenv("AGENTMGR_TEST_OPERATIONS", logPath)
					t.Setenv("AGENTMGR_TEST_FAILURE", failure)
					t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
					script := `#!/bin/sh
case "$1" in
install|update|upgrade)
  printf 'package\n' >> "$AGENTMGR_TEST_OPERATIONS"
  if [ "$AGENTMGR_TEST_FAILURE" = package ]; then exit 1; fi
  ;;
list) printf 'agent-browser@1.2.3\n' ;;
esac
`
					if err := os.WriteFile(filepath.Join(dir, manager), []byte(script), 0700); err != nil {
						t.Fatal(err)
					}
					plat := newMockPlatform()
					plat.id = platform.Linux
					method := catalog.InstallMethodDef{Method: manager, Package: "agent-browser", Metadata: map[string]string{
						"post_install_cmd": "exit 99",
						"post_install_cmd_linux": `printf 'setup\n' >> "$AGENTMGR_TEST_OPERATIONS"
if [ "$AGENTMGR_TEST_FAILURE" = setup ]; then exit 2; fi
printf 'runtime ready\n'`,
					}}
					def := catalog.AgentDef{ID: "agent-browser", Name: "Browser"}
					var provider interface {
						Install(context.Context, catalog.AgentDef, catalog.InstallMethodDef, bool) (*Result, error)
						Update(context.Context, *agent.Installation, catalog.AgentDef, catalog.InstallMethodDef) (*Result, error)
					}
					if manager == "npm" {
						provider = NewNPMProvider(plat)
					} else {
						provider = NewBrewProvider(plat)
					}
					var result *Result
					var err error
					if operation == "install" {
						result, err = provider.Install(t.Context(), def, method, false)
					} else {
						result, err = provider.Update(t.Context(), &agent.Installation{}, def, method)
					}
					if (err != nil) != (failure != "none") {
						t.Fatalf("failure=%s, got %v", failure, err)
					}
					if failure == "setup" && !strings.Contains(err.Error(), "post-install") {
						t.Fatalf("setup failure not explained: %v", err)
					}
					if failure == "none" && !strings.Contains(result.Output, "runtime ready") {
						t.Fatal("runtime setup output was lost")
					}
					data, err := os.ReadFile(logPath)
					if err != nil {
						t.Fatal(err)
					}
					want := "package\nsetup\n"
					if failure == "package" {
						want = "package\n"
					}
					if string(data) != want {
						t.Fatalf("setup order: got %q, want %q", data, want)
					}
				})
			}
		}
	}
}

func TestPostInstallRespectsCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock platform uses a POSIX shell")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	method := catalog.InstallMethodDef{Metadata: map[string]string{"post_install_cmd": "exit 0"}}
	if _, err := runPostInstall(ctx, newMockPlatform(), method); err == nil {
		t.Fatal("canceled runtime setup reported success")
	}
}
