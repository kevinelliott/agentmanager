package ipc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
)

func TestNewMessage(t *testing.T) {
	tests := []struct {
		name    string
		msgType MessageType
		payload interface{}
		wantErr bool
	}{
		{
			name:    "simple message without payload",
			msgType: MessageTypeGetStatus,
			payload: nil,
			wantErr: false,
		},
		{
			name:    "message with struct payload",
			msgType: MessageTypeListAgents,
			payload: ListAgentsRequest{Filter: nil},
			wantErr: false,
		},
		{
			name:    "message with complex payload",
			msgType: MessageTypeInstallAgent,
			payload: InstallAgentRequest{
				AgentID: "claude-code",
				Method:  agent.InstallMethodNPM,
				Global:  true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewMessage(tt.msgType, tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if msg == nil {
				t.Error("NewMessage() returned nil message")
				return
			}
			if msg.Type != tt.msgType {
				t.Errorf("Type = %v, want %v", msg.Type, tt.msgType)
			}
			if msg.ID == "" {
				t.Error("ID should not be empty")
			}
			if msg.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}
		})
	}
}

func TestMessageDecodePayload(t *testing.T) {
	t.Run("decode struct payload", func(t *testing.T) {
		original := InstallAgentRequest{
			AgentID: "claude-code",
			Method:  agent.InstallMethodNPM,
			Global:  true,
		}

		msg, err := NewMessage(MessageTypeInstallAgent, original)
		if err != nil {
			t.Fatal(err)
		}

		var decoded InstallAgentRequest
		if err := msg.DecodePayload(&decoded); err != nil {
			t.Fatalf("DecodePayload() error = %v", err)
		}

		if decoded.AgentID != original.AgentID {
			t.Errorf("AgentID = %q, want %q", decoded.AgentID, original.AgentID)
		}
		if decoded.Method != original.Method {
			t.Errorf("Method = %v, want %v", decoded.Method, original.Method)
		}
		if decoded.Global != original.Global {
			t.Errorf("Global = %v, want %v", decoded.Global, original.Global)
		}
	})

	t.Run("decode nil payload", func(t *testing.T) {
		msg, err := NewMessage(MessageTypeGetStatus, nil)
		if err != nil {
			t.Fatal(err)
		}

		var decoded StatusResponse
		if err := msg.DecodePayload(&decoded); err != nil {
			t.Errorf("DecodePayload(nil) error = %v", err)
		}
	})
}

func TestHandlerFunc(t *testing.T) {
	called := false
	handler := HandlerFunc(func(ctx context.Context, msg *Message) (*Message, error) {
		called = true
		return NewMessage(MessageTypeSuccess, nil)
	})

	msg, _ := NewMessage(MessageTypeGetStatus, nil)
	resp, err := handler.HandleMessage(context.Background(), msg)

	if err != nil {
		t.Errorf("HandleMessage() error = %v", err)
	}
	if !called {
		t.Error("Handler function was not called")
	}
	if resp == nil {
		t.Error("Response should not be nil")
	}
	if resp.Type != MessageTypeSuccess {
		t.Errorf("Response type = %v, want %v", resp.Type, MessageTypeSuccess)
	}
}

func TestUnixServerBasics(t *testing.T) {
	// Create temp socket path
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create server
	server := NewUnixServer(socketPath)

	// Set up handler
	server.SetHandler(HandlerFunc(func(ctx context.Context, msg *Message) (*Message, error) {
		return NewMessage(MessageTypeSuccess, StatusResponse{
			Running:    true,
			AgentCount: 5,
		})
	}))

	// Start server
	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Server.Start() error = %v", err)
	}
	defer server.Stop(context.Background())

	if !server.IsRunning() {
		t.Error("Server should be running")
	}

	if server.Address() != socketPath {
		t.Errorf("Address() = %q, want %q", server.Address(), socketPath)
	}

	// Verify socket was created
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Error("Socket file should exist")
	}
}

func TestUnixClientConnect(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create and start server
	server := NewUnixServer(socketPath)
	server.SetHandler(HandlerFunc(func(ctx context.Context, msg *Message) (*Message, error) {
		return NewMessage(MessageTypeSuccess, nil)
	}))

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Server.Start() error = %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	// Create client and connect
	client := NewUnixClient(socketPath)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Client.Connect() error = %v", err)
	}
	defer client.Disconnect()

	if !client.IsConnected() {
		t.Error("Client should be connected")
	}
}

func TestUnixServerAlreadyRunning(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	server := NewUnixServer(socketPath)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("First Start() error = %v", err)
	}
	defer server.Stop(context.Background())

	// Try to start again
	if err := server.Start(ctx); err == nil {
		t.Error("Second Start() should return error")
	}
}

func TestUnixServerStop(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	server := NewUnixServer(socketPath)
	ctx := context.Background()

	// Start and stop
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if server.IsRunning() {
		t.Error("Server should not be running after Stop()")
	}

	// Stop again should be idempotent
	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Second Stop() error = %v", err)
	}
}

func TestUnixClientNotConnected(t *testing.T) {
	client := NewUnixClient("/nonexistent/socket.sock")

	// Send without connecting
	msg, _ := NewMessage(MessageTypeGetStatus, nil)
	_, err := client.Send(context.Background(), msg)
	if err != ErrNotConnected {
		t.Errorf("Send() error = %v, want ErrNotConnected", err)
	}

	// SendAsync without connecting
	if err := client.SendAsync(msg); err != ErrNotConnected {
		t.Errorf("SendAsync() error = %v, want ErrNotConnected", err)
	}

	// IsConnected should return false
	if client.IsConnected() {
		t.Error("IsConnected() should return false")
	}
}

func TestUnixClientConnectFailed(t *testing.T) {
	client := NewUnixClient("/nonexistent/socket.sock")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := client.Connect(ctx)
	if err == nil {
		t.Error("Connect() should fail for nonexistent socket")
	}
}

func TestUnixClientDisconnect(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	server := NewUnixServer(socketPath)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	client := NewUnixClient(socketPath)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// Disconnect
	if err := client.Disconnect(); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}

	if client.IsConnected() {
		t.Error("IsConnected() should return false after Disconnect()")
	}

	// Disconnect again should be idempotent
	if err := client.Disconnect(); err != nil {
		t.Fatalf("Second Disconnect() error = %v", err)
	}
}

func TestUnixClientSubscribe(t *testing.T) {
	client := NewUnixClient("/test.sock").(*unixClient)

	notifications := make(chan *Message, 10)
	client.Subscribe(func(msg *Message) {
		notifications <- msg
	})

	if len(client.subscribers) != 1 {
		t.Errorf("Subscribers count = %d, want 1", len(client.subscribers))
	}

	// Subscribe another
	client.Subscribe(func(msg *Message) {})
	if len(client.subscribers) != 2 {
		t.Errorf("Subscribers count = %d, want 2", len(client.subscribers))
	}
}

func TestConnectionWrapper(t *testing.T) {
	// Test the connection wrapper with an in-process pipe
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	serverConn := newConnection(server)
	clientConn := newConnection(client)

	// Send message from client to server
	go func() {
		msg, _ := NewMessage(MessageTypeGetStatus, StatusResponse{Running: true})
		clientConn.Send(msg)
	}()

	// Receive on server
	msg, err := serverConn.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}

	if msg.Type != MessageTypeGetStatus {
		t.Errorf("Type = %v, want %v", msg.Type, MessageTypeGetStatus)
	}

	var status StatusResponse
	if err := msg.DecodePayload(&status); err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}

	if !status.Running {
		t.Error("Status.Running should be true")
	}
}

func TestMessageTypes(t *testing.T) {
	// Verify all message type constants are unique
	types := map[MessageType]bool{
		MessageTypeListAgents:      true,
		MessageTypeGetAgent:        true,
		MessageTypeInstallAgent:    true,
		MessageTypeUpdateAgent:     true,
		MessageTypeUninstallAgent:  true,
		MessageTypeRefreshCatalog:  true,
		MessageTypeCheckUpdates:    true,
		MessageTypeGetStatus:       true,
		MessageTypeShutdown:        true,
		MessageTypeSuccess:         true,
		MessageTypeError:           true,
		MessageTypeProgress:        true,
		MessageTypeUpdateAvailable: true,
		MessageTypeAgentInstalled:  true,
		MessageTypeAgentUpdated:    true,
		MessageTypeAgentRemoved:    true,
	}

	if len(types) != 16 {
		t.Errorf("Expected 16 unique message types, got %d", len(types))
	}
}

func TestRequestPayloads(t *testing.T) {
	t.Run("ListAgentsRequest", func(t *testing.T) {
		req := ListAgentsRequest{Filter: &agent.Filter{}}
		msg, err := NewMessage(MessageTypeListAgents, req)
		if err != nil {
			t.Fatal(err)
		}
		var decoded ListAgentsRequest
		if err := msg.DecodePayload(&decoded); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("GetAgentRequest", func(t *testing.T) {
		req := GetAgentRequest{Key: "claude-code:npm"}
		msg, err := NewMessage(MessageTypeGetAgent, req)
		if err != nil {
			t.Fatal(err)
		}
		var decoded GetAgentRequest
		if err := msg.DecodePayload(&decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Key != req.Key {
			t.Errorf("Key = %q, want %q", decoded.Key, req.Key)
		}
	})

	t.Run("UpdateAgentRequest", func(t *testing.T) {
		req := UpdateAgentRequest{Key: "claude-code:npm"}
		msg, err := NewMessage(MessageTypeUpdateAgent, req)
		if err != nil {
			t.Fatal(err)
		}
		var decoded UpdateAgentRequest
		if err := msg.DecodePayload(&decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Key != req.Key {
			t.Errorf("Key = %q, want %q", decoded.Key, req.Key)
		}
	})

	t.Run("UninstallAgentRequest", func(t *testing.T) {
		req := UninstallAgentRequest{Key: "claude-code:npm"}
		msg, err := NewMessage(MessageTypeUninstallAgent, req)
		if err != nil {
			t.Fatal(err)
		}
		var decoded UninstallAgentRequest
		if err := msg.DecodePayload(&decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Key != req.Key {
			t.Errorf("Key = %q, want %q", decoded.Key, req.Key)
		}
	})
}

func TestResponsePayloads(t *testing.T) {
	t.Run("ListAgentsResponse", func(t *testing.T) {
		resp := ListAgentsResponse{
			Agents: []agent.Installation{{AgentID: "test"}},
			Total:  1,
		}
		msg, err := NewMessage(MessageTypeSuccess, resp)
		if err != nil {
			t.Fatal(err)
		}
		var decoded ListAgentsResponse
		if err := msg.DecodePayload(&decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Total != resp.Total {
			t.Errorf("Total = %d, want %d", decoded.Total, resp.Total)
		}
	})

	t.Run("StatusResponse", func(t *testing.T) {
		resp := StatusResponse{
			Running:          true,
			Uptime:           3600,
			AgentCount:       5,
			UpdatesAvailable: 2,
		}
		msg, err := NewMessage(MessageTypeSuccess, resp)
		if err != nil {
			t.Fatal(err)
		}
		var decoded StatusResponse
		if err := msg.DecodePayload(&decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.AgentCount != resp.AgentCount {
			t.Errorf("AgentCount = %d, want %d", decoded.AgentCount, resp.AgentCount)
		}
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		resp := ErrorResponse{
			Code:    "test_error",
			Message: "Test error message",
			Details: "Some details",
		}
		msg, err := NewMessage(MessageTypeError, resp)
		if err != nil {
			t.Fatal(err)
		}
		var decoded ErrorResponse
		if err := msg.DecodePayload(&decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Code != resp.Code {
			t.Errorf("Code = %q, want %q", decoded.Code, resp.Code)
		}
	})

	t.Run("ProgressResponse", func(t *testing.T) {
		resp := ProgressResponse{
			Operation: "install",
			Progress:  0.5,
			Message:   "Installing...",
		}
		msg, err := NewMessage(MessageTypeProgress, resp)
		if err != nil {
			t.Fatal(err)
		}
		var decoded ProgressResponse
		if err := msg.DecodePayload(&decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Progress != resp.Progress {
			t.Errorf("Progress = %f, want %f", decoded.Progress, resp.Progress)
		}
	})
}

func TestUpdateAvailableNotification(t *testing.T) {
	notif := UpdateAvailableNotification{
		AgentID:     "claude-code",
		AgentName:   "Claude Code",
		FromVersion: "1.0.0",
		ToVersion:   "1.1.0",
		Changelog:   "Bug fixes",
	}

	msg, err := NewMessage(MessageTypeUpdateAvailable, notif)
	if err != nil {
		t.Fatal(err)
	}

	var decoded UpdateAvailableNotification
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.AgentID != notif.AgentID {
		t.Errorf("AgentID = %q, want %q", decoded.AgentID, notif.AgentID)
	}
	if decoded.FromVersion != notif.FromVersion {
		t.Errorf("FromVersion = %q, want %q", decoded.FromVersion, notif.FromVersion)
	}
	if decoded.ToVersion != notif.ToVersion {
		t.Errorf("ToVersion = %q, want %q", decoded.ToVersion, notif.ToVersion)
	}
}

func TestGenerateMessageID(t *testing.T) {
	id1 := generateMessageID()
	time.Sleep(time.Millisecond)
	id2 := generateMessageID()

	if id1 == "" {
		t.Error("ID should not be empty")
	}
	if id1 == id2 {
		t.Error("IDs should be unique")
	}
}

func TestSocketCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "cleanup_test.sock")

	server := NewUnixServer(socketPath)
	ctx := context.Background()

	// Start server
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Error("Socket file should exist after Start()")
	}

	// Stop server
	server.Stop(context.Background())

	// Server should not be running
	if server.IsRunning() {
		t.Error("Server should not be running after Stop()")
	}
}
