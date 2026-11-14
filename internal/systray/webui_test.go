package systray

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/config"
)

func mkTestUI(t *testing.T, a *App) *uiServer {
	t.Helper()
	u, err := newUIServer(a)
	if err != nil {
		t.Fatalf("newUIServer: %v", err)
	}
	t.Cleanup(func() { _ = u.listener.Close() })
	a.ui = u
	return u
}

func uiRequest(u *uiServer, method, path string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Host = u.listener.Addr().String()
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	u.server.Handler.ServeHTTP(rec, req)
	return rec
}

func TestUIServesIndexOnLoopbackOnly(t *testing.T) {
	u := mkTestUI(t, mkTestApp())

	if !strings.HasPrefix(u.listener.Addr().String(), "127.0.0.1:") {
		t.Fatalf("UI listens on %s, want loopback", u.listener.Addr())
	}

	rec := uiRequest(u, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "AgentManager") {
		t.Fatalf("GET / = %d, body %q", rec.Code, rec.Body.String())
	}
}

func TestUIGuardRejectsForeignHostAndOrigin(t *testing.T) {
	u := mkTestUI(t, mkTestApp())

	// A DNS-rebinding page reaches us with its own name in Host.
	rec := uiRequest(u, http.MethodGet, "/", func(r *http.Request) { r.Host = "evil.example:1234" })
	if rec.Code != http.StatusForbidden {
		t.Errorf("foreign Host: got %d, want 403", rec.Code)
	}

	rec = uiRequest(u, http.MethodPost, "/api/refresh", func(r *http.Request) {
		r.Header.Set("Origin", "https://evil.example")
		r.AddCookie(&http.Cookie{Name: uiCookieName, Value: u.token})
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("foreign Origin: got %d, want 403", rec.Code)
	}
}

func TestUIAPIRequiresSession(t *testing.T) {
	u := mkTestUI(t, mkTestApp())

	rec := uiRequest(u, http.MethodPost, "/api/jobs/clear", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no cookie: got %d, want 401", rec.Code)
	}

	rec = uiRequest(u, http.MethodPost, "/api/jobs/clear", func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: uiCookieName, Value: "wrong"})
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong cookie: got %d, want 401", rec.Code)
	}

	rec = uiRequest(u, http.MethodPost, "/api/jobs/clear", func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: uiCookieName, Value: u.token})
	})
	if rec.Code != http.StatusOK {
		t.Errorf("valid cookie: got %d, want 200", rec.Code)
	}
}

func TestUITokenExchange(t *testing.T) {
	u := mkTestUI(t, mkTestApp())

	rec := uiRequest(u, http.MethodGet, "/?t=nope", nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("bad token: got %d, want 403", rec.Code)
	}

	rec = uiRequest(u, http.MethodGet, "/?t="+u.token, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("good token: got %d to %q, want 303 to /", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != u.token || !cookies[0].HttpOnly {
		t.Errorf("session cookie not set as expected: %+v", cookies)
	}

	if got := u.entryURL("/settings"); !strings.HasSuffix(got, "/?t="+u.token+"#/settings") {
		t.Errorf("entryURL = %q", got)
	}
}

func TestUIEnqueueRefusesDuplicates(t *testing.T) {
	u := mkTestUI(t, mkTestApp())

	if _, err := u.enqueue("codex", "Codex", "npm", actionUpdate); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	if _, err := u.enqueue("codex", "Codex", "npm", actionUpdate); err == nil {
		t.Error("second enqueue for the same agent and method should be refused")
	}
	// A different install method of the same agent is independent.
	if _, err := u.enqueue("codex", "Codex", "brew", actionUpdate); err != nil {
		t.Errorf("other method: %v", err)
	}
}

func TestUIEnqueueAllUpdates(t *testing.T) {
	a := mkTestApp(
		mkInstallation("codex", "npm", "1.0.0", "2.0.0"),
		mkInstallation("amp", "npm", "1.0.0", "1.0.0"),
		mkInstallation("opencode", "brew", "0.1.0", "0.2.0"),
	)
	u := mkTestUI(t, a)

	if got := u.enqueueAllUpdates(); got != 2 {
		t.Errorf("queued %d updates, want 2", got)
	}
	if got := u.enqueueAllUpdates(); got != 0 {
		t.Errorf("re-queued %d updates while the first are pending, want 0", got)
	}
}

func TestSaveSettingIsSafeForConcurrentUse(t *testing.T) {
	a := mkTestApp()
	a.config = &config.Config{}

	// Run with -race: overlapping saves and reads must not race.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(on bool) {
			defer wg.Done()
			_ = a.saveSetting("updates.notify", on, func(c *config.Config) { c.Updates.Notify = on })
		}(i%2 == 0)
		go func() {
			defer wg.Done()
			_ = a.notifyEnabled()
			_ = a.autoCheckEnabled()
		}()
	}
	wg.Wait()

	if err := a.saveSetting("updates.auto_check", true, func(c *config.Config) { c.Updates.AutoCheck = true }); err != nil {
		t.Fatalf("saveSetting: %v", err)
	}
	if !a.autoCheckEnabled() {
		t.Error("saveSetting did not apply the value")
	}
}

func TestTrayTitles(t *testing.T) {
	cases := map[int]string{0: "No agents installed", 1: "1 agent installed", 7: "7 agents installed"}
	for n, want := range cases {
		if got := statusTitle(n); got != want {
			t.Errorf("statusTitle(%d) = %q, want %q", n, got, want)
		}
	}
	if got := updateAllTitle(1); got != "Update 1 Agent" {
		t.Errorf("updateAllTitle(1) = %q", got)
	}
	if got := updateAllTitle(3); got != "Update 3 Agents" {
		t.Errorf("updateAllTitle(3) = %q", got)
	}
}
