//go:build windows

package ipc

import (
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

// DefaultSocketPath returns the default IPC socket/pipe path for the current platform.
func DefaultSocketPath() string {
	plat := platform.Current()
	return plat.GetIPCSocketPath()
}

// NewServer creates a new IPC server appropriate for the current platform.
func NewServer(address string) Server {
	if address == "" {
		address = DefaultSocketPath()
	}
	return NewNamedPipeServer(address)
}

// NewClient creates a new IPC client appropriate for the current platform.
func NewClient(address string) Client {
	if address == "" {
		address = DefaultSocketPath()
	}
	return NewNamedPipeClient(address)
}
