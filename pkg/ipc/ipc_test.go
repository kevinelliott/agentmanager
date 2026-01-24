package ipc

import (
	"context"
	"errors"
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

// Additional tests for improved coverage

func TestNewMessageWithUnmarshalablePayload(t *testing.T) {
	// Channels cannot be marshaled to JSON
	ch := make(chan int)
	_, err := NewMessage(MessageTypeGetStatus, ch)
	if err == nil {
		t.Error("NewMessage() should fail for unmarshalable payload")
	}
}

func TestDecodePayloadWithInvalidJSON(t *testing.T) {
	msg := &Message{
		ID:      "test",
		Type:    MessageTypeGetStatus,
		Payload: []byte(`{invalid json}`),
	}

	var decoded StatusResponse
	err := msg.DecodePayload(&decoded)
	if err == nil {
		t.Error("DecodePayload() should fail for invalid JSON")
	}
}

func TestDecodePayloadTypeMismatch(t *testing.T) {
	original := StatusResponse{
		Running:    true,
		AgentCount: 5,
	}

	msg, err := NewMessage(MessageTypeGetStatus, original)
	if err != nil {
		t.Fatal(err)
	}

	// Decode into wrong type - this actually works in JSON since it maps fields
	var decoded InstallAgentRequest
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatalf("DecodePayload failed: %v", err)
	}
	// JSON is lenient, so we check field values instead
	if decoded.AgentID != "" {
		t.Error("AgentID should be empty for mismatched type")
	}
}

func TestMessagePayloadPreservation(t *testing.T) {
	req := InstallAgentRequest{
		AgentID: "test-agent",
		Method:  agent.InstallMethodNPM,
		Global:  true,
	}

	msg, err := NewMessage(MessageTypeInstallAgent, req)
	if err != nil {
		t.Fatal(err)
	}

	// Decode multiple times to verify payload is not consumed
	var decoded1, decoded2 InstallAgentRequest
	if err := msg.DecodePayload(&decoded1); err != nil {
		t.Fatal(err)
	}
	if err := msg.DecodePayload(&decoded2); err != nil {
		t.Fatal(err)
	}

	if decoded1.AgentID != decoded2.AgentID {
		t.Error("Multiple decodes should produce same result")
	}
}

func TestConnectionSendReceiveLargeMessage(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	serverConn := newConnection(server)
	clientConn := newConnection(client)

	// Create a large payload
	largeDetails := ""
	for i := 0; i < 10000; i++ {
		largeDetails += "x"
	}

	go func() {
		msg, _ := NewMessage(MessageTypeError, ErrorResponse{
			Code:    "large_error",
			Message: "Test message",
			Details: largeDetails,
		})
		clientConn.Send(msg)
	}()

	msg, err := serverConn.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}

	var decoded ErrorResponse
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Details) != 10000 {
		t.Errorf("Details length = %d, want 10000", len(decoded.Details))
	}
}

func TestConnectionClose(t *testing.T) {
	server, client := net.Pipe()
	serverConn := newConnection(server)
	clientConn := newConnection(client)

	// Close client side
	if err := clientConn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Server should get error on receive
	_, err := serverConn.Receive()
	if err == nil {
		t.Error("Receive() should fail after connection closed")
	}

	// Close server side (already closed from other end, but should be safe)
	serverConn.Close()
}

func TestConnectionSetDeadline(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := newConnection(client)

	// Set a deadline in the past
	pastDeadline := time.Now().Add(-1 * time.Second)
	if err := conn.SetDeadline(pastDeadline); err != nil {
		t.Fatalf("SetDeadline() error = %v", err)
	}

	// Should timeout on send
	msg, _ := NewMessage(MessageTypeGetStatus, nil)
	err := conn.Send(msg)
	if err == nil {
		t.Error("Send() should fail with past deadline")
	}
}

func TestUnixServerWithNoHandler(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	server := NewUnixServer(socketPath)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	// Connect client
	client := NewUnixClient(socketPath)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Disconnect()

	// Server with no handler should not crash when receiving message
	// Send async since no response will come
	msg, _ := NewMessage(MessageTypeGetStatus, nil)
	if err := client.SendAsync(msg); err != nil {
		t.Fatalf("SendAsync() error = %v", err)
	}

	// Give server time to process
	time.Sleep(50 * time.Millisecond)
}

func TestUnixServerHandlerError(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	server := NewUnixServer(socketPath)
	server.SetHandler(HandlerFunc(func(ctx context.Context, msg *Message) (*Message, error) {
		return nil, errors.New("handler error")
	}))

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
	defer client.Disconnect()

	// Send message and expect error response
	msg, _ := NewMessage(MessageTypeGetStatus, nil)
	resp, err := client.Send(ctx, msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if resp.Type != MessageTypeError {
		t.Errorf("Response type = %v, want %v", resp.Type, MessageTypeError)
	}

	var errResp ErrorResponse
	if err := resp.DecodePayload(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "handler_error" {
		t.Errorf("Error code = %q, want %q", errResp.Code, "handler_error")
	}
}

func TestUnixServerHandlerReturnsNilResponse(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "ipc_test_nil.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)

	server := NewUnixServer(socketPath)
	server.SetHandler(HandlerFunc(func(ctx context.Context, msg *Message) (*Message, error) {
		return nil, nil // Return nil response with no error
	}))

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
	defer client.Disconnect()

	// Send async since no response expected
	msg, _ := NewMessage(MessageTypeShutdown, nil)
	if err := client.SendAsync(msg); err != nil {
		t.Fatalf("SendAsync() error = %v", err)
	}

	// Give server time to process
	time.Sleep(50 * time.Millisecond)
}

func TestUnixClientAlreadyConnected(t *testing.T) {
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

	// First connect
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("First Connect() error = %v", err)
	}
	defer client.Disconnect()

	// Second connect should be a no-op (already connected)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Second Connect() error = %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should still be connected")
	}
}

func TestUnixClientSendWithTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	server := NewUnixServer(socketPath)
	// Handler that takes a long time
	server.SetHandler(HandlerFunc(func(ctx context.Context, msg *Message) (*Message, error) {
		time.Sleep(5 * time.Second)
		return NewMessage(MessageTypeSuccess, nil)
	}))

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
	defer client.Disconnect()

	// Send with short timeout
	ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	msg, _ := NewMessage(MessageTypeGetStatus, nil)
	_, err := client.Send(ctxTimeout, msg)
	if err == nil {
		t.Error("Send() should timeout")
	}
}

func TestStaleSocketCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "stale.sock")

	// Create a stale socket file (not a real socket)
	f, err := os.Create(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Server should clean up stale socket and start successfully
	server := NewUnixServer(socketPath)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() should succeed after cleaning stale socket: %v", err)
	}
	defer server.Stop(context.Background())

	if !server.IsRunning() {
		t.Error("Server should be running")
	}
}

func TestServerStopClosesConnections(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "ipc_test_stop.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)

	server := NewUnixServer(socketPath)
	server.SetHandler(HandlerFunc(func(ctx context.Context, msg *Message) (*Message, error) {
		return NewMessage(MessageTypeSuccess, nil)
	}))

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Connect multiple clients
	clients := make([]Client, 3)
	for i := 0; i < 3; i++ {
		clients[i] = NewUnixClient(socketPath)
		if err := clients[i].Connect(ctx); err != nil {
			t.Fatalf("Connect() error = %v", err)
		}
	}

	// Stop server
	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Give connections time to close
	time.Sleep(50 * time.Millisecond)

	// Cleanup clients
	for _, client := range clients {
		client.Disconnect()
	}
}

func TestAllNotificationTypes(t *testing.T) {
	tests := []struct {
		name    string
		msgType MessageType
		payload interface{}
	}{
		{
			name:    "AgentInstalled notification",
			msgType: MessageTypeAgentInstalled,
			payload: map[string]interface{}{
				"agent_id": "claude-code",
				"version":  "1.0.0",
			},
		},
		{
			name:    "AgentUpdated notification",
			msgType: MessageTypeAgentUpdated,
			payload: map[string]interface{}{
				"agent_id":     "claude-code",
				"from_version": "1.0.0",
				"to_version":   "1.1.0",
			},
		},
		{
			name:    "AgentRemoved notification",
			msgType: MessageTypeAgentRemoved,
			payload: map[string]interface{}{
				"agent_id": "claude-code",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewMessage(tt.msgType, tt.payload)
			if err != nil {
				t.Fatalf("NewMessage() error = %v", err)
			}
			if msg.Type != tt.msgType {
				t.Errorf("Type = %v, want %v", msg.Type, tt.msgType)
			}
		})
	}
}

func TestGetAgentResponse(t *testing.T) {
	resp := GetAgentResponse{
		Agent: &agent.Installation{
			AgentID: "test-agent",
			Method:  agent.InstallMethodNPM,
		},
	}

	msg, err := NewMessage(MessageTypeSuccess, resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded GetAgentResponse
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Agent == nil {
		t.Fatal("Agent should not be nil")
	}
	if decoded.Agent.AgentID != "test-agent" {
		t.Errorf("AgentID = %q, want %q", decoded.Agent.AgentID, "test-agent")
	}
}

func TestInstallAgentResponse(t *testing.T) {
	resp := InstallAgentResponse{
		Installation: &agent.Installation{
			AgentID: "claude-code",
			Method:  agent.InstallMethodBrew,
		},
		Success: true,
		Message: "Installation complete",
	}

	msg, err := NewMessage(MessageTypeSuccess, resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded InstallAgentResponse
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatal(err)
	}

	if !decoded.Success {
		t.Error("Success should be true")
	}
	if decoded.Message != "Installation complete" {
		t.Errorf("Message = %q, want %q", decoded.Message, "Installation complete")
	}
}

func TestUpdateAgentResponse(t *testing.T) {
	resp := UpdateAgentResponse{
		Installation: &agent.Installation{
			AgentID: "claude-code",
		},
		FromVersion: "1.0.0",
		ToVersion:   "2.0.0",
		Success:     true,
	}

	msg, err := NewMessage(MessageTypeSuccess, resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded UpdateAgentResponse
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.FromVersion != "1.0.0" {
		t.Errorf("FromVersion = %q, want %q", decoded.FromVersion, "1.0.0")
	}
	if decoded.ToVersion != "2.0.0" {
		t.Errorf("ToVersion = %q, want %q", decoded.ToVersion, "2.0.0")
	}
}

func TestStatusResponseAllFields(t *testing.T) {
	now := time.Now()
	resp := StatusResponse{
		Running:            true,
		PID:                12345,
		Uptime:             3600,
		AgentCount:         10,
		UpdatesAvailable:   3,
		LastCatalogRefresh: now,
		LastUpdateCheck:    now,
	}

	msg, err := NewMessage(MessageTypeSuccess, resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded StatusResponse
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.PID != 12345 {
		t.Errorf("PID = %d, want %d", decoded.PID, 12345)
	}
	if decoded.Uptime != 3600 {
		t.Errorf("Uptime = %d, want %d", decoded.Uptime, 3600)
	}
	if decoded.UpdatesAvailable != 3 {
		t.Errorf("UpdatesAvailable = %d, want %d", decoded.UpdatesAvailable, 3)
	}
}

func TestFactoryNewServerWithEmptyAddress(t *testing.T) {
	// NewServer with empty address should use default
	server := NewServer("")
	if server == nil {
		t.Fatal("NewServer() returned nil")
	}
	defer server.Stop(context.Background())

	// Address should be the default socket path
	if server.Address() == "" {
		t.Error("Server address should not be empty")
	}
}

func TestFactoryNewClientWithEmptyAddress(t *testing.T) {
	// NewClient with empty address should use default
	client := NewClient("")
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	// Should not be connected initially
	if client.IsConnected() {
		t.Error("Client should not be connected initially")
	}
}

func TestFactoryNewServerWithCustomAddress(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "custom.sock")

	server := NewServer(socketPath)
	if server == nil {
		t.Fatal("NewServer() returned nil")
	}

	if server.Address() != socketPath {
		t.Errorf("Address() = %q, want %q", server.Address(), socketPath)
	}
}

func TestFactoryNewClientWithCustomAddress(t *testing.T) {
	socketPath := "/custom/path/socket.sock"
	client := NewClient(socketPath)
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
}

func TestDefaultSocketPath(t *testing.T) {
	path := DefaultSocketPath()
	if path == "" {
		t.Error("DefaultSocketPath() should not return empty string")
	}
}

func TestConnectionMultipleSendReceive(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	serverConn := newConnection(server)
	clientConn := newConnection(client)

	// Send multiple messages in sequence
	go func() {
		for i := 0; i < 5; i++ {
			msg, _ := NewMessage(MessageTypeGetStatus, StatusResponse{
				Running:    true,
				AgentCount: i,
			})
			clientConn.Send(msg)
		}
	}()

	// Receive all messages
	for i := 0; i < 5; i++ {
		msg, err := serverConn.Receive()
		if err != nil {
			t.Fatalf("Receive() error = %v", err)
		}

		var status StatusResponse
		if err := msg.DecodePayload(&status); err != nil {
			t.Fatal(err)
		}

		if status.AgentCount != i {
			t.Errorf("AgentCount = %d, want %d", status.AgentCount, i)
		}
	}
}

func TestUnixClientMultipleSubscribers(t *testing.T) {
	client := NewUnixClient("/test.sock").(*unixClient)

	var received1, received2 int

	client.Subscribe(func(msg *Message) {
		received1++
	})
	client.Subscribe(func(msg *Message) {
		received2++
	})

	if len(client.subscribers) != 2 {
		t.Errorf("Subscribers count = %d, want 2", len(client.subscribers))
	}

	// Simulate calling subscribers (normally done in listenForNotifications)
	client.subMu.RLock()
	testMsg, _ := NewMessage(MessageTypeUpdateAvailable, nil)
	for _, sub := range client.subscribers {
		sub(testMsg)
	}
	client.subMu.RUnlock()

	if received1 != 1 {
		t.Errorf("Subscriber 1 received %d, want 1", received1)
	}
	if received2 != 1 {
		t.Errorf("Subscriber 2 received %d, want 1", received2)
	}
}

func TestMessageTypeStrings(t *testing.T) {
	types := []struct {
		msgType MessageType
		str     string
	}{
		{MessageTypeListAgents, "list_agents"},
		{MessageTypeGetAgent, "get_agent"},
		{MessageTypeInstallAgent, "install_agent"},
		{MessageTypeUpdateAgent, "update_agent"},
		{MessageTypeUninstallAgent, "uninstall_agent"},
		{MessageTypeRefreshCatalog, "refresh_catalog"},
		{MessageTypeCheckUpdates, "check_updates"},
		{MessageTypeGetStatus, "get_status"},
		{MessageTypeShutdown, "shutdown"},
		{MessageTypeSuccess, "success"},
		{MessageTypeError, "error"},
		{MessageTypeProgress, "progress"},
		{MessageTypeUpdateAvailable, "update_available"},
		{MessageTypeAgentInstalled, "agent_installed"},
		{MessageTypeAgentUpdated, "agent_updated"},
		{MessageTypeAgentRemoved, "agent_removed"},
	}

	for _, tt := range types {
		t.Run(string(tt.msgType), func(t *testing.T) {
			if string(tt.msgType) != tt.str {
				t.Errorf("MessageType = %q, want %q", string(tt.msgType), tt.str)
			}
		})
	}
}

func TestErrorVariables(t *testing.T) {
	// Verify error variables are defined correctly
	if ErrNotConnected.Error() != "not connected to server" {
		t.Errorf("ErrNotConnected = %q", ErrNotConnected.Error())
	}
	if ErrServerClosed.Error() != "server is closed" {
		t.Errorf("ErrServerClosed = %q", ErrServerClosed.Error())
	}
	if ErrTimeout.Error() != "operation timed out" {
		t.Errorf("ErrTimeout = %q", ErrTimeout.Error())
	}
	if ErrInvalidMessage.Error() != "invalid message format" {
		t.Errorf("ErrInvalidMessage = %q", ErrInvalidMessage.Error())
	}
}

func TestListenForNotificationsContextCanceled(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "ipc_test_listen.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)

	server := NewUnixServer(socketPath)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	client := NewUnixClient(socketPath).(*unixClient)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Disconnect()

	// Create a context we can cancel
	listenCtx, cancel := context.WithCancel(ctx)

	// Start listening
	done := make(chan struct{})
	go func() {
		client.listenForNotifications(listenCtx)
		close(done)
	}()

	// Cancel context
	cancel()

	// Should exit quickly
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("listenForNotifications did not exit after context cancel")
	}
}

func TestServerContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	server := NewUnixServer(socketPath)
	ctx, cancel := context.WithCancel(context.Background())

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Cancel context
	cancel()

	// Give accept loop time to notice
	time.Sleep(100 * time.Millisecond)

	// Stop server
	server.Stop(context.Background())
}

func TestListAgentsRequestWithFilter(t *testing.T) {
	filter := &agent.Filter{}
	req := ListAgentsRequest{Filter: filter}

	msg, err := NewMessage(MessageTypeListAgents, req)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ListAgentsRequest
	if err := msg.DecodePayload(&decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Filter == nil {
		t.Error("Filter should not be nil")
	}
}

func TestCleanupStaleSocketWithActiveServer(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "ipc_test_active.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)

	// Start first server
	server1 := NewUnixServer(socketPath)
	ctx := context.Background()

	if err := server1.Start(ctx); err != nil {
		t.Fatalf("First server Start() error = %v", err)
	}
	defer server1.Stop(context.Background())

	// Try to start second server on same socket - should fail
	server2 := NewUnixServer(socketPath)
	err := server2.Start(ctx)
	if err == nil {
		server2.Stop(context.Background())
		t.Error("Second server Start() should fail when another server is active")
	}
}

func TestListenForNotificationsNotConnected(t *testing.T) {
	client := NewUnixClient("/nonexistent.sock").(*unixClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// listenForNotifications should return immediately when not connected
	done := make(chan struct{})
	go func() {
		client.listenForNotifications(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Expected - exits quickly when not connected
	case <-time.After(1 * time.Second):
		t.Error("listenForNotifications should exit when not connected")
	}
}

func TestListenForNotificationsConnectionClosed(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "ipc_test_close.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)

	server := NewUnixServer(socketPath)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	client := NewUnixClient(socketPath).(*unixClient)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// Start listening
	listenCtx := context.Background()
	done := make(chan struct{})
	go func() {
		client.listenForNotifications(listenCtx)
		close(done)
	}()

	// Give listener time to start
	time.Sleep(50 * time.Millisecond)

	// Stop server to close connections
	server.Stop(context.Background())

	// listenForNotifications should exit after connection closes
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("listenForNotifications should exit after connection closes")
	}
}

func TestDisconnectWithNilConnection(t *testing.T) {
	client := NewUnixClient("/nonexistent.sock").(*unixClient)

	// Manually set connected to true but leave conn nil (edge case)
	client.mu.Lock()
	client.connected = true
	client.conn = nil
	client.mu.Unlock()

	// Disconnect should handle nil conn gracefully
	err := client.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() error = %v", err)
	}

	if client.IsConnected() {
		t.Error("Client should not be connected after Disconnect()")
	}
}

func TestSendWithNilConnection(t *testing.T) {
	client := NewUnixClient("/nonexistent.sock").(*unixClient)

	// Manually set connected to true but leave conn nil (edge case)
	client.mu.Lock()
	client.connected = true
	client.conn = nil
	client.mu.Unlock()

	msg, _ := NewMessage(MessageTypeGetStatus, nil)
	_, err := client.Send(context.Background(), msg)
	if err != ErrNotConnected {
		t.Errorf("Send() error = %v, want ErrNotConnected", err)
	}
}

func TestSendAsyncWithNilConnection(t *testing.T) {
	client := NewUnixClient("/nonexistent.sock").(*unixClient)

	// Manually set connected to true but leave conn nil (edge case)
	client.mu.Lock()
	client.connected = true
	client.conn = nil
	client.mu.Unlock()

	msg, _ := NewMessage(MessageTypeGetStatus, nil)
	err := client.SendAsync(msg)
	if err != ErrNotConnected {
		t.Errorf("SendAsync() error = %v, want ErrNotConnected", err)
	}
}

func TestServerAcceptLoopClosedListener(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "ipc_test_accept.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)

	server := NewUnixServer(socketPath)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Immediately stop the server
	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Accept loop should have exited cleanly
	if server.IsRunning() {
		t.Error("Server should not be running")
	}
}
