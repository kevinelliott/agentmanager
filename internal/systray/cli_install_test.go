package systray

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyCLIForWindows(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "bundle")
	target := filepath.Join(dir, "install", "agentmgr.exe")
	if err := os.WriteFile(source, []byte("bundled executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := copyCLIForWindows(source, target); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "bundled executable" {
		t.Fatalf("installed binary = %q %v", data, err)
	}
	if err := copyCLIForWindows(source, filepath.Join(dir, "unexpected")); err == nil {
		t.Fatal("accepted an unexpected installation filename")
	}
}

func TestCopyCLIForWindowsRejectsEscapingSymlink(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "bundle")
	victim := filepath.Join(dir, "victim")
	for _, path := range []string{source, victim} {
		if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	installDir := filepath.Join(dir, "install")
	if err := os.Mkdir(installDir, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(installDir, "agentmgr.exe")
	if err := os.Symlink(victim, target); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := copyCLIForWindows(source, target); err == nil {
		t.Fatal("expected refusal to write outside installation root")
	}
	data, err := os.ReadFile(victim)
	if err != nil || string(data) != "keep" {
		t.Fatalf("symlink target changed: %q %v", data, err)
	}
}
