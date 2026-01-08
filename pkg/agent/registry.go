package agent

import (
	"context"
	"strings"
)

// Registry manages the collection of detected agent installations.
type Registry interface {
	// List returns all detected agent installations.
	List(ctx context.Context) ([]Installation, error)

	// Get returns a specific installation by unique key.
	Get(ctx context.Context, key string) (*Installation, error)

	// GetByAgent returns all installations of a specific agent.
	GetByAgent(ctx context.Context, agentID string) ([]Installation, error)

	// GetByMethod returns all installations using a specific method.
	GetByMethod(ctx context.Context, method InstallMethod) ([]Installation, error)

	// Register adds or updates an installation.
	Register(ctx context.Context, installation Installation) error

	// Unregister removes an installation.
	Unregister(ctx context.Context, key string) error

	// Refresh re-detects all agents.
	Refresh(ctx context.Context) error

	// Count returns the number of registered installations.
	Count(ctx context.Context) (int, error)

	// CountWithUpdates returns the number of installations with available updates.
	CountWithUpdates(ctx context.Context) (int, error)
}

// RegistryOptions configures the registry behavior.
type RegistryOptions struct {
	// AutoRefresh enables automatic periodic refresh of agent detection.
	AutoRefresh bool

	// RefreshInterval specifies how often to refresh when AutoRefresh is enabled.
	RefreshInterval int // in seconds

	// IncludeHidden includes hidden agents in listings.
	IncludeHidden bool

	// PlatformFilter limits detection to specific platforms.
	PlatformFilter []string
}

// DefaultRegistryOptions returns the default registry options.
func DefaultRegistryOptions() RegistryOptions {
	return RegistryOptions{
		AutoRefresh:     false,
		RefreshInterval: 3600, // 1 hour
		IncludeHidden:   false,
		PlatformFilter:  nil,
	}
}

// RegistryEvent represents an event from the registry.
type RegistryEvent struct {
	Type         RegistryEventType `json:"type"`
	Installation *Installation     `json:"installation,omitempty"`
	Error        error             `json:"error,omitempty"`
}

// RegistryEventType defines types of registry events.
type RegistryEventType string

const (
	RegistryEventAdded     RegistryEventType = "added"
	RegistryEventUpdated   RegistryEventType = "updated"
	RegistryEventRemoved   RegistryEventType = "removed"
	RegistryEventRefreshed RegistryEventType = "refreshed"
	RegistryEventError     RegistryEventType = "error"
)

// RegistryObserver receives registry events.
type RegistryObserver interface {
	OnRegistryEvent(event RegistryEvent)
}

// ObservableRegistry extends Registry with event observation capabilities.
type ObservableRegistry interface {
	Registry

	// Subscribe adds an observer to receive events.
	Subscribe(observer RegistryObserver)

	// Unsubscribe removes an observer.
	Unsubscribe(observer RegistryObserver)
}

// Filter defines criteria for filtering installations.
type Filter struct {
	// AgentID limits results to a specific agent (singular convenience).
	AgentID string

	// AgentIDs limits results to specific agents (multiple).
	AgentIDs []string

	// Method limits results to a specific installation method (singular convenience).
	Method InstallMethod

	// Methods limits results to specific installation methods (multiple).
	Methods []InstallMethod

	// HasUpdate filters to only installations with updates available.
	HasUpdate *bool

	// IsGlobal filters to only global or local installations.
	IsGlobal *bool

	// Query performs a text search across agent names and IDs.
	Query string
}

// Matches returns true if the installation matches this filter.
func (f Filter) Matches(inst Installation) bool {
	// Check singular agent ID filter
	if f.AgentID != "" && inst.AgentID != f.AgentID {
		return false
	}

	// Check plural agent IDs filter
	if len(f.AgentIDs) > 0 {
		found := false
		for _, id := range f.AgentIDs {
			if inst.AgentID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check singular method filter
	if f.Method != "" && inst.Method != f.Method {
		return false
	}

	// Check plural methods filter
	if len(f.Methods) > 0 {
		found := false
		for _, m := range f.Methods {
			if inst.Method == m {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check update filter
	if f.HasUpdate != nil {
		if *f.HasUpdate != inst.HasUpdate() {
			return false
		}
	}

	// Check global filter
	if f.IsGlobal != nil {
		if *f.IsGlobal != inst.IsGlobal {
			return false
		}
	}

	// Check query filter (case-insensitive)
	if f.Query != "" {
		query := strings.ToLower(f.Query)
		if !strings.Contains(strings.ToLower(inst.AgentID), query) &&
			!strings.Contains(strings.ToLower(inst.AgentName), query) {
			return false
		}
	}

	return true
}

// SortField defines fields for sorting installations.
type SortField string

const (
	SortByName      SortField = "name"
	SortByMethod    SortField = "method"
	SortByVersion   SortField = "version"
	SortByUpdatedAt SortField = "updated_at"
	SortByStatus    SortField = "status"
)

// SortOrder defines the sort direction.
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// ListOptions configures listing behavior.
type ListOptions struct {
	Filter    *Filter
	SortBy    SortField
	SortOrder SortOrder
	Limit     int
	Offset    int
}

// DefaultListOptions returns default listing options.
func DefaultListOptions() ListOptions {
	return ListOptions{
		Filter:    nil,
		SortBy:    SortByName,
		SortOrder: SortAsc,
		Limit:     0, // No limit
		Offset:    0,
	}
}
