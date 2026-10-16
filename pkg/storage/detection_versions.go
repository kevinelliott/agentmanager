package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

// DetectionCacheUpdater persists version metadata without extending detection freshness.
// It only updates installations still present in the current snapshot.
type DetectionCacheUpdater interface {
	UpdateDetectionVersions(context.Context, []*agent.Installation) error
}

// UpdateDetectionVersions merges latest versions without changing cached_at.
func (s *SQLiteStore) UpdateDetectionVersions(ctx context.Context, installations []*agent.Installation) error {
	versions := make(map[string]string, len(installations))
	for _, inst := range installations {
		if inst.LatestVersion != nil {
			versions[inst.Key()] = inst.LatestVersion.String()
		}
	}
	if len(versions) == 0 {
		return nil
	}
	for attempt := 0; attempt < 3; attempt++ {
		data, _, err := s.readDetectionSnapshot(ctx)
		if err != nil || data == nil {
			return err
		}
		var records []*InstallationRecord
		if err := json.Unmarshal(data, &records); err != nil {
			return err
		}
		changed := false
		for _, record := range records {
			if version, ok := versions[record.Key]; ok && record.LatestVersion != version {
				record.LatestVersion = version
				changed = true
			}
		}
		if !changed {
			return nil
		}
		updated, err := json.Marshal(records)
		if err != nil {
			return err
		}
		result, err := s.db.ExecContext(ctx, "UPDATE detection_cache SET data = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1 AND data = ?", updated, data)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
	}
	return fmt.Errorf("detection cache changed during version update")
}
