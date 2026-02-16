package taskqueue

import (
	"context"
	"testing"
)

func TestCoordinator_OnAgentFailure(t *testing.T) {
	q := NewQueue()

	// Register agents.
	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
	})
	q.RegisterAgent(AgentCapability{
		AgentName: "agent2",
		Available: true,
	})

	// Add tasks to agent1.
	q.AddTask(&Task{
		ID:           "task1",
		AssignedTo:   "agent1",
		Status:       TaskStatusInProgress,
		Handoverable: true,
	})
	q.AddTask(&Task{
		ID:           "task2",
		AssignedTo:   "agent1",
		Status:       TaskStatusInProgress,
		Handoverable: true,
	})
	q.tasksByAgent["agent1"] = []string{"task1", "task2"}

	// Track handover handler calls.
	handoverCalls := 0
	handoverHandler := func(ctx context.Context, result HandoverResult) error {
		handoverCalls++
		return nil
	}

	// Track notify handler calls.
	notifyCalls := 0
	var lastNotifyMessage string
	notifyHandler := func(ctx context.Context, target, message string) error {
		notifyCalls++
		lastNotifyMessage = message
		return nil
	}

	coord := NewCoordinator(q,
		WithHandoverHandler(handoverHandler),
		WithNotifyHandler(notifyHandler),
	)

	// Simulate agent1 failure.
	results := coord.OnAgentFailure(context.Background(), "agent1", "agent unresponsive")

	// Verify handovers.
	if len(results) != 2 {
		t.Errorf("expected 2 handover results, got %d", len(results))
	}

	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
			if r.ToAgent != "agent2" {
				t.Errorf("expected handover to agent2, got %s", r.ToAgent)
			}
		}
	}
	if successCount != 2 {
		t.Errorf("expected 2 successful handovers, got %d", successCount)
	}

	// Verify handler calls.
	if handoverCalls != 2 {
		t.Errorf("expected 2 handover handler calls, got %d", handoverCalls)
	}
	if notifyCalls != 1 {
		t.Errorf("expected 1 notify call, got %d", notifyCalls)
	}
	if lastNotifyMessage == "" {
		t.Error("expected notify message to be set")
	}
}

func TestCoordinator_OnAgentFailure_NoTasks(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
	})

	coord := NewCoordinator(q)

	results := coord.OnAgentFailure(context.Background(), "agent1", "agent unresponsive")

	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
}

func TestCoordinator_OnAgentFailure_PartialSuccess(t *testing.T) {
	q := NewQueue()

	// Only one other agent, will be at capacity after first handover.
	q.RegisterAgent(AgentCapability{
		AgentName:    "agent1",
		Available:    true,
		CurrentTasks: 0,
	})
	q.RegisterAgent(AgentCapability{
		AgentName:    "agent2",
		Available:    true,
		MaxTasks:     1, // Can only take 1 task.
		CurrentTasks: 0,
	})

	// Add 2 tasks to agent1.
	q.AddTask(&Task{
		ID:           "task1",
		AssignedTo:   "agent1",
		Status:       TaskStatusInProgress,
		Handoverable: true,
	})
	q.AddTask(&Task{
		ID:           "task2",
		AssignedTo:   "agent1",
		Status:       TaskStatusInProgress,
		Handoverable: true,
	})
	q.tasksByAgent["agent1"] = []string{"task1", "task2"}

	var notifyMessage string
	notifyHandler := func(ctx context.Context, target, message string) error {
		notifyMessage = message
		return nil
	}

	coord := NewCoordinator(q, WithNotifyHandler(notifyHandler))

	results := coord.OnAgentFailure(context.Background(), "agent1", "agent unresponsive")

	// First handover should succeed, second should fail.
	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	if successCount != 1 {
		t.Errorf("expected 1 success, got %d", successCount)
	}
	if failCount != 1 {
		t.Errorf("expected 1 failure, got %d", failCount)
	}

	// Notify message should mention manual intervention.
	if notifyMessage == "" {
		t.Error("expected notify message to be set")
	}
}

func TestCoordinator_MarkAgentAvailability(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
	})

	coord := NewCoordinator(q)

	// Mark unavailable.
	err := coord.MarkAgentUnavailable("agent1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agents := q.GetAvailableAgents()
	if len(agents) != 0 {
		t.Errorf("expected 0 available agents, got %d", len(agents))
	}

	// Mark available.
	err = coord.MarkAgentAvailable("agent1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agents = q.GetAvailableAgents()
	if len(agents) != 1 {
		t.Errorf("expected 1 available agent, got %d", len(agents))
	}
}

func TestCoordinator_RegisterTask(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
	})

	coord := NewCoordinator(q)

	task := &Task{
		ID:           "task1",
		Description:  "Test task",
		Handoverable: true,
	}

	err := coord.RegisterTask(task, "agent1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := q.GetTask("task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AssignedTo != "agent1" {
		t.Errorf("expected agent1, got %s", got.AssignedTo)
	}
	if got.Status != TaskStatusInProgress {
		t.Errorf("expected in_progress status, got %s", got.Status)
	}
}

func TestCoordinator_CompleteAndFailTask(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
	})

	coord := NewCoordinator(q)

	// Test CompleteTask.
	task1 := &Task{ID: "task1", Handoverable: true}
	_ = coord.RegisterTask(task1, "agent1")

	err := coord.CompleteTask("task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got1, _ := q.GetTask("task1")
	if got1.Status != TaskStatusCompleted {
		t.Errorf("expected completed, got %s", got1.Status)
	}

	// Test FailTask.
	task2 := &Task{ID: "task2", Handoverable: true}
	_ = coord.RegisterTask(task2, "agent1")

	err = coord.FailTask("task2", "test error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got2, _ := q.GetTask("task2")
	if got2.Status != TaskStatusFailed {
		t.Errorf("expected failed, got %s", got2.Status)
	}
}

func TestCoordinator_GetTasks(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
	})

	coord := NewCoordinator(q)

	// Add some tasks.
	_ = coord.RegisterTask(&Task{ID: "task1"}, "agent1")
	q.AddTask(&Task{ID: "task2", Status: TaskStatusPending})

	// Test GetTasksForAgent.
	tasks := coord.GetTasksForAgent("agent1")
	if len(tasks) != 1 {
		t.Errorf("expected 1 task for agent1, got %d", len(tasks))
	}

	// Test GetPendingTasks.
	pending := coord.GetPendingTasks()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(pending))
	}
}

func TestCoordinator_WithPMAgent(t *testing.T) {
	q := NewQueue()

	var notifyTarget string
	notifyHandler := func(ctx context.Context, target, message string) error {
		notifyTarget = target
		return nil
	}

	coord := NewCoordinator(q,
		WithPMAgent("custom_pm"),
		WithNotifyHandler(notifyHandler),
	)

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
	})
	q.RegisterAgent(AgentCapability{
		AgentName: "agent2",
		Available: true,
	})

	q.AddTask(&Task{
		ID:           "task1",
		AssignedTo:   "agent1",
		Status:       TaskStatusInProgress,
		Handoverable: true,
	})
	q.tasksByAgent["agent1"] = []string{"task1"}

	coord.OnAgentFailure(context.Background(), "agent1", "test")

	if notifyTarget != "custom_pm" {
		t.Errorf("expected custom_pm, got %s", notifyTarget)
	}
}
