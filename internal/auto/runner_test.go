package auto

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

func TestNewRunner(t *testing.T) {
	executor := NewStubExecutor()
	runner := NewRunner(executor, DefaultConfig())

	if runner.State() != RunnerStateIdle {
		t.Errorf("expected idle state, got %s", runner.State())
	}

	if len(runner.Tasks()) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(runner.Tasks()))
	}
}

func TestRunner_AddTask(t *testing.T) {
	executor := NewStubExecutor()
	runner := NewRunner(executor, DefaultConfig())

	runner.AddTask(1, "First task")
	runner.AddTask(2, "Second task")

	tasks := runner.Tasks()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	if tasks[0].IssueNumber != 1 || tasks[0].Title != "First task" {
		t.Errorf("unexpected first task: %+v", tasks[0])
	}

	if tasks[1].IssueNumber != 2 || tasks[1].Title != "Second task" {
		t.Errorf("unexpected second task: %+v", tasks[1])
	}

	if runner.PendingCount() != 2 {
		t.Errorf("expected 2 pending, got %d", runner.PendingCount())
	}
}

func TestRunner_AddTasks(t *testing.T) {
	executor := NewStubExecutor()
	runner := NewRunner(executor, DefaultConfig())

	issues := []IssueInfo{
		{Number: 10, Title: "Issue 10"},
		{Number: 20, Title: "Issue 20"},
		{Number: 30, Title: "Issue 30"},
	}
	runner.AddTasks(issues)

	if len(runner.Tasks()) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(runner.Tasks()))
	}

	if runner.PendingCount() != 3 {
		t.Errorf("expected 3 pending, got %d", runner.PendingCount())
	}
}

func TestRunner_Run_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	executor := NewStubExecutor()
	executor.SimulatedDuration = 10 * time.Millisecond
	executor.NextPRNumber = 100
	executor.Logger = logger

	runner := NewRunner(executor, Config{Logger: logger})
	runner.AddTask(1, "Task 1")
	runner.AddTask(2, "Task 2")

	ctx := context.Background()
	err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runner.State() != RunnerStateCompleted {
		t.Errorf("expected completed state, got %s", runner.State())
	}

	if runner.CompletedCount() != 2 {
		t.Errorf("expected 2 completed, got %d", runner.CompletedCount())
	}

	if runner.PendingCount() != 0 {
		t.Errorf("expected 0 pending, got %d", runner.PendingCount())
	}

	tasks := runner.Tasks()
	if tasks[0].PRNumber != 100 {
		t.Errorf("expected PR #100, got #%d", tasks[0].PRNumber)
	}
	if tasks[1].PRNumber != 101 {
		t.Errorf("expected PR #101, got #%d", tasks[1].PRNumber)
	}
}

func TestRunner_Run_WithFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	executor := NewStubExecutor()
	executor.SimulatedDuration = 10 * time.Millisecond
	executor.Logger = logger
	executor.SetFailure(2, errors.New("simulated failure"))

	runner := NewRunner(executor, Config{Logger: logger})
	runner.AddTask(1, "Task 1")
	runner.AddTask(2, "Task 2 (will fail)")
	runner.AddTask(3, "Task 3")

	ctx := context.Background()
	err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runner.CompletedCount() != 2 {
		t.Errorf("expected 2 completed, got %d", runner.CompletedCount())
	}

	if runner.FailedCount() != 1 {
		t.Errorf("expected 1 failed, got %d", runner.FailedCount())
	}

	tasks := runner.Tasks()
	if tasks[1].State != TaskStateFailed {
		t.Errorf("expected task 2 to be failed, got %s", tasks[1].State)
	}
	if tasks[1].Error == nil {
		t.Error("expected task 2 to have an error")
	}
}

func TestRunner_Run_Cancellation(t *testing.T) {
	executor := NewStubExecutor()
	executor.SimulatedDuration = 1 * time.Second // long duration

	runner := NewRunner(executor, DefaultConfig())
	runner.AddTask(1, "Task 1")
	runner.AddTask(2, "Task 2")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := runner.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded, got %v", err)
	}
}

func TestRunner_Callbacks(t *testing.T) {
	executor := NewStubExecutor()
	executor.SimulatedDuration = 10 * time.Millisecond

	runner := NewRunner(executor, DefaultConfig())

	var startedTasks []int
	var completedTasks []int
	var allCompleteCount int

	runner.SetOnTaskStart(func(task *Task) {
		startedTasks = append(startedTasks, task.IssueNumber)
	})

	runner.SetOnTaskComplete(func(task *Task) {
		completedTasks = append(completedTasks, task.IssueNumber)
	})

	runner.SetOnAllComplete(func(tasks []*Task) {
		allCompleteCount = len(tasks)
	})

	runner.AddTask(1, "Task 1")
	runner.AddTask(2, "Task 2")

	ctx := context.Background()
	_ = runner.Run(ctx)

	if len(startedTasks) != 2 {
		t.Errorf("expected 2 started callbacks, got %d", len(startedTasks))
	}

	if len(completedTasks) != 2 {
		t.Errorf("expected 2 completed callbacks, got %d", len(completedTasks))
	}

	if allCompleteCount != 2 {
		t.Errorf("expected all complete callback with 2 tasks, got %d", allCompleteCount)
	}
}

func TestRunner_DoubleRun(t *testing.T) {
	executor := NewStubExecutor()
	executor.SimulatedDuration = 100 * time.Millisecond

	runner := NewRunner(executor, DefaultConfig())
	runner.AddTask(1, "Task 1")

	// Start first run in goroutine
	go func() {
		_ = runner.Run(context.Background())
	}()

	// Wait for it to start
	time.Sleep(20 * time.Millisecond)

	// Try to run again
	err := runner.Run(context.Background())
	if err == nil {
		t.Error("expected error for double run")
	}
}

func TestRunner_Reset(t *testing.T) {
	executor := NewStubExecutor()
	executor.SimulatedDuration = 10 * time.Millisecond

	runner := NewRunner(executor, DefaultConfig())
	runner.AddTask(1, "Task 1")

	ctx := context.Background()
	_ = runner.Run(ctx)

	if runner.State() != RunnerStateCompleted {
		t.Errorf("expected completed state, got %s", runner.State())
	}

	runner.Reset()

	if runner.State() != RunnerStateIdle {
		t.Errorf("expected idle state after reset, got %s", runner.State())
	}

	if len(runner.Tasks()) != 0 {
		t.Errorf("expected 0 tasks after reset, got %d", len(runner.Tasks()))
	}
}

func TestRunner_Summary(t *testing.T) {
	executor := NewStubExecutor()
	runner := NewRunner(executor, DefaultConfig())

	runner.AddTask(1, "Task 1")
	runner.AddTask(2, "Task 2")

	summary := runner.Summary()
	if !strings.Contains(summary, "2 total") {
		t.Errorf("summary should contain '2 total': %s", summary)
	}
	if !strings.Contains(summary, "2 pending") {
		t.Errorf("summary should contain '2 pending': %s", summary)
	}
}

func TestRunner_PauseResume(t *testing.T) {
	executor := NewStubExecutor()
	runner := NewRunner(executor, DefaultConfig())

	// Can't pause when idle
	runner.Pause()
	if runner.State() != RunnerStateIdle {
		t.Errorf("pause should not change idle state, got %s", runner.State())
	}

	// Can't resume when idle
	runner.Resume()
	if runner.State() != RunnerStateIdle {
		t.Errorf("resume should not change idle state, got %s", runner.State())
	}
}

func TestStubExecutor(t *testing.T) {
	executor := NewStubExecutor()
	executor.SimulatedDuration = 10 * time.Millisecond
	executor.NextPRNumber = 200

	ctx := context.Background()

	prNumber, err := executor.Execute(ctx, 1, "Test task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prNumber != 200 {
		t.Errorf("expected PR #200, got #%d", prNumber)
	}

	// Second call should return 201
	prNumber, err = executor.Execute(ctx, 2, "Test task 2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prNumber != 201 {
		t.Errorf("expected PR #201, got #%d", prNumber)
	}
}

func TestStubExecutor_WithFailure(t *testing.T) {
	executor := NewStubExecutor()
	executor.SetFailure(5, errors.New("cannot process issue 5"))

	ctx := context.Background()

	_, err := executor.Execute(ctx, 5, "Will fail")
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "cannot process issue 5") {
		t.Errorf("unexpected error message: %v", err)
	}
}
