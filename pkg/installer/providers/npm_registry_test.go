package providers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type registryRoundTrip func(*http.Request) (*http.Response, error)

func (f registryRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPublicNPMConfigPreservesCustomSettings(t *testing.T) {
	for _, test := range []struct {
		name, data string
		public     bool
	}{
		{"default", `{"registry":"https://registry.npmjs.org/","tag":"latest","strict-ssl":true,"auth-type":"web","_auth":null,"ca":null,"omit":[]}`, true},
		{"registry", `{"registry":"https://private.example/"}`, false},
		{"scope", `{"registry":"https://registry.npmjs.org/","@company:registry":"https://private.example/"}`, false},
		{"token", `{"registry":"https://registry.npmjs.org/","//registry.npmjs.org/:_authToken":"(protected)"}`, false},
		{"tls", `{"registry":"https://registry.npmjs.org/","strict-ssl":false}`, false},
		{"proxy", `{"registry":"https://registry.npmjs.org/","https-proxy":"http://proxy.example"}`, false},
		{"ca", `{"registry":"https://registry.npmjs.org/","ca":["custom"]}`, false},
		{"offline", `{"registry":"https://registry.npmjs.org/","offline":true}`, false},
		{"tag", `{"registry":"https://registry.npmjs.org/","tag":"next"}`, false},
		{"invalid", `not-json`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := publicNPMConfig([]byte(test.data)); got != test.public {
				t.Fatalf("public=%v, want %v", got, test.public)
			}
		})
	}
}

func TestNPMRegistrySharesConfigAndUsesScopedMetadataURL(t *testing.T) {
	t.Setenv("NODE_EXTRA_CA_CERTS", "")
	configCalls, requests := 0, 0
	registry := &npmRegistry{
		config: func(context.Context) ([]byte, error) {
			configCalls++
			return []byte(`{"registry":"https://registry.npmjs.org/"}`), nil
		},
		client: &http.Client{Transport: registryRoundTrip(func(r *http.Request) (*http.Response, error) {
			requests++
			if r.URL.Host != "registry.npmjs.org" || !strings.HasSuffix(r.URL.EscapedPath(), "scope%2Fpackage/latest") {
				t.Errorf("unexpected metadata URL: %s", r.URL)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"version":"2.3.4"}`)), Header: make(http.Header)}, nil
		})},
	}
	for range 2 {
		version, ok := registry.latest(t.Context(), "@scope/package")
		if !ok || version.String() != "2.3.4" {
			t.Fatalf("version=%v, ok=%v", version, ok)
		}
	}
	if configCalls != 1 || requests != 2 {
		t.Fatalf("config=%d requests=%d", configCalls, requests)
	}
}

func TestCustomNPMRegistryNeverUsesPublicEndpoint(t *testing.T) {
	registry := &npmRegistry{
		config: func(context.Context) ([]byte, error) { return []byte(`{"registry":"https://private.example/"}`), nil },
		client: &http.Client{Transport: registryRoundTrip(func(*http.Request) (*http.Response, error) {
			t.Fatal("custom registry contacted public npm")
			return nil, nil
		})},
	}
	if _, ok := registry.latest(t.Context(), "@private/package"); ok {
		t.Fatal("custom registry should use npm fallback")
	}
}

func TestNPMRegistryRetriesFailedConfiguration(t *testing.T) {
	for _, failure := range []error{errors.New("temporary npm failure"), context.Canceled} {
		t.Run(failure.Error(), func(t *testing.T) {
			t.Setenv("NODE_EXTRA_CA_CERTS", "")
			configCalls, requests := 0, 0
			registry := &npmRegistry{
				config: func(context.Context) ([]byte, error) {
					configCalls++
					if configCalls == 1 {
						return nil, failure
					}
					return []byte(`{"registry":"https://registry.npmjs.org/"}`), nil
				},
				client: &http.Client{Transport: registryRoundTrip(func(*http.Request) (*http.Response, error) {
					requests++
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"version":"2.3.4"}`)), Header: make(http.Header)}, nil
				})},
			}
			if _, ok := registry.latest(t.Context(), "example"); ok || requests != 0 {
				t.Fatal("failed configuration must use npm fallback")
			}
			version, ok := registry.latest(t.Context(), "example")
			if !ok || version.String() != "2.3.4" || configCalls != 2 || requests != 1 {
				t.Fatalf("configuration failure blocked recovery: version=%v ok=%v config=%d requests=%d", version, ok, configCalls, requests)
			}
		})
	}
}

func TestNPMRegistryReloadsChangedConfiguration(t *testing.T) {
	t.Setenv("NODE_EXTRA_CA_CERTS", "")
	configCalls, requests := 0, 0
	registry := &npmRegistry{
		config: func(context.Context) ([]byte, error) {
			configCalls++
			if configCalls == 1 {
				return []byte(`{"registry":"https://registry.npmjs.org/"}`), nil
			}
			return []byte(`{"registry":"https://private.example/"}`), nil
		},
		client: &http.Client{Transport: registryRoundTrip(func(*http.Request) (*http.Response, error) {
			requests++
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"version":"1.0.0"}`)), Header: make(http.Header)}, nil
		})},
	}
	if _, ok := registry.latest(t.Context(), "example"); !ok {
		t.Fatal("expected public metadata")
	}
	t.Setenv("NPM_CONFIG_REGISTRY", "https://private.example/")
	if _, ok := registry.latest(t.Context(), "example"); ok {
		t.Fatal("configuration change ignored")
	}
	if configCalls != 2 || requests != 1 {
		t.Fatalf("config=%d requests=%d", configCalls, requests)
	}
}

func TestNPMFingerprintTracksConfigurationFileContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "npmrc")
	t.Setenv("NPM_CONFIG_USERCONFIG", path)
	if err := os.WriteFile(path, []byte("tag=latest\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := NPMConfigFingerprint()
	if err := os.WriteFile(path, []byte("tag=next\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if after := NPMConfigFingerprint(); before == after {
		t.Fatal("changed npm configuration reused a registry cache namespace")
	}
	// Non-regular configured paths are never opened by the fingerprint reader.
	t.Setenv("NPM_CONFIG_USERCONFIG", filepath.Dir(path))
	if NPMConfigFingerprint() == "" {
		t.Fatal("invalid fingerprint")
	}
}
