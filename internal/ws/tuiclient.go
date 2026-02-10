package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// TUIClient represents a WebSocket client for the TUI
type TUIClient struct {
	serverURL     string
	clientID      string
	conn          *websocket.Conn
	mu            sync.RWMutex
	connected     atomic.Bool
	incoming      chan *Message
	outgoing      chan *Message
	done          chan struct{}
	reconnectWait time.Duration
	maxReconnect  int
	onMessage     func(*Message)
	onConnect     func()
	onDisconnect  func(error)
}

// TUIClientConfig holds configuration for the TUI client
type TUIClientConfig struct {
	ServerURL     string
	ClientID      string
	ReconnectWait time.Duration
	MaxReconnect  int
}

// DefaultTUIClientConfig returns default configuration
func DefaultTUIClientConfig(port int, clientID string) TUIClientConfig {
	return TUIClientConfig{
		ServerURL:     fmt.Sprintf("ws://localhost:%d/ws", port),
		ClientID:      clientID,
		ReconnectWait: 2 * time.Second,
		MaxReconnect:  10,
	}
}

// NewTUIClient creates a new TUI WebSocket client
func NewTUIClient(cfg TUIClientConfig) *TUIClient {
	return &TUIClient{
		serverURL:     cfg.ServerURL,
		clientID:      cfg.ClientID,
		incoming:      make(chan *Message, 256),
		outgoing:      make(chan *Message, 256),
		done:          make(chan struct{}),
		reconnectWait: cfg.ReconnectWait,
		maxReconnect:  cfg.MaxReconnect,
	}
}

// SetMessageHandler sets the callback for incoming messages
func (c *TUIClient) SetMessageHandler(handler func(*Message)) {
	c.onMessage = handler
}

// SetConnectHandler sets the callback for successful connection
func (c *TUIClient) SetConnectHandler(handler func()) {
	c.onConnect = handler
}

// SetDisconnectHandler sets the callback for disconnection
func (c *TUIClient) SetDisconnectHandler(handler func(error)) {
	c.onDisconnect = handler
}

// Connect establishes a WebSocket connection to the server
func (c *TUIClient) Connect(ctx context.Context) error {
	u, err := url.Parse(c.serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	// Add client ID and history request to query
	q := u.Query()
	q.Set("id", c.clientID)
	q.Set("history", "true")
	u.RawQuery = q.Encode()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	c.connected.Store(true)

	if c.onConnect != nil {
		c.onConnect()
	}

	// Start read/write pumps
	go c.readPump()
	go c.writePump()

	return nil
}

// ConnectWithRetry connects with automatic retry on failure
func (c *TUIClient) ConnectWithRetry(ctx context.Context) error {
	var lastErr error
	for i := 0; i < c.maxReconnect; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := c.Connect(ctx); err != nil {
			lastErr = err
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.reconnectWait):
				continue
			}
		}
		return nil
	}
	return fmt.Errorf("max reconnect attempts reached: %w", lastErr)
}

// Disconnect closes the WebSocket connection
func (c *TUIClient) Disconnect() error {
	if !c.connected.Load() {
		return nil
	}

	c.connected.Store(false)

	// Close done channel (only once)
	select {
	case <-c.done:
		// Already closed
	default:
		close(c.done)
	}

	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()

	var err error
	if conn != nil {
		// Send close message
		deadline := time.Now().Add(time.Second)
		conn.SetWriteDeadline(deadline)
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		err = conn.Close()
	}

	// Call disconnect handler
	if c.onDisconnect != nil {
		c.onDisconnect(nil)
	}

	return err
}

// IsConnected returns true if the client is connected
func (c *TUIClient) IsConnected() bool {
	return c.connected.Load()
}

// Send sends a message to the server
func (c *TUIClient) Send(msg *Message) error {
	if !c.connected.Load() {
		return fmt.Errorf("not connected")
	}

	select {
	case c.outgoing <- msg:
		return nil
	default:
		return fmt.Errorf("send buffer full")
	}
}

// SendChat sends a chat message
func (c *TUIClient) SendChat(to, content string) error {
	msg := NewChatMessage(c.clientID, to, content)
	return c.Send(msg)
}

// Incoming returns the channel for incoming messages
func (c *TUIClient) Incoming() <-chan *Message {
	return c.incoming
}

// readPump reads messages from the WebSocket connection
func (c *TUIClient) readPump() {
	defer func() {
		c.handleDisconnect(nil)
	}()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return
	}

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.handleDisconnect(err)
			}
			return
		}

		// Try to parse as history response first
		if histResp, err := tryParseHistoryResponse(data); err == nil {
			for _, msg := range histResp.Messages {
				c.deliverMessage(msg)
			}
			continue
		}

		// Parse as regular message
		msg, err := ParseMessage(data)
		if err != nil {
			continue
		}

		c.deliverMessage(msg)
	}
}

// writePump writes messages to the WebSocket connection
func (c *TUIClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return
	}

	for {
		select {
		case <-c.done:
			return

		case msg := <-c.outgoing:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			data, err := msg.ToJSON()
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.handleDisconnect(err)
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.handleDisconnect(err)
				return
			}
		}
	}
}

// deliverMessage delivers a message to handlers
func (c *TUIClient) deliverMessage(msg *Message) {
	// Deliver to handler if set
	if c.onMessage != nil {
		c.onMessage(msg)
	}

	// Also send to channel for polling
	select {
	case c.incoming <- msg:
	default:
		// Buffer full, drop oldest
		select {
		case <-c.incoming:
		default:
		}
		select {
		case c.incoming <- msg:
		default:
		}
	}
}

// handleDisconnect handles disconnection
func (c *TUIClient) handleDisconnect(err error) {
	if !c.connected.Load() {
		return
	}

	c.connected.Store(false)

	// Close done channel (only once)
	select {
	case <-c.done:
		// Already closed
	default:
		close(c.done)
	}

	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	if c.onDisconnect != nil {
		c.onDisconnect(err)
	}
}

// tryParseHistoryResponse attempts to parse data as a HistoryResponse
func tryParseHistoryResponse(data []byte) (*HistoryResponse, error) {
	var resp HistoryResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Type != TypeHistory {
		return nil, fmt.Errorf("not a history response")
	}
	return &resp, nil
}
