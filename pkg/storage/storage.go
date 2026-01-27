// Package storage provides persistent storage for agent data.
package storage

import (
	"context"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

// Store defines the storage interface for agent data.
type Store interface {
	// Initialize sets up the database and runs migrations.
	Initialize(ctx context.Context) error

	// Close closes the storage connection.
	Close() error

	// Installation operations
	SaveInstallation(ctx context.Context, inst *agent.Installation) error
	GetInstallation(ctx context.Context, key string) (*agent.Installation, error)
	ListInstallations(ctx context.Context, filter *agent.Filter) ([]*agent.Installation, error)
	DeleteInstallation(ctx context.Context, key string) error

	// Update history operations
	SaveUpdateEvent(ctx context.Context, event *UpdateEvent) error
	GetUpdateHistory(ctx context.Context, agentID string, limit int) ([]*UpdateEvent, error)

	// Catalog cache operations
	SaveCatalogCache(ctx context.Context, data []byte, etag string) error
	GetCatalogCache(ctx context.Context) ([]byte, string, time.Time, error)

	// Detection cache operations
	SaveDetectionCache(ctx context.Context, installations []*agent.Installation) error
	GetDetectionCache(ctx context.Context) ([]*agent.Installation, time.Time, error)
	ClearDetectionCache(ctx context.Context) error
	GetDetectionCacheTime(ctx context.Context) (time.Time, error)
	SetLastUpdateCheckTime(ctx context.Context, t time.Time) error
	GetLastUpdateCheckTime(ctx context.Context) (time.Time, error)

	// Settings operations
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	DeleteSetting(ctx context.Context, key string) error
}

// UpdateEvent represents a recorded update event.
type UpdateEvent struct {
	ID            int64
	AgentID       string
	AgentName     string
	InstallMethod string
	FromVersion   string
	ToVersion     string
	Status        UpdateStatus
	ErrorMessage  string
	StartedAt     time.Time
	CompletedAt   *time.Time
}

// UpdateStatus represents the status of an update.
type UpdateStatus string

const (
	UpdateStatusPending   UpdateStatus = "pending"
	UpdateStatusRunning   UpdateStatus = "running"
	UpdateStatusCompleted UpdateStatus = "completed"
	UpdateStatusFailed    UpdateStatus = "failed"
	UpdateStatusCancelled UpdateStatus = "cancelled"
)

// InstallationRecord represents a stored installation record.
type InstallationRecord struct {
	Key              string
	AgentID          string
	AgentName        string
	InstallMethod    string
	InstalledVersion string
	LatestVersion    string
	ExecutablePath   string
	InstallPath      string
	FirstDetectedAt  time.Time
	LastCheckedAt    time.Time
	LastUpdatedAt    *time.Time
	Metadata         map[string]string
}

// ToInstallation converts an InstallationRecord to an agent.Installation.
func (r *InstallationRecord) ToInstallation() *agent.Installation {
	var latestVer *agent.Version
	if r.LatestVersion != "" {
		v, err := agent.ParseVersion(r.LatestVersion)
		if err == nil {
			latestVer = &v
		}
	}

	var installedVer agent.Version
	if r.InstalledVersion != "" {
		if v, err := agent.ParseVersion(r.InstalledVersion); err == nil {
			installedVer = v
		}
	}

	return &agent.Installation{
		AgentID:          r.AgentID,
		AgentName:        r.AgentName,
		Method:           agent.InstallMethod(r.InstallMethod),
		InstalledVersion: installedVer,
		LatestVersion:    latestVer,
		ExecutablePath:   r.ExecutablePath,
		InstallPath:      r.InstallPath,
		DetectedAt:       r.FirstDetectedAt,
		LastChecked:      r.LastCheckedAt,
		Metadata:         r.Metadata,
	}
}

// FromInstallation creates an InstallationRecord from an agent.Installation.
func FromInstallation(inst *agent.Installation) *InstallationRecord {
	var latestVer string
	if inst.LatestVersion != nil {
		latestVer = inst.LatestVersion.String()
	}

	return &InstallationRecord{
		Key:              inst.Key(),
		AgentID:          inst.AgentID,
		AgentName:        inst.AgentName,
		InstallMethod:    string(inst.Method),
		InstalledVersion: inst.InstalledVersion.String(),
		LatestVersion:    latestVer,
		ExecutablePath:   inst.ExecutablePath,
		InstallPath:      inst.InstallPath,
		FirstDetectedAt:  inst.DetectedAt,
		LastCheckedAt:    inst.LastChecked,
		Metadata:         inst.Metadata,
	}
}
