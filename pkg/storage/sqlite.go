package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db     *sql.DB
	dbPath string
}

// NewSQLiteStore creates a new SQLite store at the given path.
func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	dbPath := filepath.Join(dataDir, "agentmgr.db")
	return &SQLiteStore{
		dbPath: dbPath,
	}, nil
}

// Initialize opens the database and runs migrations.
func (s *SQLiteStore) Initialize(ctx context.Context) error {
	// Ensure the data directory exists
	dir := filepath.Dir(s.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sql.Open("sqlite3", s.dbPath+"?_journal_mode=WAL&_foreign_keys=ON")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	s.db = db

	// Run migrations
	if err := s.migrate(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// migrate runs database migrations.
func (s *SQLiteStore) migrate(ctx context.Context) error {
	migrations := []string{
		// Schema version tracking
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Installations table
		`CREATE TABLE IF NOT EXISTS installations (
			key TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			agent_name TEXT NOT NULL,
			install_method TEXT NOT NULL,
			installed_version TEXT NOT NULL,
			latest_version TEXT,
			executable_path TEXT,
			install_path TEXT,
			first_detected_at TIMESTAMP NOT NULL,
			last_checked_at TIMESTAMP NOT NULL,
			last_updated_at TIMESTAMP,
			metadata TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Index on agent_id for faster lookups
		`CREATE INDEX IF NOT EXISTS idx_installations_agent_id ON installations(agent_id)`,

		// Index on install_method for filtering
		`CREATE INDEX IF NOT EXISTS idx_installations_install_method ON installations(install_method)`,

		// Update events table
		`CREATE TABLE IF NOT EXISTS update_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id TEXT NOT NULL,
			agent_name TEXT NOT NULL,
			install_method TEXT NOT NULL,
			from_version TEXT NOT NULL,
			to_version TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			error_message TEXT,
			started_at TIMESTAMP NOT NULL,
			completed_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Index on agent_id for update history lookups
		`CREATE INDEX IF NOT EXISTS idx_update_events_agent_id ON update_events(agent_id)`,

		// Catalog cache table
		`CREATE TABLE IF NOT EXISTS catalog_cache (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			data BLOB NOT NULL,
			etag TEXT,
			cached_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Settings table
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Detection cache table
		`CREATE TABLE IF NOT EXISTS detection_cache (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			data BLOB NOT NULL,
			cached_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, migration := range migrations {
		if _, err := s.db.ExecContext(ctx, migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// SaveInstallation saves or updates an installation record.
func (s *SQLiteStore) SaveInstallation(ctx context.Context, inst *agent.Installation) error {
	record := FromInstallation(inst)

	metadataJSON, err := json.Marshal(record.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO installations (
			key, agent_id, agent_name, install_method,
			installed_version, latest_version, executable_path, install_path,
			first_detected_at, last_checked_at, last_updated_at, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			installed_version = excluded.installed_version,
			latest_version = excluded.latest_version,
			executable_path = excluded.executable_path,
			install_path = excluded.install_path,
			last_checked_at = excluded.last_checked_at,
			last_updated_at = excluded.last_updated_at,
			metadata = excluded.metadata,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.ExecContext(ctx, query,
		record.Key, record.AgentID, record.AgentName, record.InstallMethod,
		record.InstalledVersion, record.LatestVersion, record.ExecutablePath, record.InstallPath,
		record.FirstDetectedAt, record.LastCheckedAt, record.LastUpdatedAt, string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to save installation: %w", err)
	}

	return nil
}

// GetInstallation retrieves an installation by key.
func (s *SQLiteStore) GetInstallation(ctx context.Context, key string) (*agent.Installation, error) {
	query := `
		SELECT key, agent_id, agent_name, install_method,
			installed_version, latest_version, executable_path, install_path,
			first_detected_at, last_checked_at, last_updated_at, metadata
		FROM installations
		WHERE key = ?
	`

	var record InstallationRecord
	var metadataJSON string
	var latestVersion sql.NullString
	var lastUpdatedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, key).Scan(
		&record.Key, &record.AgentID, &record.AgentName, &record.InstallMethod,
		&record.InstalledVersion, &latestVersion, &record.ExecutablePath, &record.InstallPath,
		&record.FirstDetectedAt, &record.LastCheckedAt, &lastUpdatedAt, &metadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get installation: %w", err)
	}

	if latestVersion.Valid {
		record.LatestVersion = latestVersion.String
	}
	if lastUpdatedAt.Valid {
		record.LastUpdatedAt = &lastUpdatedAt.Time
	}

	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &record.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return record.ToInstallation(), nil
}

// ListInstallations returns all installations matching the filter.
func (s *SQLiteStore) ListInstallations(ctx context.Context, filter *agent.Filter) ([]*agent.Installation, error) {
	query := `
		SELECT key, agent_id, agent_name, install_method,
			installed_version, latest_version, executable_path, install_path,
			first_detected_at, last_checked_at, last_updated_at, metadata
		FROM installations
		WHERE 1=1
	`
	var args []interface{}

	if filter != nil {
		if filter.AgentID != "" {
			query += " AND agent_id = ?"
			args = append(args, filter.AgentID)
		}
		if filter.Method != "" {
			query += " AND install_method = ?"
			args = append(args, string(filter.Method))
		}
	}

	query += " ORDER BY agent_name, install_method"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list installations: %w", err)
	}
	defer rows.Close()

	var installations []*agent.Installation
	for rows.Next() {
		var record InstallationRecord
		var metadataJSON string
		var latestVersion sql.NullString
		var lastUpdatedAt sql.NullTime

		err := rows.Scan(
			&record.Key, &record.AgentID, &record.AgentName, &record.InstallMethod,
			&record.InstalledVersion, &latestVersion, &record.ExecutablePath, &record.InstallPath,
			&record.FirstDetectedAt, &record.LastCheckedAt, &lastUpdatedAt, &metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan installation: %w", err)
		}

		if latestVersion.Valid {
			record.LatestVersion = latestVersion.String
		}
		if lastUpdatedAt.Valid {
			record.LastUpdatedAt = &lastUpdatedAt.Time
		}

		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &record.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		inst := record.ToInstallation()

		// Apply HasUpdate filter if specified
		if filter != nil && filter.HasUpdate != nil {
			if *filter.HasUpdate != inst.HasUpdate() {
				continue
			}
		}

		installations = append(installations, inst)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating installations: %w", err)
	}

	return installations, nil
}

// DeleteInstallation removes an installation record.
func (s *SQLiteStore) DeleteInstallation(ctx context.Context, key string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM installations WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("failed to delete installation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("installation not found: %s", key)
	}

	return nil
}

// SaveUpdateEvent records an update event.
func (s *SQLiteStore) SaveUpdateEvent(ctx context.Context, event *UpdateEvent) error {
	if event.ID == 0 {
		// Insert new event
		query := `
			INSERT INTO update_events (
				agent_id, agent_name, install_method, from_version, to_version,
				status, error_message, started_at, completed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		result, err := s.db.ExecContext(ctx, query,
			event.AgentID, event.AgentName, event.InstallMethod, event.FromVersion, event.ToVersion,
			event.Status, event.ErrorMessage, event.StartedAt, event.CompletedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to save update event: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert id: %w", err)
		}
		event.ID = id
	} else {
		// Update existing event
		query := `
			UPDATE update_events SET
				status = ?, error_message = ?, completed_at = ?
			WHERE id = ?
		`
		_, err := s.db.ExecContext(ctx, query,
			event.Status, event.ErrorMessage, event.CompletedAt, event.ID,
		)
		if err != nil {
			return fmt.Errorf("failed to update event: %w", err)
		}
	}

	return nil
}

// GetUpdateHistory retrieves update history for an agent.
func (s *SQLiteStore) GetUpdateHistory(ctx context.Context, agentID string, limit int) ([]*UpdateEvent, error) {
	query := `
		SELECT id, agent_id, agent_name, install_method, from_version, to_version,
			status, error_message, started_at, completed_at
		FROM update_events
		WHERE agent_id = ?
		ORDER BY started_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get update history: %w", err)
	}
	defer rows.Close()

	var events []*UpdateEvent
	for rows.Next() {
		var event UpdateEvent
		var completedAt sql.NullTime

		err := rows.Scan(
			&event.ID, &event.AgentID, &event.AgentName, &event.InstallMethod,
			&event.FromVersion, &event.ToVersion, &event.Status, &event.ErrorMessage,
			&event.StartedAt, &completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan update event: %w", err)
		}

		if completedAt.Valid {
			event.CompletedAt = &completedAt.Time
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating update events: %w", err)
	}

	return events, nil
}

// SaveCatalogCache stores the catalog cache.
func (s *SQLiteStore) SaveCatalogCache(ctx context.Context, data []byte, etag string) error {
	query := `
		INSERT INTO catalog_cache (id, data, etag, cached_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			data = excluded.data,
			etag = excluded.etag,
			cached_at = excluded.cached_at,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.ExecContext(ctx, query, data, etag, time.Now())
	if err != nil {
		return fmt.Errorf("failed to save catalog cache: %w", err)
	}

	return nil
}

// GetCatalogCache retrieves the cached catalog.
func (s *SQLiteStore) GetCatalogCache(ctx context.Context) ([]byte, string, time.Time, error) {
	query := "SELECT data, etag, cached_at FROM catalog_cache WHERE id = 1"

	var data []byte
	var etag sql.NullString
	var cachedAt time.Time

	err := s.db.QueryRowContext(ctx, query).Scan(&data, &etag, &cachedAt)
	if err == sql.ErrNoRows {
		return nil, "", time.Time{}, nil
	}
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("failed to get catalog cache: %w", err)
	}

	return data, etag.String, cachedAt, nil
}

// GetSetting retrieves a setting value.
func (s *SQLiteStore) GetSetting(ctx context.Context, key string) (string, error) {
	query := "SELECT value FROM settings WHERE key = ?"

	var value string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get setting: %w", err)
	}

	return value, nil
}

// SetSetting stores a setting value.
func (s *SQLiteStore) SetSetting(ctx context.Context, key, value string) error {
	query := `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.ExecContext(ctx, query, key, value)
	if err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}

	return nil
}

// DeleteSetting removes a setting.
func (s *SQLiteStore) DeleteSetting(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM settings WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("failed to delete setting: %w", err)
	}
	return nil
}

// SaveDetectionCache stores the detected agents cache.
func (s *SQLiteStore) SaveDetectionCache(ctx context.Context, installations []*agent.Installation) error {
	// Convert installations to records for JSON serialization
	records := make([]*InstallationRecord, 0, len(installations))
	for _, inst := range installations {
		records = append(records, FromInstallation(inst))
	}

	data, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("failed to marshal detection cache: %w", err)
	}

	query := `
		INSERT INTO detection_cache (id, data, cached_at)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			data = excluded.data,
			cached_at = excluded.cached_at,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.ExecContext(ctx, query, data, time.Now())
	if err != nil {
		return fmt.Errorf("failed to save detection cache: %w", err)
	}

	return nil
}

// GetDetectionCache retrieves the cached detected agents.
func (s *SQLiteStore) GetDetectionCache(ctx context.Context) ([]*agent.Installation, time.Time, error) {
	query := "SELECT data, cached_at FROM detection_cache WHERE id = 1"

	var data []byte
	var cachedAt time.Time

	err := s.db.QueryRowContext(ctx, query).Scan(&data, &cachedAt)
	if err == sql.ErrNoRows {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to get detection cache: %w", err)
	}

	var records []*InstallationRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to unmarshal detection cache: %w", err)
	}

	installations := make([]*agent.Installation, 0, len(records))
	for _, record := range records {
		installations = append(installations, record.ToInstallation())
	}

	return installations, cachedAt, nil
}

// ClearDetectionCache removes the detection cache.
func (s *SQLiteStore) ClearDetectionCache(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM detection_cache WHERE id = 1")
	if err != nil {
		return fmt.Errorf("failed to clear detection cache: %w", err)
	}
	return nil
}

// GetDetectionCacheTime returns when the detection cache was last updated.
func (s *SQLiteStore) GetDetectionCacheTime(ctx context.Context) (time.Time, error) {
	query := "SELECT cached_at FROM detection_cache WHERE id = 1"

	var cachedAt time.Time
	err := s.db.QueryRowContext(ctx, query).Scan(&cachedAt)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get detection cache time: %w", err)
	}

	return cachedAt, nil
}

// SetLastUpdateCheckTime stores when updates were last checked.
func (s *SQLiteStore) SetLastUpdateCheckTime(ctx context.Context, t time.Time) error {
	return s.SetSetting(ctx, "last_update_check_time", t.Format(time.RFC3339))
}

// GetLastUpdateCheckTime returns when updates were last checked.
func (s *SQLiteStore) GetLastUpdateCheckTime(ctx context.Context) (time.Time, error) {
	val, err := s.GetSetting(ctx, "last_update_check_time")
	if err != nil {
		return time.Time{}, err
	}
	if val == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, val)
}
