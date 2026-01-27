package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/detector"
	"github.com/kevinelliott/agentmanager/pkg/installer"
	"github.com/kevinelliott/agentmanager/pkg/platform"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

// Server is the gRPC API server.
type Server struct {
	config    *config.Config
	platform  platform.Platform
	store     storage.Store
	detector  *detector.Detector
	catalog   *catalog.Manager
	installer *installer.Manager

	grpcServer *grpc.Server
	listener   net.Listener

	// State
	agents      []*agent.Installation
	agentsMu    sync.RWMutex
	startTime   time.Time
	lastRefresh time.Time
	lastCheck   time.Time

	// Event subscribers
	subscribers []chan *AgentEvent
	subMu       sync.RWMutex
}

// ServerConfig configures the gRPC server.
type ServerConfig struct {
	Address  string
	TLS      bool
	CertFile string
	KeyFile  string
}

// NewServer creates a new gRPC server.
func NewServer(
	cfg *config.Config,
	plat platform.Platform,
	store storage.Store,
	det *detector.Detector,
	cat *catalog.Manager,
	inst *installer.Manager,
) *Server {
	return &Server{
		config:      cfg,
		platform:    plat,
		store:       store,
		detector:    det,
		catalog:     cat,
		installer:   inst,
		startTime:   time.Now(),
		subscribers: make([]chan *AgentEvent, 0),
	}
}

// Start starts the gRPC server.
func (s *Server) Start(ctx context.Context, cfg ServerConfig) error {
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.listener = listener

	// Create gRPC server with options
	opts := []grpc.ServerOption{}

	// Add TLS support if configured
	if cfg.TLS && cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}

	s.grpcServer = grpc.NewServer(opts...)

	// Register reflection for debugging
	reflection.Register(s.grpcServer)

	// Start serving
	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			// Log error
		}
	}()

	return nil
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop(ctx context.Context) error {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	if s.listener != nil {
		s.listener.Close()
	}
	return nil
}

// Address returns the server's listening address.
func (s *Server) Address() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

// ListAgents returns a list of detected agents.
func (s *Server) ListAgents(ctx context.Context, req *ListAgentsRequest) (*ListAgentsResponse, error) {
	// Refresh agents if needed
	if err := s.refreshAgents(ctx); err != nil {
		return nil, fmt.Errorf("failed to refresh agents: %w", err)
	}

	s.agentsMu.RLock()
	defer s.agentsMu.RUnlock()

	// Apply filter
	var filtered []*agent.Installation
	for _, inst := range s.agents {
		if s.matchesFilter(inst, req.Filter) {
			filtered = append(filtered, inst)
		}
	}

	// Apply pagination
	total := len(filtered)
	if req.Offset > 0 && req.Offset < len(filtered) {
		filtered = filtered[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(filtered) {
		filtered = filtered[:req.Limit]
	}

	// Convert to API format
	agents := make([]*Installation, len(filtered))
	for i, inst := range filtered {
		agents[i] = FromAgentInstallation(inst)
	}

	return &ListAgentsResponse{
		Agents: agents,
		Total:  total,
	}, nil
}

// GetAgent returns a specific agent by key.
func (s *Server) GetAgent(ctx context.Context, req *GetAgentRequest) (*GetAgentResponse, error) {
	s.agentsMu.RLock()
	defer s.agentsMu.RUnlock()

	for _, inst := range s.agents {
		if inst.Key() == req.Key {
			return &GetAgentResponse{
				Agent: FromAgentInstallation(inst),
			}, nil
		}
	}

	return &GetAgentResponse{}, nil
}

// InstallAgent installs an agent.
func (s *Server) InstallAgent(ctx context.Context, req *InstallAgentRequest) (*InstallAgentResponse, error) {
	if s.installer == nil {
		return &InstallAgentResponse{
			Success: false,
			Message: "installer not available",
		}, nil
	}

	// Get agent definition from catalog
	agentDef, err := s.catalog.GetAgent(ctx, req.AgentID)
	if err != nil {
		return &InstallAgentResponse{
			Success: false,
			Message: fmt.Sprintf("agent not found: %v", err),
		}, nil
	}

	// Find the install method
	methodDef, ok := agentDef.GetInstallMethod(req.Method)
	if !ok {
		return &InstallAgentResponse{
			Success: false,
			Message: "install method not available for this agent",
		}, nil
	}

	// Install the agent
	result, err := s.installer.Install(ctx, *agentDef, methodDef, req.Global)
	if err != nil {
		return &InstallAgentResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Refresh agents
	s.refreshAgents(ctx)

	// Find the new installation
	var inst *Installation
	s.agentsMu.RLock()
	for _, a := range s.agents {
		if a.AgentID == req.AgentID && string(a.Method) == req.Method {
			inst = FromAgentInstallation(a)
			break
		}
	}
	s.agentsMu.RUnlock()

	return &InstallAgentResponse{
		Installation: inst,
		Success:      true,
		Message:      fmt.Sprintf("Installed version %s", result.Version.String()),
	}, nil
}

// UpdateAgent updates an agent.
func (s *Server) UpdateAgent(ctx context.Context, req *UpdateAgentRequest) (*UpdateAgentResponse, error) {
	if s.installer == nil {
		return &UpdateAgentResponse{
			Success: false,
			Message: "installer not available",
		}, nil
	}

	// Find the installation
	s.agentsMu.RLock()
	var inst *agent.Installation
	for _, a := range s.agents {
		if a.Key() == req.Key {
			inst = a
			break
		}
	}
	s.agentsMu.RUnlock()

	if inst == nil {
		return &UpdateAgentResponse{
			Success: false,
			Message: "installation not found",
		}, nil
	}

	fromVersion := inst.InstalledVersion.String()

	// Get agent definition from catalog
	agentDef, err := s.catalog.GetAgent(ctx, inst.AgentID)
	if err != nil {
		return &UpdateAgentResponse{
			Success: false,
			Message: fmt.Sprintf("agent not found: %v", err),
		}, nil
	}

	// Find the install method
	methodDef, ok := agentDef.GetInstallMethod(string(inst.Method))
	if !ok {
		return &UpdateAgentResponse{
			Success: false,
			Message: "install method not available for this agent",
		}, nil
	}

	// Update the agent
	result, err := s.installer.Update(ctx, inst, *agentDef, methodDef)
	if err != nil {
		return &UpdateAgentResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Refresh agents
	s.refreshAgents(ctx)

	// Find the updated installation
	var updated *Installation
	s.agentsMu.RLock()
	for _, a := range s.agents {
		if a.Key() == req.Key {
			updated = FromAgentInstallation(a)
			break
		}
	}
	s.agentsMu.RUnlock()

	return &UpdateAgentResponse{
		Installation: updated,
		FromVersion:  fromVersion,
		ToVersion:    result.Version.String(),
		Success:      true,
		Message:      fmt.Sprintf("Updated from %s to %s", fromVersion, result.Version.String()),
	}, nil
}

// UninstallAgent uninstalls an agent.
func (s *Server) UninstallAgent(ctx context.Context, req *UninstallAgentRequest) (*UninstallAgentResponse, error) {
	if s.installer == nil {
		return &UninstallAgentResponse{
			Success: false,
			Message: "installer not available",
		}, nil
	}

	// Find the installation
	s.agentsMu.RLock()
	var inst *agent.Installation
	for _, a := range s.agents {
		if a.Key() == req.Key {
			inst = a
			break
		}
	}
	s.agentsMu.RUnlock()

	if inst == nil {
		return &UninstallAgentResponse{
			Success: false,
			Message: "installation not found",
		}, nil
	}

	// Get agent definition from catalog
	agentDef, err := s.catalog.GetAgent(ctx, inst.AgentID)
	if err != nil {
		return &UninstallAgentResponse{
			Success: false,
			Message: fmt.Sprintf("agent not found: %v", err),
		}, nil
	}

	// Find the install method
	methodDef, ok := agentDef.GetInstallMethod(string(inst.Method))
	if !ok {
		return &UninstallAgentResponse{
			Success: false,
			Message: "install method not available for this agent",
		}, nil
	}

	// Uninstall the agent
	if err := s.installer.Uninstall(ctx, inst, methodDef); err != nil {
		return &UninstallAgentResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Refresh agents
	s.refreshAgents(ctx)

	return &UninstallAgentResponse{
		Success: true,
		Message: "Agent uninstalled successfully",
	}, nil
}

// ListCatalog returns the catalog agents.
func (s *Server) ListCatalog(ctx context.Context, req *ListCatalogRequest) (*ListCatalogResponse, error) {
	var agents []catalog.AgentDef
	var err error

	if req.Platform != "" {
		agents, err = s.catalog.GetAgentsForPlatform(ctx, req.Platform)
	} else {
		cat, catErr := s.catalog.Get(ctx)
		if catErr != nil {
			return nil, catErr
		}
		agents = cat.GetAgents()
		err = nil
	}

	if err != nil {
		return nil, err
	}

	result := make([]*CatalogAgent, len(agents))
	for i, a := range agents {
		def := a
		result[i] = FromCatalogAgentDef(&def)
	}

	return &ListCatalogResponse{
		Agents: result,
		Total:  len(result),
	}, nil
}

// GetCatalogAgent returns a specific catalog agent.
func (s *Server) GetCatalogAgent(ctx context.Context, req *GetCatalogAgentRequest) (*GetCatalogAgentResponse, error) {
	agentDef, err := s.catalog.GetAgent(ctx, req.AgentID)
	if err != nil {
		return &GetCatalogAgentResponse{}, nil
	}

	return &GetCatalogAgentResponse{
		Agent: FromCatalogAgentDef(agentDef),
	}, nil
}

// RefreshCatalog refreshes the catalog.
func (s *Server) RefreshCatalog(ctx context.Context) (*RefreshCatalogResponse, error) {
	result, err := s.catalog.Refresh(ctx)
	if err != nil {
		return &RefreshCatalogResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	cat, err := s.catalog.Get(ctx)
	if err != nil {
		return &RefreshCatalogResponse{
			Success: true,
			Updated: result.Updated,
			Message: "Refreshed but failed to get count",
		}, nil
	}

	message := "Catalog already up to date"
	if result.Updated {
		message = "Catalog updated successfully"
	}

	return &RefreshCatalogResponse{
		Success:    true,
		Updated:    result.Updated,
		Message:    message,
		Version:    cat.Version,
		AgentCount: len(cat.Agents),
	}, nil
}

// SearchCatalog searches the catalog.
func (s *Server) SearchCatalog(ctx context.Context, req *SearchCatalogRequest) (*SearchCatalogResponse, error) {
	agents, err := s.catalog.Search(ctx, req.Query)
	if err != nil {
		return nil, err
	}

	// Filter by platform if specified
	var filtered []catalog.AgentDef
	for _, a := range agents {
		if req.Platform == "" || a.IsSupported(req.Platform) {
			filtered = append(filtered, a)
		}
	}

	result := make([]*CatalogAgent, len(filtered))
	for i, a := range filtered {
		def := a
		result[i] = FromCatalogAgentDef(&def)
	}

	return &SearchCatalogResponse{
		Agents: result,
		Total:  len(result),
	}, nil
}

// CheckUpdates checks for available updates.
func (s *Server) CheckUpdates(ctx context.Context) (*CheckUpdatesResponse, error) {
	if err := s.refreshAgents(ctx); err != nil {
		return nil, err
	}

	s.agentsMu.RLock()
	defer s.agentsMu.RUnlock()

	var updates []*UpdateInfo
	for _, inst := range s.agents {
		if inst.HasUpdate() {
			updates = append(updates, &UpdateInfo{
				Installation: FromAgentInstallation(inst),
				FromVersion:  inst.InstalledVersion.String(),
				ToVersion:    inst.LatestVersion.String(),
			})
		}
	}

	return &CheckUpdatesResponse{
		Updates: updates,
		Total:   len(updates),
	}, nil
}

// GetChangelog gets the changelog for an agent.
func (s *Server) GetChangelog(ctx context.Context, req *GetChangelogRequest) (*GetChangelogResponse, error) {
	fromVer, err := agent.ParseVersion(req.FromVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid from_version: %w", err)
	}

	toVer, err := agent.ParseVersion(req.ToVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid to_version: %w", err)
	}

	changelog, err := s.catalog.GetChangelog(ctx, req.AgentID, fromVer, toVer)
	if err != nil {
		return &GetChangelogResponse{
			Changelog: "",
		}, nil
	}

	return &GetChangelogResponse{
		Changelog: changelog,
	}, nil
}

// GetStatus returns the service status.
func (s *Server) GetStatus(ctx context.Context) (*StatusResponse, error) {
	s.agentsMu.RLock()
	agentCount := len(s.agents)
	updatesAvailable := 0
	for _, inst := range s.agents {
		if inst.HasUpdate() {
			updatesAvailable++
		}
	}
	s.agentsMu.RUnlock()

	return &StatusResponse{
		Running:            true,
		UptimeSeconds:      int64(time.Since(s.startTime).Seconds()),
		AgentCount:         agentCount,
		UpdatesAvailable:   updatesAvailable,
		LastCatalogRefresh: s.lastRefresh,
		LastUpdateCheck:    s.lastCheck,
		Version:            "dev",
	}, nil
}

// Subscribe subscribes to agent events.
func (s *Server) Subscribe() <-chan *AgentEvent {
	ch := make(chan *AgentEvent, 100)
	s.subMu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.subMu.Unlock()
	return ch
}

// Unsubscribe unsubscribes from agent events.
func (s *Server) Unsubscribe(ch <-chan *AgentEvent) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	for i, sub := range s.subscribers {
		if sub == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			close(sub)
			break
		}
	}
}

// publishEvent publishes an event to all subscribers.
//
//nolint:unused // Reserved for streaming events implementation
func (s *Server) publishEvent(event *AgentEvent) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

// refreshAgents refreshes the agent list.
func (s *Server) refreshAgents(ctx context.Context) error {
	// Get agent definitions from catalog
	agentDefs, err := s.catalog.GetAgentsForPlatform(ctx, string(s.platform.ID()))
	if err != nil {
		agentDefs = nil
	}

	// Detect agents
	agents, err := s.detector.DetectAll(ctx, agentDefs)
	if err != nil {
		return err
	}

	s.agentsMu.Lock()
	s.agents = agents
	s.lastRefresh = time.Now()
	s.agentsMu.Unlock()

	return nil
}

// matchesFilter checks if an installation matches a filter.
func (s *Server) matchesFilter(inst *agent.Installation, filter *AgentFilter) bool {
	if filter == nil {
		return true
	}

	// Check agent IDs
	if len(filter.AgentIDs) > 0 {
		found := false
		for _, id := range filter.AgentIDs {
			if inst.AgentID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check methods
	if len(filter.Methods) > 0 {
		found := false
		for _, m := range filter.Methods {
			if string(inst.Method) == m {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check has_update
	if filter.HasUpdate != nil && *filter.HasUpdate != inst.HasUpdate() {
		return false
	}

	// Check is_global
	if filter.IsGlobal != nil && *filter.IsGlobal != inst.IsGlobal {
		return false
	}

	// Check query
	if filter.Query != "" {
		// Simple substring match
		if !containsIgnoreCase(inst.AgentID, filter.Query) &&
			!containsIgnoreCase(inst.AgentName, filter.Query) {
			return false
		}
	}

	return true
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(substr) == 0 ||
			(len(s) > 0 && containsIgnoreCaseHelper(s, substr)))
}

func containsIgnoreCaseHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldSlice(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFoldSlice(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
