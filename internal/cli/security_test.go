package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReplaceExecutablePreservesModeAndAdjacentFiles(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "download"), filepath.Join(dir, "agent mgr")
	for name, content := range map[string]string{src: "new executable", dst: "old executable", dst + ".old": "unrelated backup", dst + ".new": "unrelated staging"} {
		if err := os.WriteFile(name, []byte(content), 0750); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutable(src, dst); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{dst: "new executable", dst + ".old": "unrelated backup", dst + ".new": "unrelated staging"} {
		data, err := os.ReadFile(name)
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, %v; want %q", name, data, err, want)
		}
	}
	after, err := os.Stat(dst)
	if err != nil || after.Mode().Perm() != before.Mode().Perm() {
		t.Fatalf("executable permissions changed: %v %v", after, err)
	}
	staging, err := filepath.Glob(filepath.Join(dir, ".agentmgr-upgrade-*"))
	if err != nil || len(staging) != 0 {
		t.Fatalf("upgrade staging left behind: %v %v", staging, err)
	}
}

func TestReplaceExecutableDoesNotFollowStagingSymlink(t *testing.T) {
	dir := t.TempDir()
	src, dst, victim := filepath.Join(dir, "download"), filepath.Join(dir, "agentmgr"), filepath.Join(dir, "victim")
	for _, path := range []string{src, dst, victim} {
		if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(victim, dst+".new"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := replaceExecutable(src, dst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(victim)
	if err != nil || string(data) != "keep" {
		t.Fatalf("staging symlink target changed: %q %v", data, err)
	}
	if err := os.Remove(dst); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, dst); err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutable(src, dst); err == nil {
		t.Fatal("expected refusal to replace a symlink destination")
	}
}

func TestDownloadFileRemovesPartialDownload(t *testing.T) {
	dir := t.TempDir()
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, dir)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("incomplete"))
	}))
	defer server.Close()
	if _, err := downloadFile(context.Background(), server.URL); err == nil {
		t.Fatal("expected truncated response error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial download remains: %v %v", entries, err)
	}
}

type failingVersionWriter struct{ err error }

func (w failingVersionWriter) Write([]byte) (int, error) { return 0, w.err }

func TestVersionJSONReturnsOutputError(t *testing.T) {
	want := errors.New("output unavailable")
	cmd := NewVersionCommand("test", "test", "test")
	cmd.SetOut(failingVersionWriter{want})
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); !errors.Is(err, want) {
		t.Fatalf("version output error = %v, want %v", err, want)
	}
}

func TestSetPluginEnabledRejectsPathTraversal(t *testing.T) {
	for _, name := range []string{"../outside", "a/../../outside", `a\..\outside`, "/tmp/plugin", ".", ""} {
		t.Run(name, func(t *testing.T) {
			if err := setPluginEnabled(name, true); err == nil {
				t.Fatalf("accepted unsafe plugin name %q on %s", name, runtime.GOOS)
			}
		})
	}
}
