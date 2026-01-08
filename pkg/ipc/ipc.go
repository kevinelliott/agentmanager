package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// Common errors
var (
	ErrNotConnected   = errors.New("not connected to server")
	ErrServerClosed   = errors.New("server is closed")
	ErrTimeout        = errors.New("operation timed out")
	ErrInvalidMessage = errors.New("invalid message format")
)

// Handler processes incoming IPC messages.
type Handler interface {
	HandleMessage(ctx context.Context, msg *Message) (*Message, error)
}

// HandlerFunc is a function adapter for Handler.
type HandlerFunc func(ctx context.Context, msg *Message) (*Message, error)

// HandleMessage implements Handler.
func (f HandlerFunc) HandleMessage(ctx context.Context, msg *Message) (*Message, error) {
	return f(ctx, msg)
}

// Server represents an IPC server.
type Server interface {
	// Start begins listening for connections.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the server.
	Stop(ctx context.Context) error

	// SetHandler sets the message handler.
	SetHandler(handler Handler)

	// IsRunning returns true if the server is running.
	IsRunning() bool

	// Address returns the server's address (socket path or pipe name).
	Address() string
}

// Client represents an IPC client.
type Client interface {
	// Connect establishes a connection to the server.
	Connect(ctx context.Context) error

	// Disconnect closes the connection.
	Disconnect() error

	// Send sends a message and waits for a response.
	Send(ctx context.Context, msg *Message) (*Message, error)

	// SendAsync sends a message without waiting for a response.
	SendAsync(msg *Message) error

	// Subscribe registers a callback for notifications.
	Subscribe(callback func(*Message))

	// IsConnected returns true if connected to the server.
	IsConnected() bool
}

// connection wraps a net.Conn with message framing.
type connection struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
	mu      sync.Mutex
}

// newConnection creates a new connection wrapper.
func newConnection(conn net.Conn) *connection {
	return &connection{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
	}
}

// Send sends a message over the connection.
func (c *connection) Send(msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.encoder.Encode(msg)
}

// Receive receives a message from the connection.
func (c *connection) Receive() (*Message, error) {
	var msg Message
	if err := c.decoder.Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// Close closes the connection.
func (c *connection) Close() error {
	return c.conn.Close()
}

// SetDeadline sets read and write deadlines.
func (c *connection) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// unixServer implements Server using Unix sockets.
type unixServer struct {
	socketPath string
	listener   net.Listener
	handler    Handler
	running    bool
	mu         sync.RWMutex
	conns      map[*connection]bool
	connsMu    sync.Mutex
	done       chan struct{}
}

// NewUnixServer creates a new Unix socket server.
func NewUnixServer(socketPath string) Server {
	return &unixServer{
		socketPath: socketPath,
		conns:      make(map[*connection]bool),
		done:       make(chan struct{}),
	}
}

// Start begins listening for connections.
func (s *unixServer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("server already running")
	}

	// Remove existing socket file
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		s.mu.Unlock()
		return err
	}

	s.listener = listener
	s.running = true
	s.done = make(chan struct{})
	s.mu.Unlock()

	go s.acceptLoop(ctx)
	return nil
}

// acceptLoop accepts incoming connections.
func (s *unixServer) acceptLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}

		c := newConnection(conn)
		s.connsMu.Lock()
		s.conns[c] = true
		s.connsMu.Unlock()

		go s.handleConnection(ctx, c)
	}
}

// handleConnection processes messages from a single connection.
func (s *unixServer) handleConnection(ctx context.Context, conn *connection) {
	defer func() {
		conn.Close()
		s.connsMu.Lock()
		delete(s.conns, conn)
		s.connsMu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		default:
		}

		msg, err := conn.Receive()
		if err != nil {
			if err == io.EOF || errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}

		s.mu.RLock()
		handler := s.handler
		s.mu.RUnlock()

		if handler != nil {
			resp, err := handler.HandleMessage(ctx, msg)
			if err != nil {
				errMsg, _ := NewMessage(MessageTypeError, ErrorResponse{
					Code:    "handler_error",
					Message: err.Error(),
				})
				conn.Send(errMsg)
				continue
			}

			if resp != nil {
				conn.Send(resp)
			}
		}
	}
}

// Stop gracefully shuts down the server.
func (s *unixServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	close(s.done)
	s.mu.Unlock()

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Close all connections
	s.connsMu.Lock()
	for conn := range s.conns {
		conn.Close()
	}
	s.conns = make(map[*connection]bool)
	s.connsMu.Unlock()

	return nil
}

// SetHandler sets the message handler.
func (s *unixServer) SetHandler(handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// IsRunning returns true if the server is running.
func (s *unixServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Address returns the server's socket path.
func (s *unixServer) Address() string {
	return s.socketPath
}

// unixClient implements Client using Unix sockets.
type unixClient struct {
	socketPath  string
	conn        *connection
	connected   bool
	mu          sync.RWMutex
	subscribers []func(*Message)
	subMu       sync.RWMutex
}

// NewUnixClient creates a new Unix socket client.
func NewUnixClient(socketPath string) Client {
	return &unixClient{
		socketPath:  socketPath,
		subscribers: make([]func(*Message), 0),
	}
}

// Connect establishes a connection to the server.
func (c *unixClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return err
	}

	c.conn = newConnection(conn)
	c.connected = true

	// Start notification listener
	go c.listenForNotifications(ctx)

	return nil
}

// listenForNotifications listens for server-pushed notifications.
func (c *unixClient) listenForNotifications(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.mu.RLock()
		if !c.connected || c.conn == nil {
			c.mu.RUnlock()
			return
		}
		conn := c.conn
		c.mu.RUnlock()

		msg, err := conn.Receive()
		if err != nil {
			if err == io.EOF || errors.Is(err, net.ErrClosed) {
				c.mu.Lock()
				c.connected = false
				c.mu.Unlock()
				return
			}
			continue
		}

		// Dispatch to subscribers
		c.subMu.RLock()
		for _, sub := range c.subscribers {
			go sub(msg)
		}
		c.subMu.RUnlock()
	}
}

// Disconnect closes the connection.
func (c *unixClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Send sends a message and waits for a response.
func (c *unixClient) Send(ctx context.Context, msg *Message) (*Message, error) {
	c.mu.RLock()
	if !c.connected || c.conn == nil {
		c.mu.RUnlock()
		return nil, ErrNotConnected
	}
	conn := c.conn
	c.mu.RUnlock()

	// Set deadline from context
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
		defer conn.SetDeadline(time.Time{})
	}

	if err := conn.Send(msg); err != nil {
		return nil, err
	}

	return conn.Receive()
}

// SendAsync sends a message without waiting for a response.
func (c *unixClient) SendAsync(msg *Message) error {
	c.mu.RLock()
	if !c.connected || c.conn == nil {
		c.mu.RUnlock()
		return ErrNotConnected
	}
	conn := c.conn
	c.mu.RUnlock()

	return conn.Send(msg)
}

// Subscribe registers a callback for notifications.
func (c *unixClient) Subscribe(callback func(*Message)) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	c.subscribers = append(c.subscribers, callback)
}

// IsConnected returns true if connected to the server.
func (c *unixClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}
