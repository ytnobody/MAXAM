package ws

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 64 * 1024
)

// Client represents a WebSocket client connection
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan *Message
	ID   string
}

// NewClient creates a new client
func NewClient(hub *Hub, conn *websocket.Conn, id string) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan *Message, 256),
		ID:   id,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log unexpected close errors if needed
			}
			break
		}

		message, err := ParseMessage(data)
		if err != nil {
			continue
		}

		// Set the sender to this client's ID
		message.From = c.ID
		if message.Timestamp.IsZero() {
			message.Timestamp = time.Now()
		}

		c.hub.Broadcast(message)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := message.ToJSON()
			if err != nil {
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// SendHistory sends the message history to this client
func (c *Client) SendHistory(messages []*Message) error {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	historyResp := NewHistoryResponse(messages)
	data, err := historyResp.ToJSON()
	if err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Send queues a message to be sent to this client
func (c *Client) Send(message *Message) {
	select {
	case c.send <- message:
	default:
		// Buffer full, drop message
	}
}
