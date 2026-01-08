//go:build windows

package ipc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

// namedPipeServer implements Server using Windows named pipes.
type namedPipeServer struct {
	pipeName string
	listener net.Listener
	handler  Handler
	running  bool
	mu       sync.RWMutex
	conns    map[*connection]bool
	connsMu  sync.Mutex
	done     chan struct{}
}

// NewNamedPipeServer creates a new Windows named pipe server.
func NewNamedPipeServer(pipeName string) Server {
	return &namedPipeServer{
		pipeName: pipeName,
		conns:    make(map[*connection]bool),
		done:     make(chan struct{}),
	}
}

// Start begins listening for connections.
func (s *namedPipeServer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("server already running")
	}

	config := &winio.PipeConfig{
		SecurityDescriptor: "",
		MessageMode:        false,
		InputBufferSize:    65536,
		OutputBufferSize:   65536,
	}

	listener, err := winio.ListenPipe(s.pipeName, config)
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
func (s *namedPipeServer) acceptLoop(ctx context.Context) {
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
			if errors.Is(err, net.ErrClosed) || errors.Is(err, winio.ErrPipeListenerClosed) {
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
func (s *namedPipeServer) handleConnection(ctx context.Context, conn *connection) {
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
func (s *namedPipeServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	close(s.done)
	s.mu.Unlock()

	if s.listener != nil {
		s.listener.Close()
	}

	s.connsMu.Lock()
	for conn := range s.conns {
		conn.Close()
	}
	s.conns = make(map[*connection]bool)
	s.connsMu.Unlock()

	return nil
}

// SetHandler sets the message handler.
func (s *namedPipeServer) SetHandler(handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// IsRunning returns true if the server is running.
func (s *namedPipeServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Address returns the server's pipe name.
func (s *namedPipeServer) Address() string {
	return s.pipeName
}

// namedPipeClient implements Client using Windows named pipes.
type namedPipeClient struct {
	pipeName    string
	conn        *connection
	connected   bool
	mu          sync.RWMutex
	subscribers []func(*Message)
	subMu       sync.RWMutex
}

// NewNamedPipeClient creates a new Windows named pipe client.
func NewNamedPipeClient(pipeName string) Client {
	return &namedPipeClient{
		pipeName:    pipeName,
		subscribers: make([]func(*Message), 0),
	}
}

// Connect establishes a connection to the server.
func (c *namedPipeClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	conn, err := winio.DialPipeContext(ctx, c.pipeName)
	if err != nil {
		return err
	}

	c.conn = newConnection(conn)
	c.connected = true

	go c.listenForNotifications(ctx)

	return nil
}

// listenForNotifications listens for server-pushed notifications.
func (c *namedPipeClient) listenForNotifications(ctx context.Context) {
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

		c.subMu.RLock()
		for _, sub := range c.subscribers {
			go sub(msg)
		}
		c.subMu.RUnlock()
	}
}

// Disconnect closes the connection.
func (c *namedPipeClient) Disconnect() error {
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
func (c *namedPipeClient) Send(ctx context.Context, msg *Message) (*Message, error) {
	c.mu.RLock()
	if !c.connected || c.conn == nil {
		c.mu.RUnlock()
		return nil, ErrNotConnected
	}
	conn := c.conn
	c.mu.RUnlock()

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
func (c *namedPipeClient) SendAsync(msg *Message) error {
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
func (c *namedPipeClient) Subscribe(callback func(*Message)) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	c.subscribers = append(c.subscribers, callback)
}

// IsConnected returns true if connected to the server.
func (c *namedPipeClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}
