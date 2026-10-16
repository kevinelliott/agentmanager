package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWritePrivateFileReplacesPermissiveFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivateFile(path, []byte("secret")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "secret" {
		t.Fatalf("read config: data=%q error=%v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("configuration permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestWritePrivateFileDoesNotFollowSymlink(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "other")
	if err := os.WriteFile(other, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.Symlink(other, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := WritePrivateFile(path, []byte("config")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(other)
	if err != nil || string(data) != "keep" {
		t.Fatalf("symlink target changed: data=%q error=%v", data, err)
	}
}

func TestWritePrivateFileFailurePreservesDestinationAndCleansStaging(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing-directory")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivateFile(path, []byte("config")); err == nil {
		t.Fatal("expected rename failure")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("destination or staging cleanup incorrect: %v %v", entries, err)
	}
}

func TestLoaderSaveFailureDoesNotTruncateConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.unsupported")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	l := NewLoader()
	l.filePath = path
	if err := l.Save(Default()); err == nil {
		t.Fatal("expected unsupported format error")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatalf("failed save damaged existing configuration: %q %v", data, err)
	}
}
