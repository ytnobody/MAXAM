// Package ws provides WebSocket server and client functionality for MAXAM chat
package ws

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AgentClient interface for WebSocket client used by agents
type AgentClient interface {
	// Connect establishes a WebSocket connection to the server
	Connect(ctx context.Context) error
	// ConnectWithReconnect connects and automatically reconnects on disconnection
	ConnectWithReconnect(ctx context.Context) error
	// Send sends a message to the server
	Send(msg *Message) error
	// Receive returns a channel for receiving messages
	Receive() <-chan *Message
	// Close closes the connection
	Close() error
	// IsConnected returns true if the client is currently connected
	IsConnected() bool
}

// AgentClientConfig holds configuration for the agent client
type AgentClientConfig struct {
	// ServerURL is the WebSocket server URL (e.g., "ws://localhost:8080/ws")
	ServerURL string
	// AgentID is the unique identifier for this agent
	AgentID string
	// ReconnectInterval is the time to wait before reconnecting
	ReconnectInterval time.Duration
	// MaxReconnectAttempts is the maximum number of reconnection attempts (0 = unlimited)
	MaxReconnectAttempts int
	// PingInterval is the interval for sending ping messages
	PingInterval time.Duration
	// WriteTimeout is the timeout for write operations
	WriteTimeout time.Duration
	// ReadTimeout is the timeout for read operations
	ReadTimeout time.Duration
}

// DefaultAgentClientConfig returns a config with sensible defaults
func DefaultAgentClientConfig(serverURL, agentID string) *AgentClientConfig {
	return &AgentClientConfig{
		ServerURL:            serverURL,
		AgentID:              agentID,
		ReconnectInterval:    5 * time.Second,
		MaxReconnectAttempts: 0, // unlimited
		PingInterval:         30 * time.Second,
		WriteTimeout:         10 * time.Second,
		ReadTimeout:          60 * time.Second,
	}
}

// Validate checks if the configuration is valid
func (c *AgentClientConfig) Validate() error {
	if c.ServerURL == "" {
		return errors.New("server URL is required")
	}
	if c.AgentID == "" {
		return errors.New("agent ID is required")
	}

	// Validate URL format
	u, err := url.Parse(c.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf("server URL must use ws:// or wss:// scheme, got %s", u.Scheme)
	}

	return nil
}

// agentClient implements AgentClient
type agentClient struct {
	config    *AgentClientConfig
	conn      *websocket.Conn
	mu        sync.RWMutex
	receive   chan *Message
	done      chan struct{}
	connected bool
	closed    bool
}

// NewAgentClient creates a new agent WebSocket client
func NewAgentClient(config *AgentClientConfig) (AgentClient, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &agentClient{
		config:  config,
		receive: make(chan *Message, 256),
		done:    make(chan struct{}),
	}, nil
}

// Connect establishes a WebSocket connection to the server
func (c *agentClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("client is closed")
	}
	c.mu.Unlock()

	// Build URL with agent ID
	u, err := url.Parse(c.config.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	q := u.Query()
	q.Set("id", c.config.AgentID)
	u.RawQuery = q.Encode()

	// Dial with context
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	// Start read/write pumps
	go c.readPump(ctx)
	go c.pingPump(ctx)

	return nil
}

// Send sends a message to the server
func (c *agentClient) Send(msg *Message) error {
	c.mu.RLock()
	if !c.connected || c.conn == nil {
		c.mu.RUnlock()
		return errors.New("not connected")
	}
	conn := c.conn
	c.mu.RUnlock()

	// Set sender to agent ID
	msg.From = c.config.AgentID
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// Receive returns a channel for receiving messages
func (c *agentClient) Receive() <-chan *Message {
	return c.receive
}

// Close closes the connection
func (c *agentClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	c.connected = false
	close(c.done)

	if c.conn != nil {
		// Send close message
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		return c.conn.Close()
	}

	return nil
}

// IsConnected returns true if the client is currently connected
func (c *agentClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// readPump reads messages from the WebSocket connection
func (c *agentClient) readPump(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		c.connected = false
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	}()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Could log here if needed
			}
			return
		}

		msg, err := ParseMessage(data)
		if err != nil {
			continue
		}

		select {
		case c.receive <- msg:
		default:
			// Buffer full, drop message
		}
	}
}

// pingPump sends periodic ping messages to keep the connection alive
func (c *agentClient) pingPump(ctx context.Context) {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			connected := c.connected
			c.mu.RUnlock()

			if !connected || conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ConnectWithReconnect connects to the server and automatically reconnects on disconnection
func (c *agentClient) ConnectWithReconnect(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("client is closed")
	}
	c.mu.Unlock()

	// Initial connection
	if err := c.Connect(ctx); err != nil {
		// Start reconnection loop in background
		go c.reconnectLoop(ctx)
		return err
	}

	// Monitor connection and reconnect if needed
	go c.monitorConnection(ctx)

	return nil
}

// reconnectLoop attempts to reconnect to the server
func (c *agentClient) reconnectLoop(ctx context.Context) {
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
		}

		// Check if we've exceeded max attempts
		if c.config.MaxReconnectAttempts > 0 && attempts >= c.config.MaxReconnectAttempts {
			return
		}

		// Wait before reconnecting
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-time.After(c.config.ReconnectInterval):
		}

		attempts++

		// Try to connect
		if err := c.Connect(ctx); err != nil {
			continue
		}

		// Connection successful, reset attempts and start monitoring
		attempts = 0
		go c.monitorConnection(ctx)
		return
	}
}

// monitorConnection monitors the connection and triggers reconnection if disconnected
func (c *agentClient) monitorConnection(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			if !c.IsConnected() {
				// Connection lost, start reconnection
				go c.reconnectLoop(ctx)
				return
			}
		}
	}
}
