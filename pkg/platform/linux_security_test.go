//go:build linux

package platform

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func linuxSystemctlFixture(t *testing.T, script string) (*linuxPlatform, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AGENTMGR_TEST_CALLS", logPath)
	t.Setenv("PATH", dir)
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$AGENTMGR_TEST_CALLS\"\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	return &linuxPlatform{}, logPath
}

func TestEnableAutoStartStopsWhenReloadFails(t *testing.T) {
	p, logPath := linuxSystemctlFixture(t, "exit 1\n")
	if err := p.EnableAutoStart(context.Background()); err == nil {
		t.Fatal("expected daemon-reload failure")
	}
	data, err := os.ReadFile(logPath)
	if err != nil || string(data) != "--user daemon-reload\n" {
		t.Fatalf("unexpected systemctl calls: %q %v", data, err)
	}
	info, err := os.Stat(filepath.Join(p.getSystemdUserDir(), "agentmgr-helper.service"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("service file must be private: %v %v", info, err)
	}
}

func TestDisableAutoStartAttemptsBothBackendsAndReturnsFailure(t *testing.T) {
	p, logPath := linuxSystemctlFixture(t, "case \"$*\" in *stop*) exit 1;; esac\n")
	servicePath := filepath.Join(p.getSystemdUserDir(), "agentmgr-helper.service")
	desktopPath := filepath.Join(p.getXDGAutostartDir(), "agentmgr-helper.desktop")
	for _, path := range []string{servicePath, desktopPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.DisableAutoStart(context.Background()); err == nil {
		t.Fatal("expected stop failure to be returned")
	}
	data, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(data), "--user disable agentmgr-helper.service") || !strings.Contains(string(data), "--user daemon-reload") {
		t.Fatalf("cleanup did not continue after stop failure: %q %v", data, err)
	}
	for _, path := range []string{servicePath, desktopPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("autostart file not removed: %s %v", path, err)
		}
	}
	if err := p.DisableAutoStart(context.Background()); err != nil {
		t.Fatalf("disabling absent autostart should be idempotent: %v", err)
	}
}

func TestNotificationPassesLiteralArguments(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "arguments")
	t.Setenv("PATH", dir)
	t.Setenv("AGENTMGR_TEST_ARGS", output)
	if err := os.WriteFile(filepath.Join(dir, "notify-send"), []byte("#!/bin/sh\nprintf '%s\\000' \"$@\" > \"$AGENTMGR_TEST_ARGS\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	title, message := "--untrusted-title", "literal 'quote' $(exit 1); & text"
	if err := (&linuxPlatform{}).ShowNotification(title, message); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	want := "--\x00" + title + "\x00" + message + "\x00"
	if err != nil || string(data) != want {
		t.Fatalf("notification arguments = %q %v, want %q", data, err, want)
	}
}

func TestDisableAutoStartRemovesUnitWithoutSystemctl(t *testing.T) {
	p, _ := linuxSystemctlFixture(t, "exit 0\n")
	t.Setenv("PATH", t.TempDir())
	servicePath := filepath.Join(p.getSystemdUserDir(), "agentmgr-helper.service")
	enablement := filepath.Join(p.getSystemdUserDir(), "default.target.wants", "agentmgr-helper.service")
	if err := os.MkdirAll(filepath.Dir(enablement), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(servicePath, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(servicePath, enablement); err != nil {
		t.Fatal(err)
	}
	if err := p.DisableAutoStart(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{servicePath, enablement} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("autostart remains without systemctl: %s %v", path, err)
		}
	}
	// A dangling enablement left by older versions must also be removed.
	if err := os.Symlink(servicePath, enablement); err != nil {
		t.Fatal(err)
	}
	if err := p.DisableAutoStart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(enablement); !os.IsNotExist(err) {
		t.Fatalf("dangling enablement was not removed: %v", err)
	}
}
