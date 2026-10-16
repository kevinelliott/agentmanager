package strategies

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/catalog"
)

func TestVersionProbeDoesNotWaitForDescendantOutputAfterTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX descendant process fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "probe")
	pidFile := filepath.Join(dir, "child-pid")
	t.Setenv("AGENTMGR_TIMEOUT_TEST_PID", pidFile)
	body := "#!/bin/sh\n/bin/sleep 10 &\nprintf '%s\\n' \"$!\" > \"$AGENTMGR_TIMEOUT_TEST_PID\"\nwait\n"
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		data, err := os.ReadFile(pidFile)
		if err != nil {
			return
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return
		}
		child, err := os.FindProcess(pid)
		if err == nil {
			_ = child.Kill()
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	started := time.Now()
	version := NewBinaryStrategy(newMockPlatform()).getVersion(ctx, catalog.AgentDef{Detection: catalog.DetectionDef{VersionCmd: "probe --version"}}, script)
	if !version.IsZero() {
		t.Fatalf("timed out probe returned a version: %v", version)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("inherited output pipe defeated cancellation: %v", elapsed)
	}
}
