package ws

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for local development
	// TODO: Add proper origin validation for production
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Server represents a WebSocket server
type Server struct {
	hub        *Hub
	httpServer *http.Server
	port       int
	running    atomic.Bool
	mu         sync.Mutex
	clientID   int64 // For generating unique client IDs
}

// NewServer creates a new WebSocket server
func NewServer(port int) *Server {
	return &Server{
		hub:  NewHub(),
		port: port,
	}
}

// Start starts the WebSocket server
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running.Load() {
		return fmt.Errorf("server already running")
	}

	// Start the hub
	go s.hub.Run()

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	s.running.Store(true)

	// Start HTTP server in background
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.running.Store(false)
		}
	}()

	return nil
}

// Stop stops the WebSocket server
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running.Load() {
		return nil
	}

	s.running.Store(false)

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}

// handleWebSocket handles WebSocket connection requests
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Generate a unique client ID or use query param
	clientID := r.URL.Query().Get("id")
	if clientID == "" {
		clientID = fmt.Sprintf("client-%d", atomic.AddInt64(&s.clientID, 1))
	}

	client := NewClient(s.hub, conn, clientID)

	// Register the client
	s.hub.Register(client)

	// Send history if requested
	if r.URL.Query().Get("history") == "true" {
		history := s.hub.GetHistory()
		if len(history) > 0 {
			client.SendHistory(history)
		}
	}

	// Start client pumps
	go client.WritePump()
	client.ReadPump()
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","clients":%d}`, s.hub.ClientCount())
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	return s.running.Load()
}

// GetHub returns the hub instance
func (s *Server) GetHub() *Hub {
	return s.hub
}

// Port returns the server port
func (s *Server) Port() int {
	return s.port
}

// Broadcast sends a message to all connected clients
func (s *Server) Broadcast(message *Message) {
	s.hub.Broadcast(message)
}

// ClientCount returns the number of connected clients
func (s *Server) ClientCount() int {
	return s.hub.ClientCount()
}
