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

func TestAgentClientConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *AgentClientConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &AgentClientConfig{
				ServerURL: "ws://localhost:8080/ws",
				AgentID:   "agent-1",
			},
			wantErr: false,
		},
		{
			name: "valid config with wss",
			config: &AgentClientConfig{
				ServerURL: "wss://example.com/ws",
				AgentID:   "agent-1",
			},
			wantErr: false,
		},
		{
			name: "empty server URL",
			config: &AgentClientConfig{
				ServerURL: "",
				AgentID:   "agent-1",
			},
			wantErr: true,
			errMsg:  "server URL is required",
		},
		{
			name: "empty agent ID",
			config: &AgentClientConfig{
				ServerURL: "ws://localhost:8080/ws",
				AgentID:   "",
			},
			wantErr: true,
			errMsg:  "agent ID is required",
		},
		{
			name: "invalid URL scheme",
			config: &AgentClientConfig{
				ServerURL: "http://localhost:8080/ws",
				AgentID:   "agent-1",
			},
			wantErr: true,
			errMsg:  "must use ws:// or wss://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestNewAgentClient(t *testing.T) {
	tests := []struct {
		name    string
		config  *AgentClientConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultAgentClientConfig("ws://localhost:8080/ws", "agent-1"),
			wantErr: false,
		},
		{
			name: "invalid config",
			config: &AgentClientConfig{
				ServerURL: "",
				AgentID:   "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewAgentClient(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Error("expected client but got nil")
				}
			}
		})
	}
}

func TestDefaultAgentClientConfig(t *testing.T) {
	config := DefaultAgentClientConfig("ws://localhost:8080/ws", "agent-1")

	if config.ServerURL != "ws://localhost:8080/ws" {
		t.Errorf("expected ServerURL ws://localhost:8080/ws, got %s", config.ServerURL)
	}
	if config.AgentID != "agent-1" {
		t.Errorf("expected AgentID agent-1, got %s", config.AgentID)
	}
	if config.ReconnectInterval != 5*time.Second {
		t.Errorf("expected ReconnectInterval 5s, got %v", config.ReconnectInterval)
	}
	if config.MaxReconnectAttempts != 0 {
		t.Errorf("expected MaxReconnectAttempts 0, got %d", config.MaxReconnectAttempts)
	}
	if config.PingInterval != 30*time.Second {
		t.Errorf("expected PingInterval 30s, got %v", config.PingInterval)
	}
}

func TestAgentClient_Connect(t *testing.T) {
	// Create a test WebSocket server
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Echo server - just keep connection alive
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	// Convert http URL to ws URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	config := DefaultAgentClientConfig(wsURL, "agent-test")
	client, err := NewAgentClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test connect
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	if !client.IsConnected() {
		t.Error("expected client to be connected")
	}

	// Test close
	err = client.Close()
	if err != nil {
		t.Errorf("failed to close: %v", err)
	}

	if client.IsConnected() {
		t.Error("expected client to be disconnected after close")
	}
}

func TestAgentClient_SendReceive(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Echo server
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	config := DefaultAgentClientConfig(wsURL, "agent-test")
	client, err := NewAgentClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	// Send a message
	msg := NewChatMessage("agent-test", "owner", "Hello!")
	err = client.Send(msg)
	if err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	// Receive the echoed message
	recvCtx, recvCancel := context.WithTimeout(ctx, 2*time.Second)
	defer recvCancel()

	received, err := client.Receive(recvCtx)
	if err != nil {
		t.Fatalf("failed to receive: %v", err)
	}

	if received.Content != "Hello!" {
		t.Errorf("expected content 'Hello!', got %q", received.Content)
	}
	if received.From != "agent-test" {
		t.Errorf("expected from 'agent-test', got %q", received.From)
	}
}

func TestAgentClient_SendNotConnected(t *testing.T) {
	config := DefaultAgentClientConfig("ws://localhost:8080/ws", "agent-test")
	client, err := NewAgentClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to send without connecting
	msg := NewChatMessage("agent-test", "owner", "Hello!")
	err = client.Send(msg)
	if err == nil {
		t.Error("expected error when sending without connection")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestAgentClient_DoubleClose(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	config := DefaultAgentClientConfig(wsURL, "agent-test")
	client, err := NewAgentClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Close twice - should not panic
	err = client.Close()
	if err != nil {
		t.Errorf("first close failed: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestAgentClient_ConnectFailure(t *testing.T) {
	// Try to connect to a non-existent server
	config := DefaultAgentClientConfig("ws://localhost:59999/ws", "agent-test")
	client, err := NewAgentClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err == nil {
		t.Error("expected error when connecting to non-existent server")
	}
}

func TestAgentClient_MessageFromField(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	var receivedData []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Just receive and store the first message
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		receivedData = data
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	config := DefaultAgentClientConfig(wsURL, "my-agent")
	client, err := NewAgentClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	// Send a message with different From field
	msg := NewChatMessage("wrong-agent", "owner", "Test")
	err = client.Send(msg)
	if err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	// Wait for message to be received
	time.Sleep(100 * time.Millisecond)

	// Parse and check the From field was overwritten
	if len(receivedData) > 0 {
		received, err := ParseMessage(receivedData)
		if err != nil {
			t.Fatalf("failed to parse received message: %v", err)
		}
		if received.From != "my-agent" {
			t.Errorf("expected From to be 'my-agent', got %q", received.From)
		}
	}
}
