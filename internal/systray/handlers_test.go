package systray

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/ipc"
)

// mkTestApp builds a minimal *App sufficient for testing pure IPC/menu
// methods. It deliberately leaves out store/detector/catalog/installer —
// those handlers need separate integration setup.
func mkTestApp(agents ...agent.Installation) *App {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
		shutdownCh: make(chan struct{}),
		startTime:  time.Now().Add(-42 * time.Second),
		version:    "test",
		agents:     append([]agent.Installation(nil), agents...),
	}
	return a
}

func mkInstallation(agentID, method, installedVer, latestVer string) agent.Installation {
	inst := agent.Installation{
		AgentID:          agentID,
		AgentName:        agentID,
		Method:           agent.InstallMethod(method),
		ExecutablePath:   "/usr/local/bin/" + agentID,
		InstalledVersion: mustParseVersion(installedVer),
	}
	if latestVer != "" {
		v := mustParseVersion(latestVer)
		inst.LatestVersion = &v
	}
	return inst
}

func mustParseVersion(v string) agent.Version {
	out, _ := agent.ParseVersion(v)
	return out
}

func TestHandleListAgents_Empty(t *testing.T) {
	a := mkTestApp()
	req, _ := ipc.NewMessage(ipc.MessageTypeListAgents, nil)

	resp, err := a.handleListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Type != ipc.MessageTypeSuccess {
		t.Errorf("Type = %q, want Success", resp.Type)
	}

	var out ipc.ListAgentsResponse
	if err := resp.DecodePayload(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Total != 0 || len(out.Agents) != 0 {
		t.Errorf("expected empty response, got Total=%d Agents=%d", out.Total, len(out.Agents))
	}
}

func TestHandleListAgents_Populated(t *testing.T) {
	a := mkTestApp(
		mkInstallation("aider", "brew", "0.86.1", ""),
		mkInstallation("codex", "npm", "1.0.0", "1.0.1"),
	)

	req, _ := ipc.NewMessage(ipc.MessageTypeListAgents, nil)
	resp, _ := a.handleListAgents(context.Background(), req)

	var out ipc.ListAgentsResponse
	_ = resp.DecodePayload(&out)

	if out.Total != 2 {
		t.Errorf("Total = %d, want 2", out.Total)
	}

	// Verify the returned slice is a copy — mutating it must not affect the App.
	out.Agents[0].AgentName = "tampered"
	a.agentsMu.RLock()
	original := a.agents[0].AgentName
	a.agentsMu.RUnlock()
	if original == "tampered" {
		t.Error("handleListAgents returned a shared slice — App state was mutated")
	}
}

func TestHandleGetAgent_Found(t *testing.T) {
	inst := mkInstallation("aider", "brew", "0.86.1", "")
	a := mkTestApp(inst)

	req, _ := ipc.NewMessage(ipc.MessageTypeGetAgent, ipc.GetAgentRequest{Key: inst.Key()})
	resp, _ := a.handleGetAgent(context.Background(), req)

	if resp.Type != ipc.MessageTypeSuccess {
		t.Fatalf("Type = %q, want Success", resp.Type)
	}

	var out ipc.GetAgentResponse
	_ = resp.DecodePayload(&out)
	if out.Agent == nil || out.Agent.AgentID != "aider" {
		t.Errorf("did not get aider back: %+v", out)
	}
}

func TestHandleGetAgent_NotFound(t *testing.T) {
	a := mkTestApp(mkInstallation("aider", "brew", "0.86.1", ""))

	req, _ := ipc.NewMessage(ipc.MessageTypeGetAgent, ipc.GetAgentRequest{Key: "missing:key:x"})
	resp, _ := a.handleGetAgent(context.Background(), req)

	var out ipc.GetAgentResponse
	_ = resp.DecodePayload(&out)
	if out.Agent != nil {
		t.Errorf("expected nil agent for missing key, got %+v", out.Agent)
	}
}

func TestHandleGetAgent_InvalidPayload(t *testing.T) {
	a := mkTestApp()

	// Hand-craft a message with a non-decodable payload.
	req := &ipc.Message{Type: ipc.MessageTypeGetAgent, Payload: []byte(`not-json`)}
	resp, err := a.handleGetAgent(context.Background(), req)
	if err != nil {
		t.Fatalf("handler should not return a Go error: %v", err)
	}
	if resp.Type != ipc.MessageTypeError {
		t.Errorf("Type = %q, want Error", resp.Type)
	}
}

func TestHandleGetStatus(t *testing.T) {
	a := mkTestApp(
		mkInstallation("a", "npm", "1.0.0", "1.0.1"),  // has update
		mkInstallation("b", "brew", "2.0.0", "2.0.0"), // no update (equal)
		mkInstallation("c", "brew", "3.0.0", ""),      // unknown latest
		mkInstallation("d", "npm", "0.1.0", "0.2.0"),  // has update
	)
	a.lastRefresh = time.Now().Add(-5 * time.Minute)
	a.lastCheck = time.Now().Add(-1 * time.Minute)

	req, _ := ipc.NewMessage(ipc.MessageTypeGetStatus, nil)
	resp, _ := a.handleGetStatus(context.Background(), req)

	if resp.Type != ipc.MessageTypeSuccess {
		t.Fatalf("Type = %q, want Success", resp.Type)
	}
	var out ipc.StatusResponse
	_ = resp.DecodePayload(&out)

	if !out.Running {
		t.Error("Running should be true")
	}
	if out.AgentCount != 4 {
		t.Errorf("AgentCount = %d, want 4", out.AgentCount)
	}
	if out.UpdatesAvailable != 2 {
		t.Errorf("UpdatesAvailable = %d, want 2", out.UpdatesAvailable)
	}
	if out.Uptime < 40 {
		t.Errorf("Uptime = %d, want >=40 (startTime was 42s ago)", out.Uptime)
	}
	if out.LastCatalogRefresh.IsZero() || out.LastUpdateCheck.IsZero() {
		t.Error("timestamps should be non-zero")
	}
}

func TestHandleIPCMessage_Dispatch(t *testing.T) {
	a := mkTestApp()

	cases := []struct {
		name string
		msg  ipc.MessageType
		want ipc.MessageType
	}{
		{"list routes to success", ipc.MessageTypeListAgents, ipc.MessageTypeSuccess},
		{"status routes to success", ipc.MessageTypeGetStatus, ipc.MessageTypeSuccess},
		{"unknown routes to error", ipc.MessageType("nope"), ipc.MessageTypeError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := ipc.NewMessage(tc.msg, nil)
			resp, _ := a.handleIPCMessage(context.Background(), req)
			if resp.Type != tc.want {
				t.Errorf("Type = %q, want %q", resp.Type, tc.want)
			}
		})
	}
}

func TestFormatAgentMenuTitle(t *testing.T) {
	a := mkTestApp()

	// Up-to-date (no LatestVersion)
	title := a.formatAgentMenuTitle(mkInstallation("aider", "brew", "0.86.1", ""))
	if !strings.HasPrefix(title, "●") {
		t.Errorf("up-to-date prefix = %q, want ●", title)
	}
	for _, want := range []string{"aider", "(brew)", "0.86.1"} {
		if !strings.Contains(title, want) {
			t.Errorf("missing %q in %q", want, title)
		}
	}

	// Has update — equal versions mean no update; use strictly newer to force
	inst := mkInstallation("codex", "npm", "1.0.0", "1.2.0")
	title = a.formatAgentMenuTitle(inst)
	if !strings.HasPrefix(title, "⬆") {
		t.Errorf("with-update prefix = %q, want ⬆", title)
	}
	for _, want := range []string{"codex", "(npm)", "1.0.0", "1.2.0"} {
		if !strings.Contains(title, want) {
			t.Errorf("missing %q in %q", want, title)
		}
	}

	// Empty method → no parenthetical segment
	blank2 := agent.Installation{AgentID: "y", AgentName: "y", InstalledVersion: mustParseVersion("1.0.0")}
	title = a.formatAgentMenuTitle(blank2)
	if strings.Contains(title, "()") {
		t.Errorf("empty method should not produce empty parens, got %q", title)
	}
}

func TestRequestShutdown_Idempotent(t *testing.T) {
	a := mkTestApp()

	// First call closes shutdownCh; subsequent calls must be no-ops.
	a.requestShutdown()
	a.requestShutdown()
	a.requestShutdown()

	select {
	case <-a.shutdownCh:
		// expected: channel closed, receive returns immediately
	case <-time.After(50 * time.Millisecond):
		t.Fatal("shutdownCh was not closed after requestShutdown")
	}
}

func TestRequestShutdown_ConcurrentSafe(t *testing.T) {
	a := mkTestApp()

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.requestShutdown()
		}()
	}
	wg.Wait()

	select {
	case <-a.shutdownCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("shutdownCh was not closed after concurrent requestShutdown")
	}
}

func TestDialogTracking(t *testing.T) {
	a := mkTestApp()

	// Track a couple of inert commands (we never call .Run()).
	cmd1 := exec.Command("true")
	cmd2 := exec.Command("true")

	a.trackDialog(cmd1)
	a.trackDialog(cmd2)

	a.dialogProcsMu.Lock()
	if len(a.dialogProcs) != 2 {
		t.Errorf("dialogProcs len = %d, want 2", len(a.dialogProcs))
	}
	a.dialogProcsMu.Unlock()

	// Untrack one → length drops to 1 and remaining is cmd2.
	a.untrackDialog(cmd1)

	a.dialogProcsMu.Lock()
	if len(a.dialogProcs) != 1 || a.dialogProcs[0] != cmd2 {
		t.Errorf("after untrack: dialogProcs = %v", a.dialogProcs)
	}
	a.dialogProcsMu.Unlock()

	// Untrack something not tracked is a no-op (exercise the loop miss).
	a.untrackDialog(cmd1)
	a.dialogProcsMu.Lock()
	if len(a.dialogProcs) != 1 {
		t.Errorf("untrack of missing cmd changed length: %d", len(a.dialogProcs))
	}
	a.dialogProcsMu.Unlock()
}
