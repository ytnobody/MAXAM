package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/ytnobody/MAXAM/internal/taskqueue"
)

func TestCoordinator_RegisterAgent(t *testing.T) {
	store := taskqueue.NewMemoryStore()
	queue := taskqueue.NewQueue(store)
	coord := NewCoordinator(DefaultConfig(), queue)

	coord.RegisterAgent("yuki-1")

	info, exists := coord.GetAgentState("yuki-1")
	if !exists {
		t.Fatal("Agent should exist after registration")
	}
	if info.State != StateHealthy {
		t.Errorf("State = %s, want healthy", info.State)
	}
}

func TestCoordinator_HandleHeartbeat(t *testing.T) {
	store := taskqueue.NewMemoryStore()
	queue := taskqueue.NewQueue(store)
	coord := NewCoordinator(DefaultConfig(), queue)

	// Auto-register via heartbeat
	coord.HandleHeartbeat("rin-1")

	info, exists := coord.GetAgentState("rin-1")
	if !exists {
		t.Fatal("Agent should exist after heartbeat")
	}
	if info.State != StateHealthy {
		t.Errorf("State = %s, want healthy", info.State)
	}
	if info.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", info.FailCount)
	}
}

func TestCoordinator_HandleFailure_WithRecovery(t *testing.T) {
	store := taskqueue.NewMemoryStore()
	queue := taskqueue.NewQueue(store)

	cfg := Config{
		HealthCheckInterval: 100 * time.Millisecond,
		ResponseTimeout:     100 * time.Millisecond,
		MaxRetries:          3,
		BackoffBase:         10 * time.Millisecond, // Short for testing
		BackoffMax:          50 * time.Millisecond,
	}
	coord := NewCoordinator(cfg, queue)

	// Set recovery function that succeeds
	recoverCalled := false
	coord.SetRecoverFunc(func(ctx context.Context, name string) error {
		recoverCalled = true
		return nil
	})

	coord.RegisterAgent("shiori-1")

	ctx := context.Background()
	recovered := coord.HandleFailure(ctx, "shiori-1")

	if !recoverCalled {
		t.Error("Recovery function should have been called")
	}
	if !recovered {
		t.Error("HandleFailure should return true when recovery succeeds")
	}

	info, _ := coord.GetAgentState("shiori-1")
	if info.State != StateHealthy {
		t.Errorf("State = %s, want healthy after recovery", info.State)
	}
}

func TestCoordinator_HandleFailure_MaxRetries(t *testing.T) {
	store := taskqueue.NewMemoryStore()
	queue := taskqueue.NewQueue(store)

	cfg := Config{
		MaxRetries:  2,
		BackoffBase: 1 * time.Millisecond,
		BackoffMax:  5 * time.Millisecond,
	}
	coord := NewCoordinator(cfg, queue)

	// Set recovery function that always fails
	coord.SetRecoverFunc(func(ctx context.Context, name string) error {
		return context.DeadlineExceeded
	})

	// Track tasks for the agent
	coord.SetGetTasksFunc(func(name string) []int {
		return []int{123, 456}
	})

	coord.RegisterAgent("priya-1")

	ctx := context.Background()

	// Fail multiple times
	for i := 0; i < cfg.MaxRetries; i++ {
		coord.HandleFailure(ctx, "priya-1")
	}

	info, _ := coord.GetAgentState("priya-1")
	if info.State != StateFailed {
		t.Errorf("State = %s, want failed after max retries", info.State)
	}

	// Check that tasks were added to queue
	pending, _ := queue.ListPending()
	if len(pending) != 2 {
		t.Errorf("Pending tasks = %d, want 2", len(pending))
	}
}

func TestCoordinator_CalculateBackoff(t *testing.T) {
	cfg := Config{
		BackoffBase: 5 * time.Second,
		BackoffMax:  60 * time.Second,
	}
	coord := NewCoordinator(cfg, nil)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 5 * time.Second},  // base * 2^0 = 5
		{2, 10 * time.Second}, // base * 2^1 = 10
		{3, 20 * time.Second}, // base * 2^2 = 20
		{4, 40 * time.Second}, // base * 2^3 = 40
		{5, 60 * time.Second}, // capped at max
		{6, 60 * time.Second}, // capped at max
	}

	for _, tt := range tests {
		got := coord.calculateBackoff(tt.attempt)
		if got != tt.expected {
			t.Errorf("calculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

func TestCoordinator_GetAllAgents(t *testing.T) {
	store := taskqueue.NewMemoryStore()
	queue := taskqueue.NewQueue(store)
	coord := NewCoordinator(DefaultConfig(), queue)

	coord.RegisterAgent("yuki-1")
	coord.RegisterAgent("rin-1")
	coord.RegisterAgent("shiori-1")

	agents := coord.GetAllAgents()
	if len(agents) != 3 {
		t.Errorf("GetAllAgents() = %d agents, want 3", len(agents))
	}
}

func TestCoordinator_UnregisterAgent(t *testing.T) {
	store := taskqueue.NewMemoryStore()
	queue := taskqueue.NewQueue(store)
	coord := NewCoordinator(DefaultConfig(), queue)

	coord.RegisterAgent("amara-1")
	coord.UnregisterAgent("amara-1")

	_, exists := coord.GetAgentState("amara-1")
	if exists {
		t.Error("Agent should not exist after unregistration")
	}
}
