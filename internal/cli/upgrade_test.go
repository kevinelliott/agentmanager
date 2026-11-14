package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseVersion(t *testing.T) {
	cases := map[string]string{
		"v1.5.0":                  "1.5.0",
		"1.5.0":                   "1.5.0",
		"1.5.0-34-gb10cffc":       "1.5.0",
		"1.5.0-34-gb10cffc-dirty": "1.5.0",
		"1.6.0-rc.1":              "1.6.0-rc.1",
		"1.6.0-rc.1-2-gabc1234":   "1.6.0-rc.1",
	}
	for in, want := range cases {
		if got := releaseVersion(in); got != want {
			t.Errorf("releaseVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetAssetNameMatchesReleaseArchives(t *testing.T) {
	got := getAssetName("1.6.0")
	wantPrefix := "agentmanager_1.6.0_" + runtime.GOOS + "_" + runtime.GOARCH
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("getAssetName = %q, want prefix %q", got, wantPrefix)
	}
	if !strings.HasSuffix(got, ".tar.gz") && !strings.HasSuffix(got, ".zip") {
		t.Errorf("getAssetName = %q, want an archive", got)
	}
}

func TestManagedInstallHint(t *testing.T) {
	if managedInstallHint("/opt/homebrew/Cellar/agentmanager/1.5.0/bin/agentmgr") == "" {
		t.Error("Homebrew install should be redirected to brew")
	}
	if managedInstallHint("/Applications/AgentManager.app/Contents/MacOS/agentmgr") == "" {
		t.Error("app bundle install should be redirected to the app download")
	}
	if hint := managedInstallHint("/usr/local/bin/agentmgr"); hint != "" {
		t.Errorf("plain install got hint %q", hint)
	}
}

// writeTarGz writes an archive holding the given files and returns its path.
func writeTarGz(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "release.tar.gz")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVerifyChecksumAndExtract(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release archives are zip on Windows")
	}
	dir := t.TempDir()
	const asset = "agentmanager_1.6.0_test.tar.gz"
	archive := writeTarGz(t, dir, map[string]string{
		"README.md": "docs",
		"agentmgr":  "new binary",
	})

	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	sums := filepath.Join(dir, "checksums.txt")
	good := "deadbeef  other.tar.gz\n" + hex.EncodeToString(sum[:]) + "  " + asset + "\n"
	if err := os.WriteFile(sums, []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(archive, sums, asset); err != nil {
		t.Fatalf("good checksum rejected: %v", err)
	}
	if err := verifyChecksum(archive, sums, "missing.tar.gz"); err == nil {
		t.Error("unlisted asset accepted")
	}

	bad := strings.Repeat("0", 64) + "  " + asset + "\n"
	if err := os.WriteFile(sums, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(archive, sums, asset); err == nil {
		t.Error("wrong checksum accepted")
	}

	bin, err := extractBinary(archive, asset)
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	defer os.Remove(bin)
	got, err := os.ReadFile(bin)
	if err != nil || string(got) != "new binary" {
		t.Errorf("extracted %q, %v", got, err)
	}

	empty := writeTarGz(t, t.TempDir(), map[string]string{"README.md": "docs"})
	if _, err := extractBinary(empty, asset); err == nil {
		t.Error("archive without the binary accepted")
	}
}
