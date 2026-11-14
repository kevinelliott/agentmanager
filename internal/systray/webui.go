package systray

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/config"
)

// The desktop window is a web UI embedded in the helper and served only to
// this machine. One UI serves macOS, Windows and Linux; each platform only
// decides how to put it in a window (see window_darwin.go / window_other.go).

//go:embed webui
var webUIFiles embed.FS

const (
	uiCookieName = "agentmgr_ui"

	// cliInstallPath is where the CLI is installed on macOS and Linux.
	cliInstallPath = "/usr/local/bin/agentmgr"
	uiMaxJobs      = 100

	jobQueued  = "queued"
	jobRunning = "running"
	jobDone    = "done"
	jobFailed  = "failed"
)

// uiJob is one install, update or uninstall requested from the window.
type uiJob struct {
	ID         string     `json:"id"`
	AgentID    string     `json:"agent_id"`
	AgentName  string     `json:"agent_name"`
	Method     string     `json:"method"`
	Action     string     `json:"action"`
	Status     string     `json:"status"`
	Detail     string     `json:"detail,omitempty"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// uiServer serves the embedded web UI and its JSON API on loopback.
type uiServer struct {
	app      *App
	token    string
	listener net.Listener
	server   *http.Server

	jobsMu sync.Mutex
	jobs   []*uiJob
	jobSeq int
	jobCh  chan *uiJob

	refreshing atomic.Bool
}

// newUIServer binds a random loopback port for the UI without serving yet.
func newUIServer(a *App) (*uiServer, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate UI token: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for UI: %w", err)
	}

	u := &uiServer{
		app:      a,
		token:    hex.EncodeToString(tokenBytes),
		listener: ln,
		jobCh:    make(chan *uiJob, uiMaxJobs),
	}
	u.server = &http.Server{
		Handler:           u.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return u, nil
}

// startUIServer starts serving the UI and running the jobs it requests.
func (a *App) startUIServer() error {
	u, err := newUIServer(a)
	if err != nil {
		return err
	}
	a.ui = u

	go func() {
		if err := u.server.Serve(u.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.reportError("serve UI", err)
		}
	}()

	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		u.jobWorker()
	}()
	return nil
}

func (u *uiServer) stop(ctx context.Context) error {
	return u.server.Shutdown(ctx)
}

// origin is the only origin the UI is ever served from.
func (u *uiServer) origin() string {
	return "http://" + u.listener.Addr().String()
}

// entryURL is the address a window opens. The one-time token in it is traded
// for a cookie on first load, so it never stays in the address bar. route is
// a UI route such as "/settings" or "/agent/claude-code".
func (u *uiServer) entryURL(route string) string {
	url := u.origin() + "/?t=" + u.token
	if route != "" {
		url += "#" + route
	}
	return url
}

func (u *uiServer) routes() http.Handler {
	mux := http.NewServeMux()

	static, err := fs.Sub(webUIFiles, "webui")
	if err != nil {
		panic(err) // the directory is embedded at build time
	}
	files := http.FileServer(http.FS(static))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if t := r.URL.Query().Get("t"); t != "" {
				if !u.tokenMatches(t) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				http.SetCookie(w, &http.Cookie{
					Name:     uiCookieName,
					Value:    u.token,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteStrictMode,
				})
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		files.ServeHTTP(w, r)
	})

	mux.HandleFunc("GET /api/state", u.handleState)
	mux.HandleFunc("GET /api/catalog", u.handleCatalog)
	mux.HandleFunc("POST /api/refresh", u.handleRefresh)
	mux.HandleFunc("POST /api/action", u.handleAction)
	mux.HandleFunc("POST /api/update-all", u.handleUpdateAll)
	mux.HandleFunc("POST /api/jobs/clear", u.handleClearJobs)
	mux.HandleFunc("POST /api/settings", u.handleSettings)
	mux.HandleFunc("POST /api/cli", u.handleCLI)
	mux.HandleFunc("POST /api/open-tui", u.handleOpenTUI)
	mux.HandleFunc("POST /api/open-url", u.handleOpenURL)

	return u.guard(mux)
}

func (u *uiServer) tokenMatches(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(u.token)) == 1
}

// guard keeps other sites and other machines out: the Host must be our
// loopback address (blocks DNS rebinding), cross-origin requests are refused,
// and every API call needs the session cookie.
func (u *uiServer) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != u.listener.Addr().String() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && o != u.origin() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			c, err := r.Cookie(uiCookieName)
			if err != nil || !u.tokenMatches(c.Value) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- state ---------------------------------------------------------------

type uiInstall struct {
	Key        string    `json:"key"`
	AgentID    string    `json:"agent_id"`
	Name       string    `json:"name"`
	Method     string    `json:"method"`
	Version    string    `json:"version"`
	Latest     string    `json:"latest,omitempty"`
	HasUpdate  bool      `json:"has_update"`
	Path       string    `json:"path,omitempty"`
	DetectedAt time.Time `json:"detected_at"`
}

type uiSettings struct {
	Notify          bool   `json:"notify"`
	AutoCheck       bool   `json:"auto_check"`
	AutoStart       bool   `json:"auto_start"`
	CLIPath         string `json:"cli_path"`
	CLIInstalled    bool   `json:"cli_installed"`
	CLICanInstall   bool   `json:"cli_can_install"`
	CLICanUninstall bool   `json:"cli_can_uninstall"`
	TUIAvailable    bool   `json:"tui_available"`
	ConfigPath      string `json:"config_path"`
}

func (u *uiServer) handleState(w http.ResponseWriter, r *http.Request) {
	a := u.app

	a.agentsMu.RLock()
	installs := make([]uiInstall, 0, len(a.agents))
	for _, ag := range a.agents {
		in := uiInstall{
			Key:        ag.Key(),
			AgentID:    ag.AgentID,
			Name:       ag.AgentName,
			Method:     string(ag.Method),
			Version:    ag.InstalledVersion.String(),
			HasUpdate:  ag.HasUpdate(),
			Path:       ag.ExecutablePath,
			DetectedAt: ag.DetectedAt,
		}
		if ag.LatestVersion != nil {
			in.Latest = ag.LatestVersion.String()
		}
		installs = append(installs, in)
	}
	lastRefresh := a.lastRefresh
	a.agentsMu.RUnlock()

	sort.Slice(installs, func(i, j int) bool {
		if li, lj := strings.ToLower(installs[i].Name), strings.ToLower(installs[j].Name); li != lj {
			return li < lj
		}
		return installs[i].Method < installs[j].Method
	})

	u.jobsMu.Lock()
	jobs := make([]uiJob, len(u.jobs))
	for i, j := range u.jobs {
		jobs[i] = *j
	}
	u.jobsMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"version":      a.version,
		"platform":     string(a.platform.ID()),
		"installs":     installs,
		"jobs":         jobs,
		"refreshing":   u.refreshing.Load(),
		"last_refresh": lastRefresh,
		"settings":     u.settings(r.Context()),
	})
}

func (u *uiServer) settings(ctx context.Context) uiSettings {
	a := u.app
	s := uiSettings{
		Notify:          a.config.Updates.Notify,
		AutoCheck:       a.config.Updates.AutoCheck,
		CLICanUninstall: canUninstallCLI,
		ConfigPath:      config.GetConfigPath(),
	}
	if enabled, err := a.platform.IsAutoStartEnabled(ctx); err == nil {
		s.AutoStart = enabled
	}
	// "Installed" means reachable from a terminal, not merely shipped inside
	// the app next to the helper.
	if path, err := exec.LookPath("agentmgr"); err == nil {
		s.CLIPath = path
		s.CLIInstalled = true
	} else if _, err := os.Stat(cliInstallPath); err == nil {
		s.CLIPath = cliInstallPath
		s.CLIInstalled = true
	}
	_, bundledErr := findBundledAgentMgr()
	s.CLICanInstall = bundledErr == nil
	s.CLICanUninstall = canUninstallCLI && s.CLIPath == cliInstallPath
	_, tuiErr := findAgentMgrBinary()
	s.TUIAvailable = tuiErr == nil
	return s
}

// --- catalog -------------------------------------------------------------

type uiMethod struct {
	Name      string `json:"name"`
	Package   string `json:"package,omitempty"`
	Command   string `json:"command,omitempty"`
	Available bool   `json:"available"`
}

type uiCatalogAgent struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Category      string     `json:"category,omitempty"`
	Tags          []string   `json:"tags,omitempty"`
	Homepage      string     `json:"homepage,omitempty"`
	Repository    string     `json:"repository,omitempty"`
	Documentation string     `json:"documentation,omitempty"`
	Vendor        string     `json:"vendor,omitempty"`
	License       string     `json:"license,omitempty"`
	Methods       []uiMethod `json:"methods"`
}

func (u *uiServer) handleCatalog(w http.ResponseWriter, r *http.Request) {
	a := u.app
	platformID := string(a.platform.ID())

	defs, err := a.catalog.GetAgentsForPlatform(r.Context(), platformID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load the agent catalog: "+err.Error())
		return
	}

	// Asking a package manager whether it is installed spawns a process, so
	// ask once per method name rather than once per agent.
	available := map[string]bool{}
	out := make([]uiCatalogAgent, 0, len(defs))
	for _, def := range defs {
		ca := uiCatalogAgent{
			ID:            def.ID,
			Name:          def.Name,
			Description:   def.Description,
			Category:      def.Category,
			Tags:          def.Tags,
			Homepage:      def.Homepage,
			Repository:    def.Repository,
			Documentation: def.Documentation,
			Vendor:        def.Metadata["vendor"],
			License:       def.Metadata["license"],
			Methods:       []uiMethod{},
		}
		for name, m := range def.InstallMethods {
			if !containsString(m.Platforms, platformID) {
				continue
			}
			ok, seen := available[name]
			if !seen {
				ok = a.installer.IsMethodAvailable(name)
				available[name] = ok
			}
			ca.Methods = append(ca.Methods, uiMethod{Name: name, Package: m.Package, Command: m.Command, Available: ok})
		}
		sort.Slice(ca.Methods, func(i, j int) bool {
			if ca.Methods[i].Available != ca.Methods[j].Available {
				return ca.Methods[i].Available
			}
			return ca.Methods[i].Name < ca.Methods[j].Name
		})
		out = append(out, ca)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	writeJSON(w, http.StatusOK, map[string]any{"agents": out})
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// --- actions -------------------------------------------------------------

func (u *uiServer) handleRefresh(w http.ResponseWriter, _ *http.Request) {
	u.refreshInBackground(true)
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// refreshInBackground re-detects agents without blocking the caller. Only
// one refresh runs at a time.
func (u *uiServer) refreshInBackground(force bool) {
	if !u.refreshing.CompareAndSwap(false, true) {
		return
	}
	a := u.app
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer u.refreshing.Store(false)
		if force {
			_ = a.store.ClearDetectionCache(a.ctx)
			a.reportError("refresh agents", a.forceRefreshAgents(a.ctx))
			return
		}
		a.reportError("refresh agents", a.refreshAgents(a.ctx))
	}()
}

type actionRequest struct {
	AgentID string `json:"agent_id"`
	Method  string `json:"method"`
	Action  string `json:"action"`
}

func (u *uiServer) handleAction(w http.ResponseWriter, r *http.Request) {
	var req actionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	switch req.Action {
	case actionInstall, actionUpdate, actionUninstall:
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}

	def, err := u.app.catalog.GetAgent(r.Context(), req.AgentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "That agent is not in the catalog.")
		return
	}
	if _, ok := def.InstallMethods[req.Method]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s cannot be managed with %s.", def.Name, req.Method))
		return
	}

	job, err := u.enqueue(def.ID, def.Name, req.Method, req.Action)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (u *uiServer) handleUpdateAll(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusAccepted, map[string]int{"queued": u.enqueueAllUpdates()})
}

// enqueueAllUpdates queues an update for every installation that has one and
// returns how many were queued.
func (u *uiServer) enqueueAllUpdates() int {
	a := u.app
	a.agentsMu.RLock()
	var pending []agent.Installation
	for _, ag := range a.agents {
		if ag.HasUpdate() {
			pending = append(pending, ag)
		}
	}
	a.agentsMu.RUnlock()

	queued := 0
	for _, ag := range pending {
		if _, err := u.enqueue(ag.AgentID, ag.AgentName, string(ag.Method), actionUpdate); err == nil {
			queued++
		}
	}
	return queued
}

func (u *uiServer) handleClearJobs(w http.ResponseWriter, _ *http.Request) {
	u.jobsMu.Lock()
	kept := u.jobs[:0]
	for _, j := range u.jobs {
		if j.Status == jobQueued || j.Status == jobRunning {
			kept = append(kept, j)
		}
	}
	u.jobs = kept
	u.jobsMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// enqueue records a job and hands it to the worker. A second request for the
// same agent and method while one is still pending is refused.
func (u *uiServer) enqueue(agentID, agentName, method, action string) (*uiJob, error) {
	u.jobsMu.Lock()
	defer u.jobsMu.Unlock()

	for _, j := range u.jobs {
		if j.AgentID == agentID && j.Method == method && (j.Status == jobQueued || j.Status == jobRunning) {
			return nil, fmt.Errorf("%s is already being worked on", agentName)
		}
	}

	u.jobSeq++
	job := &uiJob{
		ID:        fmt.Sprintf("job-%d", u.jobSeq),
		AgentID:   agentID,
		AgentName: agentName,
		Method:    method,
		Action:    action,
		Status:    jobQueued,
		CreatedAt: time.Now(),
	}

	select {
	case u.jobCh <- job:
	default:
		return nil, errors.New("too many operations are waiting; try again once some finish")
	}

	u.jobs = append(u.jobs, job)
	if len(u.jobs) > uiMaxJobs {
		// Drop the oldest finished jobs; pending ones are always kept.
		trimmed := u.jobs[:0]
		excess := len(u.jobs) - uiMaxJobs
		for _, j := range u.jobs {
			if excess > 0 && (j.Status == jobDone || j.Status == jobFailed) {
				excess--
				continue
			}
			trimmed = append(trimmed, j)
		}
		u.jobs = trimmed
	}
	copyJob := *job
	return &copyJob, nil
}

// jobWorker runs jobs one at a time. Package managers do not tolerate
// concurrent runs, and one at a time keeps progress easy to follow.
func (u *uiServer) jobWorker() {
	a := u.app
	for {
		select {
		case <-a.ctx.Done():
			return
		case job := <-u.jobCh:
			u.setJob(job, func(j *uiJob) { j.Status = jobRunning })

			detail, err := u.runJob(job)

			now := time.Now()
			u.setJob(job, func(j *uiJob) {
				j.FinishedAt = &now
				if err != nil {
					j.Status = jobFailed
					j.Error = err.Error()
					return
				}
				j.Status = jobDone
				j.Detail = detail
			})

			// Re-detect once the queue drains so the list reflects reality,
			// including after a failure that may have half-applied.
			if len(u.jobCh) == 0 {
				_ = a.store.ClearDetectionCache(a.ctx)
				u.refreshing.Store(true)
				a.reportError("refresh agents", a.refreshAgents(a.ctx))
				u.refreshing.Store(false)
			}
		}
	}
}

func (u *uiServer) setJob(job *uiJob, mutate func(*uiJob)) {
	u.jobsMu.Lock()
	mutate(job)
	u.jobsMu.Unlock()
}

func (u *uiServer) runJob(job *uiJob) (string, error) {
	a := u.app
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()

	def, err := a.catalog.GetAgent(ctx, job.AgentID)
	if err != nil {
		return "", fmt.Errorf("not found in the catalog: %w", err)
	}
	methodDef, ok := def.InstallMethods[job.Method]
	if !ok {
		return "", fmt.Errorf("install method %q is not defined for %s", job.Method, def.Name)
	}
	methodDef.Method = job.Method

	if job.Action == actionInstall {
		res, err := a.installer.Install(ctx, *def, methodDef, false)
		if err != nil {
			return "", err
		}
		if v := res.Version.String(); v != "" {
			return "Installed " + v, nil
		}
		return "Installed", nil
	}

	inst := a.findInstallation(job.AgentID, job.Method)
	if inst == nil {
		return "", fmt.Errorf("%s is not installed via %s", def.Name, job.Method)
	}

	if job.Action == actionUpdate {
		res, err := a.installer.Update(ctx, inst, *def, methodDef)
		if err != nil {
			return "", err
		}
		if v := res.Version.String(); v != "" {
			return "Updated to " + v, nil
		}
		return "Updated", nil
	}

	if err := a.installer.Uninstall(ctx, inst, methodDef); err != nil {
		return "", err
	}
	return "Uninstalled", nil
}

// findInstallation returns a copy of the detected installation of an agent
// by a given method, or nil.
func (a *App) findInstallation(agentID, method string) *agent.Installation {
	a.agentsMu.RLock()
	defer a.agentsMu.RUnlock()
	for i := range a.agents {
		if a.agents[i].AgentID == agentID && string(a.agents[i].Method) == method {
			inst := a.agents[i]
			return &inst
		}
	}
	return nil
}

// --- settings ------------------------------------------------------------

type settingsRequest struct {
	Notify    *bool `json:"notify"`
	AutoCheck *bool `json:"auto_check"`
	AutoStart *bool `json:"auto_start"`
}

func (u *uiServer) handleSettings(w http.ResponseWriter, r *http.Request) {
	a := u.app
	var req settingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	save := func(key string, value any) error {
		if a.configLoader == nil {
			return nil
		}
		return a.configLoader.SetAndSave(key, value)
	}

	if req.Notify != nil {
		if err := save("updates.notify", *req.Notify); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not save the setting: "+err.Error())
			return
		}
		a.config.Updates.Notify = *req.Notify
	}
	if req.AutoCheck != nil {
		if err := save("updates.auto_check", *req.AutoCheck); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not save the setting: "+err.Error())
			return
		}
		a.config.Updates.AutoCheck = *req.AutoCheck
	}
	if req.AutoStart != nil {
		var err error
		if *req.AutoStart {
			err = a.platform.EnableAutoStart(r.Context())
		} else {
			err = a.platform.DisableAutoStart(r.Context())
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not change start at login: "+err.Error())
			return
		}
		a.syncAutoStartMenu(*req.AutoStart)
	}

	writeJSON(w, http.StatusOK, u.settings(r.Context()))
}

func (u *uiServer) handleCLI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// These prompt for an administrator password on macOS and Linux, so they
	// block until the person answers.
	switch req.Action {
	case actionInstall:
		if _, err := findBundledAgentMgr(); err != nil {
			writeError(w, http.StatusConflict, "This build does not include the command line tool.")
			return
		}
		if !u.app.installOrUpdateCLI() {
			writeError(w, http.StatusConflict, "The command line tool was not installed.")
			return
		}
	case actionUninstall:
		if !canUninstallCLI {
			writeError(w, http.StatusBadRequest, "Removing the command line tool is not supported here.")
			return
		}
		if !u.app.uninstallCLI() {
			writeError(w, http.StatusConflict, "The command line tool was not removed.")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}
	writeJSON(w, http.StatusOK, u.settings(r.Context()))
}

func (u *uiServer) handleOpenTUI(w http.ResponseWriter, _ *http.Request) {
	go u.app.openTUI()
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// handleOpenURL opens an agent's homepage, repository or docs in the default
// browser. Only links that the catalog itself lists are opened.
func (u *uiServer) handleOpenURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string `json:"agent_id"`
		Kind    string `json:"kind"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	def, err := u.app.catalog.GetAgent(r.Context(), req.AgentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "That agent is not in the catalog.")
		return
	}
	var target string
	switch req.Kind {
	case "homepage":
		target = def.Homepage
	case "repository":
		target = def.Repository
	case "documentation":
		target = def.Documentation
	}
	if !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "http://") {
		writeError(w, http.StatusBadRequest, "No link is available.")
		return
	}
	if err := openInDefaultBrowser(target); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not open the link: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
