// Package providers contains installation provider implementations.
package providers

import (
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
)

// Result represents the result of an install or update operation.
type Result struct {
	AgentID        string
	AgentName      string
	Method         agent.InstallMethod
	Version        agent.Version
	FromVersion    agent.Version // For updates
	InstallPath    string
	ExecutablePath string
	Duration       time.Duration
	Output         string
	WasUpdated     bool // For updates
}
