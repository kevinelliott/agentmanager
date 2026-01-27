// Package ipc provides inter-process communication between agentmgr CLI and helper.
package ipc

import (
	"encoding/json"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
)

// MessageType defines the type of IPC message.
type MessageType string

const (
	// Request types
	MessageTypeListAgents     MessageType = "list_agents"
	MessageTypeGetAgent       MessageType = "get_agent"
	MessageTypeInstallAgent   MessageType = "install_agent"
	MessageTypeUpdateAgent    MessageType = "update_agent"
	MessageTypeUninstallAgent MessageType = "uninstall_agent"
	MessageTypeRefreshCatalog MessageType = "refresh_catalog"
	MessageTypeCheckUpdates   MessageType = "check_updates"
	MessageTypeGetStatus      MessageType = "get_status"
	MessageTypeShutdown       MessageType = "shutdown"

	// Response types
	MessageTypeSuccess  MessageType = "success"
	MessageTypeError    MessageType = "error"
	MessageTypeProgress MessageType = "progress"

	// Notification types (helper -> CLI)
	MessageTypeUpdateAvailable MessageType = "update_available"
	MessageTypeAgentInstalled  MessageType = "agent_installed"
	MessageTypeAgentUpdated    MessageType = "agent_updated"
	MessageTypeAgentRemoved    MessageType = "agent_removed"
)

// Message represents an IPC message between CLI and helper.
type Message struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// NewMessage creates a new message with the given type and payload.
func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	var payloadBytes json.RawMessage
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	return &Message{
		ID:        generateMessageID(),
		Type:      msgType,
		Timestamp: time.Now(),
		Payload:   payloadBytes,
	}, nil
}

// DecodePayload decodes the message payload into the target struct.
func (m *Message) DecodePayload(target interface{}) error {
	if m.Payload == nil {
		return nil
	}
	return json.Unmarshal(m.Payload, target)
}

// Request payloads

// ListAgentsRequest is the payload for list_agents requests.
type ListAgentsRequest struct {
	Filter *agent.Filter `json:"filter,omitempty"`
}

// GetAgentRequest is the payload for get_agent requests.
type GetAgentRequest struct {
	Key string `json:"key"`
}

// InstallAgentRequest is the payload for install_agent requests.
type InstallAgentRequest struct {
	AgentID string              `json:"agent_id"`
	Method  agent.InstallMethod `json:"method"`
	Global  bool                `json:"global"`
}

// UpdateAgentRequest is the payload for update_agent requests.
type UpdateAgentRequest struct {
	Key string `json:"key"`
}

// UninstallAgentRequest is the payload for uninstall_agent requests.
type UninstallAgentRequest struct {
	Key string `json:"key"`
}

// Response payloads

// ListAgentsResponse is the payload for list_agents responses.
type ListAgentsResponse struct {
	Agents []agent.Installation `json:"agents"`
	Total  int                  `json:"total"`
}

// GetAgentResponse is the payload for get_agent responses.
type GetAgentResponse struct {
	Agent *agent.Installation `json:"agent,omitempty"`
}

// InstallAgentResponse is the payload for install_agent responses.
type InstallAgentResponse struct {
	Installation *agent.Installation `json:"installation"`
	Success      bool                `json:"success"`
	Message      string              `json:"message,omitempty"`
}

// UpdateAgentResponse is the payload for update_agent responses.
type UpdateAgentResponse struct {
	Installation *agent.Installation `json:"installation"`
	FromVersion  string              `json:"from_version"`
	ToVersion    string              `json:"to_version"`
	Success      bool                `json:"success"`
	Message      string              `json:"message,omitempty"`
}

// StatusResponse is the payload for get_status responses.
type StatusResponse struct {
	Running            bool      `json:"running"`
	PID                int       `json:"pid"`
	Uptime             int64     `json:"uptime_seconds"`
	AgentCount         int       `json:"agent_count"`
	UpdatesAvailable   int       `json:"updates_available"`
	LastCatalogRefresh time.Time `json:"last_catalog_refresh"`
	LastUpdateCheck    time.Time `json:"last_update_check"`
}

// ErrorResponse is the payload for error responses.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ProgressResponse is the payload for progress updates.
type ProgressResponse struct {
	Operation string  `json:"operation"`
	Progress  float64 `json:"progress"` // 0.0 to 1.0
	Message   string  `json:"message,omitempty"`
}

// Notification payloads

// UpdateAvailableNotification is sent when an update is detected.
type UpdateAvailableNotification struct {
	AgentID     string `json:"agent_id"`
	AgentName   string `json:"agent_name"`
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	Changelog   string `json:"changelog,omitempty"`
}

// generateMessageID creates a unique message ID.
func generateMessageID() string {
	return time.Now().Format("20060102150405.000000000")
}
