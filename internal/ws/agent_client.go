// Package ws provides WebSocket functionality for MAXAM
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

// AgentClient interface defines the WebSocket client for agents
type AgentClient interface {
	Connect(ctx context.Context) error
	Close() error
	Send(msg *Message) error
	Receive(ctx context.Context) (*Message, error)
	IsConnected() bool
}

// AgentClientConfig holds configuration for the agent WebSocket client
type AgentClientConfig struct {
	// ServerURL is the WebSocket server URL (e.g., "ws://localhost:8080/ws")
	ServerURL string

	// AgentID identifies this agent
	AgentID string

	// ReconnectInterval is the time to wait between reconnection attempts
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

// DefaultAgentClientConfig returns a default configuration
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

// agentClientImpl is the concrete implementation of AgentClient
type agentClientImpl struct {
	config    *AgentClientConfig
	conn      *websocket.Conn
	mu        sync.RWMutex
	connected bool
	done      chan struct{}
	sendCh    chan *Message
	recvCh    chan *Message
	errCh     chan error
}

// NewAgentClient creates a new agent WebSocket client
func NewAgentClient(config *AgentClientConfig) (AgentClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &agentClientImpl{
		config: config,
		done:   make(chan struct{}),
		sendCh: make(chan *Message, 256),
		recvCh: make(chan *Message, 256),
		errCh:  make(chan error, 1),
	}, nil
}

// Connect establishes a WebSocket connection to the server
func (c *agentClientImpl) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Add agent ID as query parameter
	u, _ := url.Parse(c.config.ServerURL)
	q := u.Query()
	q.Set("id", c.config.AgentID)
	u.RawQuery = q.Encode()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket server: %w", err)
	}

	c.conn = conn
	c.connected = true
	c.done = make(chan struct{})

	// Start read/write pumps
	go c.readPump()
	go c.writePump()
	go c.pingPump()

	return nil
}

// ConnectWithReconnect connects and automatically reconnects on disconnection
func (c *agentClientImpl) ConnectWithReconnect(ctx context.Context) error {
	err := c.Connect(ctx)
	if err != nil {
		return err
	}

	go c.reconnectLoop(ctx)
	return nil
}

func (c *agentClientImpl) reconnectLoop(ctx context.Context) {
	attempts := 0
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-c.errCh:
			if err == nil {
				continue
			}

			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()

			attempts++
			if c.config.MaxReconnectAttempts > 0 && attempts >= c.config.MaxReconnectAttempts {
				return
			}

			time.Sleep(c.config.ReconnectInterval)

			if err := c.Connect(ctx); err != nil {
				continue
			}
			attempts = 0
		}
	}
}

// Close closes the WebSocket connection
func (c *agentClientImpl) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	close(c.done)

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Send sends a message through the WebSocket connection
func (c *agentClientImpl) Send(msg *Message) error {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return errors.New("not connected")
	}

	// Ensure From is set to this agent's ID
	msg.From = c.config.AgentID
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	select {
	case c.sendCh <- msg:
		return nil
	default:
		return errors.New("send buffer full")
	}
}

// Receive receives a message from the WebSocket connection
func (c *agentClientImpl) Receive(ctx context.Context) (*Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-c.recvCh:
		return msg, nil
	}
}

// IsConnected returns whether the client is currently connected
func (c *agentClientImpl) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *agentClientImpl) readPump() {
	defer func() {
		c.errCh <- errors.New("read pump stopped")
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
		return nil
	})

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.errCh <- err
			}
			return
		}

		msg, err := ParseMessage(data)
		if err != nil {
			continue
		}

		select {
		case c.recvCh <- msg:
		default:
			// Receive buffer full, drop message
		}
	}
}

func (c *agentClientImpl) writePump() {
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.sendCh:
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))

			data, err := msg.ToJSON()
			if err != nil {
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.errCh <- err
				return
			}
		}
	}
}

func (c *agentClientImpl) pingPump() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.errCh <- err
				return
			}
		}
	}
}
