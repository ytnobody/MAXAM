package ws

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestTUIClient_ConnectToServer(t *testing.T) {
	// Start a test server on a specific port
	testPort := 18765
	server := NewServer(testPort)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Stop(ctx)
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Create client
	cfg := DefaultTUIClientConfig(testPort, "test-tui")
	client := NewTUIClient(cfg)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected")
	}

	// Wait for join notification to propagate
	time.Sleep(100 * time.Millisecond)

	if server.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", server.ClientCount())
	}

	// Disconnect
	if err := client.Disconnect(); err != nil {
		t.Errorf("Failed to disconnect: %v", err)
	}

	if client.IsConnected() {
		t.Error("Client should be disconnected")
	}
}

func TestTUIClient_SendAndReceive(t *testing.T) {
	// Start server on a specific port
	testPort := 18766
	server := NewServer(testPort)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Stop(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create two clients
	cfg1 := DefaultTUIClientConfig(testPort, "client1")
	client1 := NewTUIClient(cfg1)

	cfg2 := DefaultTUIClientConfig(testPort, "client2")
	client2 := NewTUIClient(cfg2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect both clients
	if err := client1.Connect(ctx); err != nil {
		t.Fatalf("Client1 failed to connect: %v", err)
	}
	defer client1.Disconnect()

	if err := client2.Connect(ctx); err != nil {
		t.Fatalf("Client2 failed to connect: %v", err)
	}
	defer client2.Disconnect()

	// Wait for connections
	time.Sleep(200 * time.Millisecond)

	// Set up message receiver for client2
	var received *Message
	var mu sync.Mutex
	done := make(chan struct{})

	client2.SetMessageHandler(func(msg *Message) {
		mu.Lock()
		defer mu.Unlock()
		if msg.Type == TypeChat && msg.From == "client1" {
			received = msg
			close(done)
		}
	})

	// Send message from client1
	if err := client1.SendChat("client2", "Hello from client1"); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for message
	select {
	case <-done:
		mu.Lock()
		if received == nil {
			t.Error("Expected to receive message")
		} else if received.Content != "Hello from client1" {
			t.Errorf("Unexpected content: %s", received.Content)
		}
		mu.Unlock()
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestTUIClient_ConnectWithRetry(t *testing.T) {
	// Start server on a specific port
	testPort := 18767
	server := NewServer(testPort)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Stop(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create client with retry config
	cfg := TUIClientConfig{
		ServerURL:     DefaultTUIClientConfig(testPort, "").ServerURL,
		ClientID:      "retry-test",
		ReconnectWait: 100 * time.Millisecond,
		MaxReconnect:  3,
	}
	client := NewTUIClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test ConnectWithRetry succeeds
	if err := client.ConnectWithRetry(ctx); err != nil {
		t.Errorf("Failed to connect with retry: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected")
	}

	client.Disconnect()
}

func TestTUIClient_ConnectWithRetry_Fails(t *testing.T) {
	// No server running - should fail after max retries
	cfg := TUIClientConfig{
		ServerURL:     "ws://localhost:19999/ws",
		ClientID:      "retry-fail-test",
		ReconnectWait: 50 * time.Millisecond,
		MaxReconnect:  2,
	}
	client := NewTUIClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should fail after retries
	err := client.ConnectWithRetry(ctx)
	if err == nil {
		t.Error("Expected error when server not available")
	}
}

func TestTUIClient_ConnectHandlers(t *testing.T) {
	// Start server on a specific port
	testPort := 18769
	server := NewServer(testPort)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Stop(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	cfg := DefaultTUIClientConfig(testPort, "handler-test")
	client := NewTUIClient(cfg)

	// Track handlers called
	connected := make(chan struct{})
	disconnected := make(chan struct{})

	client.SetConnectHandler(func() {
		close(connected)
	})

	client.SetDisconnectHandler(func(err error) {
		close(disconnected)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Wait for connect handler
	select {
	case <-connected:
		// OK
	case <-time.After(time.Second):
		t.Error("Connect handler not called")
	}

	// Disconnect
	client.Disconnect()

	// Wait for disconnect handler
	select {
	case <-disconnected:
		// OK
	case <-time.After(time.Second):
		t.Error("Disconnect handler not called")
	}
}
