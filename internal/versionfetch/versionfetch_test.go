package versionfetch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
)

// fakeFetcher is a configurable stand-in for installer.Manager used by tests.
type fakeFetcher struct {
	mu       sync.Mutex
	inFlight int32
	peak     int32
	delay    time.Duration

	// per-package overrides
	versions map[string]agent.Version
	errs     map[string]error

	// calls keyed by package name for assertions
	calls map[string]int
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{
		versions: map[string]agent.Version{},
		errs:     map[string]error{},
		calls:    map[string]int{},
	}
}

func (f *fakeFetcher) GetLatestVersion(ctx context.Context, method catalog.InstallMethodDef) (agent.Version, error) {
	cur := atomic.AddInt32(&f.inFlight, 1)
	defer atomic.AddInt32(&f.inFlight, -1)

	for {
		peak := atomic.LoadInt32(&f.peak)
		if cur <= peak || atomic.CompareAndSwapInt32(&f.peak, peak, cur) {
			break
		}
	}

	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return agent.Version{}, ctx.Err()
		}
	}

	f.mu.Lock()
	f.calls[method.Package]++
	err := f.errs[method.Package]
	ver, ok := f.versions[method.Package]
	f.mu.Unlock()

	if err != nil {
		return agent.Version{}, err
	}
	if !ok {
		ver = agent.Version{Major: 1, Minor: 0, Patch: 0}
	}
	return ver, nil
}

func mkInstall(id, method string) *agent.Installation {
	return &agent.Installation{
		AgentID:          id,
		AgentName:        id,
		Method:           agent.InstallMethod(method),
		InstalledVersion: agent.Version{Major: 0, Minor: 9, Patch: 0},
	}
}

func mkAgentDefs(ids ...string) map[string]catalog.AgentDef {
	m := map[string]catalog.AgentDef{}
	for _, id := range ids {
		m[id] = catalog.AgentDef{
			ID:   id,
			Name: id,
			InstallMethods: map[string]catalog.InstallMethodDef{
				"npm": {Method: "npm", Package: id},
			},
		}
	}
	return m
}

func TestCheckLatestVersions_FillsVersionsAndPreservesOrder(t *testing.T) {
	f := newFakeFetcher()
	f.versions["a"] = agent.Version{Major: 2, Minor: 0, Patch: 0}
	f.versions["b"] = agent.Version{Major: 3, Minor: 1, Patch: 0}
	f.versions["c"] = agent.Version{Major: 4, Minor: 2, Patch: 1}

	insts := []*agent.Installation{
		mkInstall("a", "npm"),
		mkInstall("b", "npm"),
		mkInstall("c", "npm"),
	}
	defs := mkAgentDefs("a", "b", "c")

	errs := CheckLatestVersions(context.Background(), f, insts, defs, 4)
	for i, e := range errs {
		if e != nil {
			t.Fatalf("errs[%d] = %v, want nil", i, e)
		}
	}

	want := map[string]agent.Version{
		"a": {Major: 2, Minor: 0, Patch: 0},
		"b": {Major: 3, Minor: 1, Patch: 0},
		"c": {Major: 4, Minor: 2, Patch: 1},
	}
	for _, inst := range insts {
		if inst.LatestVersion == nil {
			t.Fatalf("%s: LatestVersion not set", inst.AgentID)
		}
		if *inst.LatestVersion != want[inst.AgentID] {
			t.Errorf("%s: LatestVersion = %v, want %v", inst.AgentID, *inst.LatestVersion, want[inst.AgentID])
		}
	}
}

func TestCheckLatestVersions_CapturesPerInstallationErrors(t *testing.T) {
	f := newFakeFetcher()
	boom := errors.New("boom")
	f.errs["b"] = boom

	insts := []*agent.Installation{
		mkInstall("a", "npm"),
		mkInstall("b", "npm"),
		mkInstall("c", "npm"),
	}
	defs := mkAgentDefs("a", "b", "c")

	errs := CheckLatestVersions(context.Background(), f, insts, defs, 4)
	if errs[0] != nil {
		t.Errorf("errs[0] = %v, want nil", errs[0])
	}
	if errs[1] == nil || !errors.Is(errs[1], boom) {
		t.Errorf("errs[1] = %v, want error wrapping boom", errs[1])
	}
	if errs[2] != nil {
		t.Errorf("errs[2] = %v, want nil", errs[2])
	}

	// Sibling successes should still have their versions filled.
	if insts[0].LatestVersion == nil {
		t.Errorf("insts[0].LatestVersion not filled despite sibling error")
	}
	if insts[2].LatestVersion == nil {
		t.Errorf("insts[2].LatestVersion not filled despite sibling error")
	}
	if insts[1].LatestVersion != nil {
		t.Errorf("insts[1].LatestVersion should be nil on error, got %v", insts[1].LatestVersion)
	}

	non := NonNilErrors(errs)
	if len(non) != 1 {
		t.Errorf("NonNilErrors returned %d, want 1", len(non))
	}
}

func TestCheckLatestVersions_SkipsUnknownAgentOrMethod(t *testing.T) {
	f := newFakeFetcher()

	insts := []*agent.Installation{
		mkInstall("a", "npm"),
		mkInstall("ghost", "npm"),  // not in defs -> skipped
		mkInstall("c", "missing"),  // method not in def -> skipped
	}
	defs := mkAgentDefs("a", "c")

	errs := CheckLatestVersions(context.Background(), f, insts, defs, 2)
	for i, e := range errs {
		if e != nil {
			t.Errorf("errs[%d] = %v, want nil", i, e)
		}
	}
	if insts[0].LatestVersion == nil {
		t.Errorf("insts[0].LatestVersion should be filled")
	}
	if insts[1].LatestVersion != nil {
		t.Errorf("insts[1].LatestVersion should be nil (unknown agent)")
	}
	if insts[2].LatestVersion != nil {
		t.Errorf("insts[2].LatestVersion should be nil (unknown method)")
	}
	if f.calls["ghost"] != 0 || f.calls["c"] != 0 {
		t.Errorf("fetcher was called for skipped installations: %+v", f.calls)
	}
}

func TestCheckLatestVersions_HonoursConcurrencyLimit(t *testing.T) {
	f := newFakeFetcher()
	f.delay = 20 * time.Millisecond

	const n = 16
	const limit = 4

	insts := make([]*agent.Installation, n)
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("agent-%d", i)
		ids[i] = id
		insts[i] = mkInstall(id, "npm")
	}
	defs := mkAgentDefs(ids...)

	errs := CheckLatestVersions(context.Background(), f, insts, defs, limit)
	for i, e := range errs {
		if e != nil {
			t.Fatalf("errs[%d] = %v, want nil", i, e)
		}
	}

	peak := atomic.LoadInt32(&f.peak)
	if peak > int32(limit) {
		t.Errorf("peak concurrency = %d, want <= %d", peak, limit)
	}
	if peak == 0 {
		t.Errorf("peak concurrency = 0; fetcher never invoked")
	}
}

func TestCheckLatestVersions_EmptyInput(t *testing.T) {
	errs := CheckLatestVersions(context.Background(), newFakeFetcher(), nil, nil, 4)
	if errs != nil {
		t.Errorf("errs = %v, want nil", errs)
	}
}

func TestCheckLatestVersions_ContextCancelled(t *testing.T) {
	f := newFakeFetcher()
	f.delay = 50 * time.Millisecond

	insts := []*agent.Installation{
		mkInstall("a", "npm"),
		mkInstall("b", "npm"),
	}
	defs := mkAgentDefs("a", "b")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	errs := CheckLatestVersions(ctx, f, insts, defs, 2)
	// At least one entry should be non-nil (a cancellation error).
	hasErr := false
	for _, e := range errs {
		if e != nil {
			hasErr = true
			break
		}
	}
	if !hasErr {
		t.Errorf("expected at least one cancellation error, got none: %v", errs)
	}
}
