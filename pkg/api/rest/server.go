// Package rest provides the REST API server for AgentManager.
package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/detector"
	"github.com/kevinelliott/agentmanager/pkg/installer"
	"github.com/kevinelliott/agentmanager/pkg/platform"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

// Server is the REST API server.
type Server struct {
	config    *config.Config
	platform  platform.Platform
	store     storage.Store
	detector  *detector.Detector
	catalog   *catalog.Manager
	installer *installer.Manager

	router     chi.Router
	httpServer *http.Server

	// State
	startTime time.Time
}

// ServerConfig configures the REST server.
type ServerConfig struct {
	Address  string
	TLS      bool
	CertFile string
	KeyFile  string
	APIKey   string // Optional API key for authentication
}

// NewServer creates a new REST server.
func NewServer(
	cfg *config.Config,
	plat platform.Platform,
	store storage.Store,
	det *detector.Detector,
	cat *catalog.Manager,
	inst *installer.Manager,
) *Server {
	s := &Server{
		config:    cfg,
		platform:  plat,
		store:     store,
		detector:  det,
		catalog:   cat,
		installer: inst,
		startTime: time.Now(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the router.
func (s *Server) setupRoutes() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(s.corsMiddleware)
	r.Use(s.contentTypeMiddleware)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Status
		r.Get("/status", s.handleGetStatus)

		// Agents
		r.Route("/agents", func(r chi.Router) {
			r.Get("/", s.handleListAgents)
			r.Get("/{key}", s.handleGetAgent)
			r.Post("/", s.handleInstallAgent)
			r.Put("/{key}", s.handleUpdateAgent)
			r.Delete("/{key}", s.handleUninstallAgent)
		})

		// Catalog
		r.Route("/catalog", func(r chi.Router) {
			r.Get("/", s.handleListCatalog)
			r.Get("/{agentID}", s.handleGetCatalogAgent)
			r.Post("/refresh", s.handleRefreshCatalog)
			r.Get("/search", s.handleSearchCatalog)
		})

		// Updates
		r.Get("/updates", s.handleCheckUpdates)
		r.Get("/changelog/{agentID}", s.handleGetChangelog)
	})

	// Health check
	r.Get("/health", s.handleHealth)

	// OpenAPI specification
	r.Get("/openapi.yaml", s.handleOpenAPISpec)
	r.Get("/openapi.json", s.handleOpenAPISpecJSON)

	s.router = r
}

// Start starts the REST server.
func (s *Server) Start(ctx context.Context, cfg ServerConfig) error {
	s.httpServer = &http.Server{
		Addr:         cfg.Address,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		var err error
		if cfg.TLS && cfg.CertFile != "" && cfg.KeyFile != "" {
			err = s.httpServer.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
		} else {
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			// Log error
		}
	}()

	return nil
}

// Stop gracefully stops the REST server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// Address returns the server's listening address.
func (s *Server) Address() string {
	if s.httpServer != nil {
		return s.httpServer.Addr
	}
	return ""
}

// Middleware

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) contentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// Handlers

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	// Get agent definitions from catalog
	ctx := r.Context()
	agentDefs, _ := s.catalog.GetAgentsForPlatform(ctx, string(s.platform.ID()))

	// Detect agents
	agents, _ := s.detector.DetectAll(ctx, agentDefs)

	agentCount := len(agents)
	updatesAvailable := 0
	for _, inst := range agents {
		if inst.HasUpdate() {
			updatesAvailable++
		}
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"running":              true,
		"uptime_seconds":       int64(time.Since(s.startTime).Seconds()),
		"agent_count":          agentCount,
		"updates_available":    updatesAvailable,
		"last_catalog_refresh": time.Time{},
		"last_update_check":    time.Time{},
		"version":              "dev",
	})
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	offset, _ := strconv.Atoi(query.Get("offset"))

	// Get agent definitions from catalog
	agentDefs, _ := s.catalog.GetAgentsForPlatform(ctx, string(s.platform.ID()))

	// Detect agents
	agents, err := s.detector.DetectAll(ctx, agentDefs)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Failed to detect agents", err)
		return
	}

	// Apply pagination
	total := len(agents)
	if offset > 0 && offset < len(agents) {
		agents = agents[offset:]
	}
	if limit > 0 && limit < len(agents) {
		agents = agents[:limit]
	}

	// Convert to API format
	result := make([]map[string]interface{}, len(agents))
	for i, inst := range agents {
		result[i] = s.installationToMap(inst)
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"agents": result,
		"total":  total,
	})
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "key")

	// Get agent definitions from catalog
	agentDefs, _ := s.catalog.GetAgentsForPlatform(ctx, string(s.platform.ID()))

	// Detect agents
	agents, err := s.detector.DetectAll(ctx, agentDefs)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Failed to detect agents", err)
		return
	}

	for _, inst := range agents {
		if inst.Key() == key {
			s.respondJSON(w, http.StatusOK, map[string]interface{}{
				"agent": s.installationToMap(inst),
			})
			return
		}
	}

	s.respondError(w, http.StatusNotFound, "Agent not found", nil)
}

func (s *Server) handleInstallAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		AgentID string `json:"agent_id"`
		Method  string `json:"method"`
		Global  bool   `json:"global"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if s.installer == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Installer not available", nil)
		return
	}

	// Get agent definition from catalog
	agentDef, err := s.catalog.GetAgent(ctx, req.AgentID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "Agent not found in catalog", err)
		return
	}

	// Find the install method
	methodDef, ok := agentDef.GetInstallMethod(req.Method)
	if !ok {
		s.respondError(w, http.StatusBadRequest, "Install method not available for this agent", nil)
		return
	}

	// Install the agent
	result, err := s.installer.Install(ctx, *agentDef, methodDef, req.Global)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Installation failed", err)
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Installed version %s", result.Version),
		"version": result.Version,
	})
}

func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "key")

	if s.installer == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Installer not available", nil)
		return
	}

	// Get agent definitions from catalog
	agentDefs, _ := s.catalog.GetAgentsForPlatform(ctx, string(s.platform.ID()))

	// Detect agents to find the installation
	agents, err := s.detector.DetectAll(ctx, agentDefs)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Failed to detect agents", err)
		return
	}

	var inst *agent.Installation
	for _, a := range agents {
		if a.Key() == key {
			inst = a
			break
		}
	}

	if inst == nil {
		s.respondError(w, http.StatusNotFound, "Agent not found", nil)
		return
	}

	// Get agent definition from catalog
	agentDef, err := s.catalog.GetAgent(ctx, inst.AgentID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "Agent not found in catalog", err)
		return
	}

	// Find the install method
	methodDef, ok := agentDef.GetInstallMethod(string(inst.Method))
	if !ok {
		s.respondError(w, http.StatusBadRequest, "Install method not available for this agent", nil)
		return
	}

	fromVersion := inst.InstalledVersion.String()

	// Update the agent
	result, err := s.installer.Update(ctx, inst, *agentDef, methodDef)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Update failed", err)
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      fmt.Sprintf("Updated from %s to %s", fromVersion, result.Version),
		"from_version": fromVersion,
		"to_version":   result.Version,
	})
}

func (s *Server) handleUninstallAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := chi.URLParam(r, "key")

	if s.installer == nil {
		s.respondError(w, http.StatusServiceUnavailable, "Installer not available", nil)
		return
	}

	// Get agent definitions from catalog
	agentDefs, _ := s.catalog.GetAgentsForPlatform(ctx, string(s.platform.ID()))

	// Detect agents to find the installation
	agents, err := s.detector.DetectAll(ctx, agentDefs)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Failed to detect agents", err)
		return
	}

	var inst *agent.Installation
	for _, a := range agents {
		if a.Key() == key {
			inst = a
			break
		}
	}

	if inst == nil {
		s.respondError(w, http.StatusNotFound, "Agent not found", nil)
		return
	}

	// Get agent definition from catalog
	agentDef, err := s.catalog.GetAgent(ctx, inst.AgentID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "Agent not found in catalog", err)
		return
	}

	// Find the install method
	methodDef, ok := agentDef.GetInstallMethod(string(inst.Method))
	if !ok {
		s.respondError(w, http.StatusBadRequest, "Install method not available for this agent", nil)
		return
	}

	// Uninstall the agent
	if err := s.installer.Uninstall(ctx, inst, methodDef); err != nil {
		s.respondError(w, http.StatusInternalServerError, "Uninstallation failed", err)
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Agent uninstalled successfully",
	})
}

func (s *Server) handleListCatalog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	platformID := r.URL.Query().Get("platform")

	var agents []catalog.AgentDef
	var err error

	if platformID != "" {
		agents, err = s.catalog.GetAgentsForPlatform(ctx, platformID)
	} else {
		cat, catErr := s.catalog.Get(ctx)
		if catErr != nil {
			s.respondError(w, http.StatusInternalServerError, "Failed to get catalog", catErr)
			return
		}
		agents = cat.GetAgents()
	}

	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Failed to get catalog", err)
		return
	}

	result := make([]map[string]interface{}, len(agents))
	for i, a := range agents {
		result[i] = s.catalogAgentToMap(&a)
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"agents": result,
		"total":  len(result),
	})
}

func (s *Server) handleGetCatalogAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentID := chi.URLParam(r, "agentID")

	agentDef, err := s.catalog.GetAgent(ctx, agentID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "Agent not found", err)
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"agent": s.catalogAgentToMap(agentDef),
	})
}

func (s *Server) handleRefreshCatalog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	result, err := s.catalog.Refresh(ctx)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Failed to refresh catalog", err)
		return
	}

	cat, err := s.catalog.Get(ctx)
	if err != nil {
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"updated": result.Updated,
			"message": "Catalog refreshed",
		})
		return
	}

	message := "Catalog already up to date"
	if result.Updated {
		message = "Catalog updated successfully"
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"updated":     result.Updated,
		"message":     message,
		"version":     cat.Version,
		"agent_count": len(cat.Agents),
	})
}

func (s *Server) handleSearchCatalog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("q")
	platformID := r.URL.Query().Get("platform")

	agents, err := s.catalog.Search(ctx, query)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Search failed", err)
		return
	}

	// Filter by platform if specified
	var filtered []catalog.AgentDef
	for _, a := range agents {
		if platformID == "" || a.IsSupported(platformID) {
			filtered = append(filtered, a)
		}
	}

	result := make([]map[string]interface{}, len(filtered))
	for i, a := range filtered {
		result[i] = s.catalogAgentToMap(&a)
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"agents": result,
		"total":  len(result),
	})
}

func (s *Server) handleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get agent definitions from catalog
	agentDefs, _ := s.catalog.GetAgentsForPlatform(ctx, string(s.platform.ID()))

	// Detect agents
	agents, err := s.detector.DetectAll(ctx, agentDefs)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "Failed to detect agents", err)
		return
	}

	var updates []map[string]interface{}
	for _, inst := range agents {
		if inst.HasUpdate() {
			updates = append(updates, map[string]interface{}{
				"installation": s.installationToMap(inst),
				"from_version": inst.InstalledVersion.String(),
				"to_version":   inst.LatestVersion.String(),
			})
		}
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"updates": updates,
		"total":   len(updates),
	})
}

func (s *Server) handleGetChangelog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentID := chi.URLParam(r, "agentID")
	fromVersion := r.URL.Query().Get("from")
	toVersion := r.URL.Query().Get("to")

	if fromVersion == "" || toVersion == "" {
		s.respondError(w, http.StatusBadRequest, "from and to version parameters required", nil)
		return
	}

	fromVer, err := agent.ParseVersion(fromVersion)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid from version", err)
		return
	}

	toVer, err := agent.ParseVersion(toVersion)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "Invalid to version", err)
		return
	}

	changelog, err := s.catalog.GetChangelog(ctx, agentID, fromVer, toVer)
	if err != nil {
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"changelog": "",
		})
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"changelog": changelog,
	})
}

// Helper methods

func (s *Server) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) respondError(w http.ResponseWriter, status int, message string, err error) {
	response := map[string]interface{}{
		"error":   message,
		"success": false,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) installationToMap(inst *agent.Installation) map[string]interface{} {
	latestVer := ""
	if inst.LatestVersion != nil {
		latestVer = inst.LatestVersion.String()
	}

	return map[string]interface{}{
		"key":               inst.Key(),
		"agent_id":          inst.AgentID,
		"agent_name":        inst.AgentName,
		"install_method":    string(inst.Method),
		"installed_version": inst.InstalledVersion.String(),
		"latest_version":    latestVer,
		"executable_path":   inst.ExecutablePath,
		"install_path":      inst.InstallPath,
		"is_global":         inst.IsGlobal,
		"detected_at":       inst.DetectedAt,
		"last_checked":      inst.LastChecked,
		"metadata":          inst.Metadata,
		"has_update":        inst.HasUpdate(),
		"status":            string(inst.GetStatus()),
	}
}

func (s *Server) catalogAgentToMap(def *catalog.AgentDef) map[string]interface{} {
	methods := make([]map[string]interface{}, 0, len(def.InstallMethods))
	for _, m := range def.InstallMethods {
		methods = append(methods, map[string]interface{}{
			"method":    m.Method,
			"package":   m.Package,
			"command":   m.Command,
			"platforms": m.Platforms,
		})
	}

	return map[string]interface{}{
		"id":              def.ID,
		"name":            def.Name,
		"description":     def.Description,
		"category":        def.Category,
		"tags":            def.Tags,
		"homepage":        def.Homepage,
		"repository":      def.Repository,
		"install_methods": methods,
	}
}

// handleOpenAPISpec serves the OpenAPI specification in YAML format.
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Write([]byte(openAPISpec))
}

// handleOpenAPISpecJSON serves the OpenAPI specification in JSON format.
func (s *Server) handleOpenAPISpecJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Simple YAML to JSON conversion for the spec
	// Since we embed the spec as a string, we serve it directly
	// For JSON, clients can use yaml-to-json conversion tools
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "Use /openapi.yaml for the full OpenAPI specification",
		"spec_url": "/openapi.yaml",
	})
}

// openAPISpec is the embedded OpenAPI specification.
// This is a simplified version; the full spec is in api/openapi.yaml.
const openAPISpec = `openapi: 3.0.3
info:
  title: AgentManager REST API
  description: REST API for managing AI development agents.
  version: 1.0.0
  contact:
    name: Kevin Elliott
    url: https://github.com/kevinelliott/agentmanager
  license:
    name: MIT
servers:
  - url: http://localhost:8080/api/v1
    description: Local development server
paths:
  /health:
    get:
      summary: Health check
      responses:
        "200":
          description: Server is healthy
  /status:
    get:
      summary: Get server status
      responses:
        "200":
          description: Server status
  /agents:
    get:
      summary: List installed agents
      parameters:
        - name: limit
          in: query
          schema:
            type: integer
        - name: offset
          in: query
          schema:
            type: integer
      responses:
        "200":
          description: List of agents
    post:
      summary: Install an agent
      responses:
        "200":
          description: Agent installed
  /agents/{key}:
    get:
      summary: Get agent details
      parameters:
        - name: key
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Agent details
    put:
      summary: Update an agent
      responses:
        "200":
          description: Agent updated
    delete:
      summary: Uninstall an agent
      responses:
        "200":
          description: Agent uninstalled
  /catalog:
    get:
      summary: List catalog agents
      responses:
        "200":
          description: Catalog agents
  /catalog/{agentID}:
    get:
      summary: Get catalog agent
      responses:
        "200":
          description: Catalog agent details
  /catalog/refresh:
    post:
      summary: Refresh catalog
      responses:
        "200":
          description: Catalog refreshed
  /catalog/search:
    get:
      summary: Search catalog
      parameters:
        - name: q
          in: query
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Search results
  /updates:
    get:
      summary: Check for updates
      responses:
        "200":
          description: Available updates
  /changelog/{agentID}:
    get:
      summary: Get changelog
      parameters:
        - name: from
          in: query
          required: true
          schema:
            type: string
        - name: to
          in: query
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Changelog
`
