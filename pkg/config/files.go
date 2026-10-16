package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WritePrivateFile writes a configuration file with owner-only permissions,
// including when overwriting an older, more permissive file.
func WritePrivateFile(path string, data []byte) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".agentmgr-config-*")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := os.Remove(f.Name()); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			err = errors.Join(err, cleanupErr)
		}
	}()
	_, writeErr := f.Write(data)
	if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
		return fmt.Errorf("write private configuration: %w", err)
	}
	// Rename replaces any old permissions or symlink without exposing partial data.
	return os.Rename(f.Name(), path)
}
