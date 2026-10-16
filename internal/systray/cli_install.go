package systray

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func copyCLIForWindows(sourcePath, targetPath string) (err error) {
	if filepath.Base(targetPath) != "agentmgr.exe" {
		return fmt.Errorf("unexpected CLI installation filename")
	}
	dir := filepath.Dir(targetPath)
	// #nosec G703 -- Explicit user-requested installation directory in LOCALAPPDATA; only creates directories.
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	// #nosec G304 -- The source is the executable bundled alongside this helper.
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	// The fixed name is confined to the installation directory, including when
	// a pre-existing symlink attempts to redirect the write outside that root.
	return root.WriteFile("agentmgr.exe", data, 0600)
}
