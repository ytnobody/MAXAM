package taskqueue

import (
	"testing"
	"time"
)

func TestQueue_RegisterAgent(t *testing.T) {
	q := NewQueue()

	cap := AgentCapability{
		AgentName:    "agent1",
		Available:    true,
		CurrentTasks: 0,
		MaxTasks:     5,
	}
	q.RegisterAgent(cap)

	agents := q.GetAvailableAgents()
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].AgentName != "agent1" {
		t.Errorf("expected agent1, got %s", agents[0].AgentName)
	}
}

func TestQueue_AddTask(t *testing.T) {
	q := NewQueue()

	task := &Task{
		ID:           "task1",
		Description:  "Test task",
		Handoverable: true,
	}
	q.AddTask(task)

	got, err := q.GetTask("task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "task1" {
		t.Errorf("expected task1, got %s", got.ID)
	}
	if got.Status != TaskStatusPending {
		t.Errorf("expected pending status, got %s", got.Status)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestQueue_AssignTask(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
		MaxTasks:  5,
	})

	task := &Task{
		ID:           "task1",
		Description:  "Test task",
		Handoverable: true,
	}
	q.AddTask(task)

	err := q.AssignTask("task1", "agent1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := q.GetTask("task1")
	if got.AssignedTo != "agent1" {
		t.Errorf("expected agent1, got %s", got.AssignedTo)
	}
	if got.Status != TaskStatusInProgress {
		t.Errorf("expected in_progress status, got %s", got.Status)
	}

	tasks := q.GetTasksForAgent("agent1")
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

func TestQueue_AssignTask_Errors(t *testing.T) {
	q := NewQueue()

	// Task not found.
	err := q.AssignTask("nonexistent", "agent1")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}

	// Agent not found.
	q.AddTask(&Task{ID: "task1"})
	err = q.AssignTask("task1", "nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestQueue_CompleteTask(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
	})

	task := &Task{ID: "task1"}
	q.AddTask(task)
	_ = q.AssignTask("task1", "agent1")

	err := q.CompleteTask("task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := q.GetTask("task1")
	if got.Status != TaskStatusCompleted {
		t.Errorf("expected completed status, got %s", got.Status)
	}
	if got.CompletedAt.IsZero() {
		t.Error("expected CompletedAt to be set")
	}

	// Task should be removed from agent's list.
	tasks := q.GetTasksForAgent("agent1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestQueue_FailTask(t *testing.T) {
	q := NewQueue()

	task := &Task{ID: "task1"}
	q.AddTask(task)

	err := q.FailTask("task1", "some error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := q.GetTask("task1")
	if got.Status != TaskStatusFailed {
		t.Errorf("expected failed status, got %s", got.Status)
	}
	if got.Metadata["fail_reason"] != "some error" {
		t.Errorf("expected fail_reason to be set")
	}
}

func TestQueue_Handover_Success(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
	})
	q.RegisterAgent(AgentCapability{
		AgentName: "agent2",
		Available: true,
	})

	task := &Task{
		ID:           "task1",
		AssignedTo:   "agent1",
		Status:       TaskStatusInProgress,
		Handoverable: true,
	}
	q.AddTask(task)
	q.tasksByAgent["agent1"] = []string{"task1"}

	result := q.Handover(HandoverRequest{
		TaskID:    "task1",
		FromAgent: "agent1",
		ToAgent:   "agent2",
		Reason:    "agent1 is unresponsive",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.ToAgent != "agent2" {
		t.Errorf("expected agent2, got %s", result.ToAgent)
	}

	got, _ := q.GetTask("task1")
	if got.AssignedTo != "agent2" {
		t.Errorf("expected agent2, got %s", got.AssignedTo)
	}
	if got.HandoverCount != 1 {
		t.Errorf("expected handover count 1, got %d", got.HandoverCount)
	}
	if len(got.PreviousAgents) != 1 || got.PreviousAgents[0] != "agent1" {
		t.Errorf("expected previous agents [agent1], got %v", got.PreviousAgents)
	}
}

func TestQueue_Handover_AutoAssign(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName:    "agent1",
		Available:    true,
		CurrentTasks: 2,
	})
	q.RegisterAgent(AgentCapability{
		AgentName:    "agent2",
		Available:    true,
		CurrentTasks: 0, // Fewer tasks, should be chosen.
	})

	task := &Task{
		ID:           "task1",
		AssignedTo:   "agent1",
		Status:       TaskStatusInProgress,
		Handoverable: true,
	}
	q.AddTask(task)
	q.tasksByAgent["agent1"] = []string{"task1"}

	result := q.Handover(HandoverRequest{
		TaskID:    "task1",
		FromAgent: "agent1",
		ToAgent:   "", // Auto-assign.
		Reason:    "agent1 is unresponsive",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.ToAgent != "agent2" {
		t.Errorf("expected agent2 (fewer tasks), got %s", result.ToAgent)
	}
}

func TestQueue_Handover_NotHandoverable(t *testing.T) {
	q := NewQueue()

	task := &Task{
		ID:           "task1",
		Handoverable: false,
	}
	q.AddTask(task)

	result := q.Handover(HandoverRequest{
		TaskID:    "task1",
		FromAgent: "agent1",
	})

	if result.Success {
		t.Error("expected failure for non-handoverable task")
	}
	if result.Error != ErrTaskNotHandoverable.Error() {
		t.Errorf("expected ErrTaskNotHandoverable, got %s", result.Error)
	}
}

func TestQueue_Handover_MaxHandovers(t *testing.T) {
	q := NewQueue()

	task := &Task{
		ID:            "task1",
		Handoverable:  true,
		MaxHandovers:  2,
		HandoverCount: 2, // Already at max.
	}
	q.AddTask(task)

	result := q.Handover(HandoverRequest{
		TaskID:    "task1",
		FromAgent: "agent1",
	})

	if result.Success {
		t.Error("expected failure when max handovers exceeded")
	}
	if result.Error != ErrMaxHandoversExceeded.Error() {
		t.Errorf("expected ErrMaxHandoversExceeded, got %s", result.Error)
	}
}

func TestQueue_Handover_NoAvailableAgent(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: false, // Not available.
	})

	task := &Task{
		ID:           "task1",
		AssignedTo:   "agent1",
		Handoverable: true,
	}
	q.AddTask(task)
	q.tasksByAgent["agent1"] = []string{"task1"}

	result := q.Handover(HandoverRequest{
		TaskID:    "task1",
		FromAgent: "agent1",
		ToAgent:   "", // Auto-assign.
	})

	if result.Success {
		t.Error("expected failure when no available agent")
	}
	if result.Error != ErrNoAvailableAgent.Error() {
		t.Errorf("expected ErrNoAvailableAgent, got %s", result.Error)
	}
}

func TestQueue_GetHandoverableTasks(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{AgentName: "agent1"})

	// Handoverable task.
	q.AddTask(&Task{
		ID:           "task1",
		AssignedTo:   "agent1",
		Handoverable: true,
	})

	// Non-handoverable task.
	q.AddTask(&Task{
		ID:           "task2",
		AssignedTo:   "agent1",
		Handoverable: false,
	})

	// Handoverable but at max handovers.
	q.AddTask(&Task{
		ID:            "task3",
		AssignedTo:    "agent1",
		Handoverable:  true,
		MaxHandovers:  1,
		HandoverCount: 1,
	})

	q.tasksByAgent["agent1"] = []string{"task1", "task2", "task3"}

	tasks := q.GetHandoverableTasks("agent1")
	if len(tasks) != 1 {
		t.Errorf("expected 1 handoverable task, got %d", len(tasks))
	}
	if tasks[0].ID != "task1" {
		t.Errorf("expected task1, got %s", tasks[0].ID)
	}
}

func TestQueue_GetPendingTasks(t *testing.T) {
	q := NewQueue()

	q.AddTask(&Task{ID: "task1", Status: TaskStatusPending})
	q.AddTask(&Task{ID: "task2", Status: TaskStatusInProgress})
	q.AddTask(&Task{ID: "task3", Status: TaskStatusPending})

	tasks := q.GetPendingTasks()
	if len(tasks) != 2 {
		t.Errorf("expected 2 pending tasks, got %d", len(tasks))
	}
}

func TestQueue_SetAgentAvailability(t *testing.T) {
	q := NewQueue()

	err := q.SetAgentAvailability("nonexistent", true)
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}

	q.RegisterAgent(AgentCapability{
		AgentName: "agent1",
		Available: true,
	})

	err = q.SetAgentAvailability("agent1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agents := q.GetAvailableAgents()
	if len(agents) != 0 {
		t.Errorf("expected 0 available agents, got %d", len(agents))
	}
}

func TestQueue_RemoveTask(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{AgentName: "agent1"})

	task := &Task{
		ID:         "task1",
		AssignedTo: "agent1",
	}
	q.AddTask(task)
	q.tasksByAgent["agent1"] = []string{"task1"}

	err := q.RemoveTask("task1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = q.GetTask("task1")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}

	tasks := q.GetTasksForAgent("agent1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestQueue_UnregisterAgent(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{AgentName: "agent1"})
	q.UnregisterAgent("agent1")

	agents := q.GetAvailableAgents()
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestTask_Metadata(t *testing.T) {
	q := NewQueue()

	task := &Task{
		ID: "task1",
		Metadata: map[string]string{
			"key": "value",
		},
	}
	q.AddTask(task)

	got, _ := q.GetTask("task1")

	// Verify copy is returned, not original.
	got.Metadata["key"] = "modified"

	original, _ := q.GetTask("task1")
	if original.Metadata["key"] != "value" {
		t.Error("original task metadata should not be modified")
	}
}

func TestQueue_AgentMaxTasks(t *testing.T) {
	q := NewQueue()

	q.RegisterAgent(AgentCapability{
		AgentName:    "agent1",
		Available:    true,
		CurrentTasks: 5,
		MaxTasks:     5, // At capacity.
	})

	task := &Task{
		ID:           "task1",
		Handoverable: true,
	}
	q.AddTask(task)

	result := q.Handover(HandoverRequest{
		TaskID:    "task1",
		FromAgent: "other",
		ToAgent:   "agent1",
	})

	if result.Success {
		t.Error("expected failure when agent at max tasks")
	}
	if result.Error != "target agent has reached max tasks" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

// Concurrency test.
func TestQueue_Concurrent(t *testing.T) {
	q := NewQueue()

	// Register agents.
	for i := 0; i < 5; i++ {
		q.RegisterAgent(AgentCapability{
			AgentName: "agent" + string(rune('0'+i)),
			Available: true,
			MaxTasks:  10,
		})
	}

	// Add tasks concurrently.
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(id int) {
			task := &Task{
				ID:           "task" + string(rune(id)),
				Handoverable: true,
			}
			q.AddTask(task)
			done <- true
		}(i)
	}

	// Wait for all goroutines.
	for i := 0; i < 100; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for concurrent operations")
		}
	}
}
