package providers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

// Public metadata requests share connections. npm remains authoritative when
// registry, authentication, TLS, proxy, offline or tag settings need its client.
var npmRegistryHTTPClient = newNPMRegistryClient()

func newNPMRegistryClient() *http.Client {
	client := &http.Client{Timeout: 5 * time.Second}
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		transport := base.Clone()
		transport.MaxIdleConnsPerHost = 16
		client.Transport = transport
	}
	return client
}

type npmRegistry struct {
	mu          sync.Mutex
	fingerprint string
	checkedAt   time.Time
	public      bool
	client      *http.Client
	config      func(context.Context) ([]byte, error)
}

func newNPMRegistry() *npmRegistry {
	return &npmRegistry{client: npmRegistryHTTPClient, config: func(ctx context.Context) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "npm", "config", "list", "--json")
		cmd.WaitDelay = time.Second
		return cmd.Output()
	}}
}

func (r *npmRegistry) latest(ctx context.Context, name string) (agent.Version, bool) {
	identity := NPMConfigFingerprint()
	r.mu.Lock()
	if r.fingerprint != identity || time.Since(r.checkedAt) >= registryLatestVersionTTL {
		data, err := r.config(ctx)
		r.public = err == nil && publicNPMConfig(data)
		r.fingerprint = identity
		r.checkedAt = time.Now()
	}
	public := r.public && os.Getenv("NODE_EXTRA_CA_CERTS") == ""
	r.mu.Unlock()

	if !public {
		return agent.Version{}, false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://registry.npmjs.org/"+url.PathEscape(name)+"/latest", nil)
	if err != nil {
		return agent.Version{}, false
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AgentManager")
	resp, err := r.client.Do(req)
	if err != nil {
		return agent.Version{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return agent.Version{}, false
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil || payload.Version == "" {
		return agent.Version{}, false
	}
	version, err := agent.ParseVersion(payload.Version)
	return version, err == nil
}

func publicNPMConfig(data []byte) bool {
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false
	}
	if registry, ok := cfg["registry"].(string); !ok || strings.TrimRight(registry, "/") != "https://registry.npmjs.org" {
		return false
	}
	for key, value := range cfg {
		if value == nil || value == false || value == "" {
			continue
		}
		lower := strings.ToLower(key)
		if strings.HasSuffix(lower, ":registry") && strings.TrimRight(fmt.Sprint(value), "/") != "https://registry.npmjs.org" {
			return false
		}
		if lower != "auth-type" && (strings.Contains(lower, "auth") || strings.Contains(lower, "password") || strings.Contains(lower, "token") || lower == "username") {
			return false
		}
		switch lower {
		case "proxy", "https-proxy", "cafile", "ca", "cert", "key", "offline", "prefer-offline", "prefer-online", "before":
			return false
		case "tag":
			if value != "latest" {
				return false
			}
		}
	}
	if strict, ok := cfg["strict-ssl"].(bool); ok && !strict {
		return false
	}
	return true
}

// NPMConfigFingerprint scopes registry caches to the working directory,
// npm configuration files, and relevant environment without storing secrets.
func NPMConfigFingerprint() string {
	hash := sha256.New()
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	_, _ = fmt.Fprintln(hash, cwd)
	env := os.Environ()
	sort.Strings(env)
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "npm_config_") || lower == "node_extra_ca_certs" || strings.Contains(lower, "proxy") {
			_, _ = fmt.Fprintln(hash, entry)
		}
	}
	paths := []string{"/etc/npmrc", "/usr/etc/npmrc", "/usr/local/etc/npmrc"}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".npmrc"))
	}
	for _, key := range []string{"NPM_CONFIG_USERCONFIG", "npm_config_userconfig", "NPM_CONFIG_GLOBALCONFIG", "npm_config_globalconfig"} {
		if path := os.Getenv(key); path != "" {
			paths = append(paths, path)
		}
	}
	for dir := cwd; dir != ""; dir = filepath.Dir(dir) {
		paths = append(paths, filepath.Join(dir, ".npmrc"))
		if filepath.Dir(dir) == dir {
			break
		}
	}
	for _, path := range paths {
		_, _ = fmt.Fprintln(hash, path)
		// Configuration paths are explicitly chosen by npm's user/environment.
		// Hash bounded regular-file contents; never return or log their bytes.
		info, err := os.Stat(path) // #nosec G703 -- Caller-selected npm configuration metadata is hashed locally, never returned.
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		_, _ = fmt.Fprintln(hash, info.Size(), info.ModTime().UnixNano())
		file, err := os.Open(path) // #nosec G304 G703 -- Reads caller-selected npm configuration only; contents are hashed locally, never exposed.
		if err == nil {
			if _, err := io.Copy(hash, io.LimitReader(file, 64<<10)); err != nil {
				_, _ = fmt.Fprint(hash, "unreadable")
			}
			_ = file.Close()
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
