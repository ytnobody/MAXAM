package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAgentClientConfig(t *testing.T) {
	t.Run("DefaultAgentClientConfig", func(t *testing.T) {
		config := DefaultAgentClientConfig("ws://localhost:8080/ws", "agent-1")

		if config.ServerURL != "ws://localhost:8080/ws" {
			t.Errorf("expected server URL ws://localhost:8080/ws, got %s", config.ServerURL)
		}
		if config.AgentID != "agent-1" {
			t.Errorf("expected agent ID agent-1, got %s", config.AgentID)
		}
		if config.ReconnectInterval != 5*time.Second {
			t.Errorf("expected reconnect interval 5s, got %v", config.ReconnectInterval)
		}
		if config.MaxReconnectAttempts != 0 {
			t.Errorf("expected max reconnect attempts 0 (unlimited), got %d", config.MaxReconnectAttempts)
		}
	})
}

func TestNewAgentClient(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := DefaultAgentClientConfig("ws://localhost:8080/ws", "agent-1")
		client, err := NewAgentClient(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("expected client, got nil")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		_, err := NewAgentClient(nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("empty server URL", func(t *testing.T) {
		config := &AgentClientConfig{
			AgentID: "agent-1",
		}
		_, err := NewAgentClient(config)
		if err == nil {
			t.Fatal("expected error for empty server URL")
		}
	})

	t.Run("empty agent ID", func(t *testing.T) {
		config := &AgentClientConfig{
			ServerURL: "ws://localhost:8080/ws",
		}
		_, err := NewAgentClient(config)
		if err == nil {
			t.Fatal("expected error for empty agent ID")
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		config := &AgentClientConfig{
			ServerURL: "://invalid",
			AgentID:   "agent-1",
		}
		_, err := NewAgentClient(config)
		if err == nil {
			t.Fatal("expected error for invalid URL")
		}
	})
}

func TestAgentClientConnection(t *testing.T) {
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
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	t.Run("Connect successfully", func(t *testing.T) {
		config := DefaultAgentClientConfig(wsURL, "test-agent")
		client, err := NewAgentClient(config)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err != nil {
			t.Fatalf("failed to connect: %v", err)
		}

		if !client.IsConnected() {
			t.Error("expected client to be connected")
		}
	})

	t.Run("Connect to non-existent server", func(t *testing.T) {
		config := DefaultAgentClientConfig("ws://localhost:59999/ws", "test-agent")
		client, err := NewAgentClient(config)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err == nil {
			t.Fatal("expected error connecting to non-existent server")
		}
	})

	t.Run("Send message", func(t *testing.T) {
		config := DefaultAgentClientConfig(wsURL, "sender-agent")
		client, err := NewAgentClient(config)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err != nil {
			t.Fatalf("failed to connect: %v", err)
		}

		msg := NewChatMessage("", "all", "hello from agent")
		if err := client.Send(msg); err != nil {
			t.Fatalf("failed to send message: %v", err)
		}
	})

	t.Run("Send message when not connected", func(t *testing.T) {
		config := DefaultAgentClientConfig(wsURL, "test-agent")
		client, err := NewAgentClient(config)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		msg := NewChatMessage("", "all", "hello")
		if err := client.Send(msg); err == nil {
			t.Fatal("expected error sending when not connected")
		}
	})

	t.Run("Receive messages", func(t *testing.T) {
		config := DefaultAgentClientConfig(wsURL, "receiver-agent")
		client, err := NewAgentClient(config)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err != nil {
			t.Fatalf("failed to connect: %v", err)
		}

		// Should receive join message
		select {
		case msg := <-client.Receive():
			if msg.Type != TypeJoin {
				t.Errorf("expected join message, got %s", msg.Type)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for message")
		}
	})

	t.Run("Close connection", func(t *testing.T) {
		config := DefaultAgentClientConfig(wsURL, "close-test-agent")
		client, err := NewAgentClient(config)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err != nil {
			t.Fatalf("failed to connect: %v", err)
		}

		if err := client.Close(); err != nil {
			t.Fatalf("failed to close: %v", err)
		}

		// Wait for connection state to update
		time.Sleep(100 * time.Millisecond)

		if client.IsConnected() {
			t.Error("expected client to be disconnected after close")
		}
	})

	t.Run("Double close is safe", func(t *testing.T) {
		config := DefaultAgentClientConfig(wsURL, "double-close-agent")
		client, err := NewAgentClient(config)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err != nil {
			t.Fatalf("failed to connect: %v", err)
		}

		if err := client.Close(); err != nil {
			t.Fatalf("first close failed: %v", err)
		}

		// Second close should not panic or error
		if err := client.Close(); err != nil {
			t.Fatalf("second close failed: %v", err)
		}
	})

	t.Run("Connect after close returns error", func(t *testing.T) {
		config := DefaultAgentClientConfig(wsURL, "reconnect-test-agent")
		client, err := NewAgentClient(config)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		if err := client.Close(); err != nil {
			t.Fatalf("close failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err == nil {
			t.Fatal("expected error connecting after close")
		}
	})
}

func TestAgentClientMessageFromSetsAgentID(t *testing.T) {
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

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	config := DefaultAgentClientConfig(wsURL, "my-agent-id")
	client, err := NewAgentClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Send a message with empty From field
	msg := &Message{
		Type:    TypeChat,
		Content: "test message",
	}
	if err := client.Send(msg); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Wait for message to be received
	time.Sleep(100 * time.Millisecond)

	// The From field should have been set to agent ID
	if msg.From != "my-agent-id" {
		t.Errorf("expected From to be 'my-agent-id', got '%s'", msg.From)
	}
}
