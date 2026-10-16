package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RefreshLocker coordinates inventory and registry refreshes across processes.
// Locks must be released by the caller. The OS also releases them on exit.
type RefreshLocker interface {
	AcquireRefreshLock(context.Context, string) (func(), error)
}

// AcquireRefreshLock waits for a named lock without holding a database connection.
func (s *SQLiteStore) AcquireRefreshLock(ctx context.Context, key string) (func(), error) {
	dir := s.dbPath + ".locks"
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%x.lock", sha256.Sum256([]byte(key)))
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			return nil, err
		}
		locked, lockErr := tryRefreshLock(file)
		if lockErr != nil {
			_ = file.Close()
			return nil, lockErr
		}
		if locked {
			var once sync.Once
			return func() {
				once.Do(func() {
					if err := unlockRefresh(file); err != nil {
						slog.Debug("could not release refresh lock", "error", err)
					}
					_ = file.Close()
				})
			}, nil
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
