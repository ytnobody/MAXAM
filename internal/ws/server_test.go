package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestMessage(t *testing.T) {
	t.Run("NewChatMessage", func(t *testing.T) {
		msg := NewChatMessage("alice", "bob", "hello")
		if msg.Type != TypeChat {
			t.Errorf("expected type %s, got %s", TypeChat, msg.Type)
		}
		if msg.From != "alice" {
			t.Errorf("expected from alice, got %s", msg.From)
		}
		if msg.To != "bob" {
			t.Errorf("expected to bob, got %s", msg.To)
		}
		if msg.Content != "hello" {
			t.Errorf("expected content hello, got %s", msg.Content)
		}
	})

	t.Run("NewSystemMessage", func(t *testing.T) {
		msg := NewSystemMessage("system alert")
		if msg.Type != TypeSystem {
			t.Errorf("expected type %s, got %s", TypeSystem, msg.Type)
		}
		if msg.From != "system" {
			t.Errorf("expected from system, got %s", msg.From)
		}
	})

	t.Run("JSON serialization", func(t *testing.T) {
		msg := NewChatMessage("alice", "bob", "hello")
		data, err := msg.ToJSON()
		if err != nil {
			t.Fatalf("failed to serialize: %v", err)
		}

		parsed, err := ParseMessage(data)
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}

		if parsed.Type != msg.Type {
			t.Errorf("type mismatch: expected %s, got %s", msg.Type, parsed.Type)
		}
		if parsed.From != msg.From {
			t.Errorf("from mismatch: expected %s, got %s", msg.From, parsed.From)
		}
		if parsed.Content != msg.Content {
			t.Errorf("content mismatch: expected %s, got %s", msg.Content, parsed.Content)
		}
	})
}

func TestHub(t *testing.T) {
	t.Run("ClientCount", func(t *testing.T) {
		hub := NewHub()
		go hub.Run()

		if hub.ClientCount() != 0 {
			t.Errorf("expected 0 clients, got %d", hub.ClientCount())
		}
	})

	t.Run("GetHistory empty", func(t *testing.T) {
		hub := NewHub()
		history := hub.GetHistory()
		if len(history) != 0 {
			t.Errorf("expected empty history, got %d messages", len(history))
		}
	})
}

func TestServer(t *testing.T) {
	t.Run("NewServer", func(t *testing.T) {
		server := NewServer(8080)
		if server.Port() != 8080 {
			t.Errorf("expected port 8080, got %d", server.Port())
		}
		if server.IsRunning() {
			t.Error("server should not be running initially")
		}
	})

	t.Run("Start and Stop", func(t *testing.T) {
		server := NewServer(0) // Use port 0 for random available port
		if err := server.Start(); err != nil {
			t.Fatalf("failed to start server: %v", err)
		}

		if !server.IsRunning() {
			t.Error("server should be running after start")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Stop(ctx); err != nil {
			t.Fatalf("failed to stop server: %v", err)
		}

		if server.IsRunning() {
			t.Error("server should not be running after stop")
		}
	})

	t.Run("Double start returns error", func(t *testing.T) {
		server := NewServer(0)
		if err := server.Start(); err != nil {
			t.Fatalf("failed to start server: %v", err)
		}
		defer server.Stop(context.Background())

		if err := server.Start(); err == nil {
			t.Error("expected error on double start")
		}
	})
}

func TestServerWebSocket(t *testing.T) {
	// Create test server
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		clientID := r.URL.Query().Get("id")
		if clientID == "" {
			clientID = "test-client"
		}

		client := NewClient(hub, conn, clientID)
		hub.Register(client)

		go client.WritePump()
		client.ReadPump()
	}))
	defer server.Close()

	// Convert http URL to ws URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?id=test"

	t.Run("Connect and receive join message", func(t *testing.T) {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Wait for join message
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message: %v", err)
		}

		msg, err := ParseMessage(data)
		if err != nil {
			t.Fatalf("failed to parse message: %v", err)
		}

		if msg.Type != TypeJoin {
			t.Errorf("expected join message, got %s", msg.Type)
		}
	})

	t.Run("Send and receive chat message", func(t *testing.T) {
		// Connect two clients
		conn1, _, err := websocket.DefaultDialer.Dial(wsURL+"1", nil)
		if err != nil {
			t.Fatalf("failed to connect client 1: %v", err)
		}
		defer conn1.Close()

		// Give time for connection to be established
		time.Sleep(100 * time.Millisecond)

		conn2, _, err := websocket.DefaultDialer.Dial(wsURL+"2", nil)
		if err != nil {
			t.Fatalf("failed to connect client 2: %v", err)
		}
		defer conn2.Close()

		// Give time for both connections to be established
		time.Sleep(100 * time.Millisecond)

		// Send a chat message from client 1
		chatMsg := NewChatMessage("", "", "hello world")
		data, _ := chatMsg.ToJSON()
		if err := conn1.WriteMessage(websocket.TextMessage, data); err != nil {
			t.Fatalf("failed to send message: %v", err)
		}

		// Read messages on client 2, looking for the chat message
		conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
		for {
			_, received, err := conn2.ReadMessage()
			if err != nil {
				t.Fatalf("failed to receive message: %v", err)
			}

			msg, err := ParseMessage(received)
			if err != nil {
				t.Fatalf("failed to parse received message: %v", err)
			}

			// Skip join/leave messages, look for chat
			if msg.Type == TypeChat {
				if msg.Content != "hello world" {
					t.Errorf("expected 'hello world', got '%s'", msg.Content)
				}
				break
			}
		}
	})
}

func TestHistoryResponse(t *testing.T) {
	messages := []*Message{
		NewChatMessage("alice", "bob", "hello"),
		NewChatMessage("bob", "alice", "hi"),
	}

	resp := NewHistoryResponse(messages)
	if resp.Type != TypeHistory {
		t.Errorf("expected type %s, got %s", TypeHistory, resp.Type)
	}
	if len(resp.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(resp.Messages))
	}

	data, err := resp.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize history response: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}
