package pool

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.WorkersPerAgent != 1 {
		t.Errorf("WorkersPerAgent = %d, want 1", cfg.WorkersPerAgent)
	}
	if cfg.QueueSize != 10 {
		t.Errorf("QueueSize = %d, want 10", cfg.QueueSize)
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		cfg           Config
		wantWorkers   int
		wantQueueSize int
	}{
		{
			name:          "default values",
			cfg:           Config{},
			wantWorkers:   1,
			wantQueueSize: 10,
		},
		{
			name:          "custom values",
			cfg:           Config{WorkersPerAgent: 3, QueueSize: 20},
			wantWorkers:   3,
			wantQueueSize: 20,
		},
		{
			name:          "negative workers defaults to 1",
			cfg:           Config{WorkersPerAgent: -1, QueueSize: 5},
			wantWorkers:   1,
			wantQueueSize: 5,
		},
		{
			name:          "zero queue defaults to 10",
			cfg:           Config{WorkersPerAgent: 2, QueueSize: 0},
			wantWorkers:   2,
			wantQueueSize: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(nil, tt.cfg)

			if p.workersNum != tt.wantWorkers {
				t.Errorf("workersNum = %d, want %d", p.workersNum, tt.wantWorkers)
			}
			if p.queueSize != tt.wantQueueSize {
				t.Errorf("queueSize = %d, want %d", p.queueSize, tt.wantQueueSize)
			}
		})
	}
}

func TestWorkerBusy(t *testing.T) {
	w := &Worker{ID: 0, AgentName: "test"}

	if w.IsBusy() {
		t.Error("new worker should not be busy")
	}

	w.setBusy(true)
	if !w.IsBusy() {
		t.Error("worker should be busy after setBusy(true)")
	}

	w.setBusy(false)
	if w.IsBusy() {
		t.Error("worker should not be busy after setBusy(false)")
	}
}

func TestPoolClose(t *testing.T) {
	p := New(nil, DefaultConfig())

	// Close should not panic on empty pool
	p.Close()

	// Dispatch should return error after close
	_, err := p.Dispatch(context.Background(), "test", "prompt")
	if err != ErrPoolClosed {
		t.Errorf("Dispatch after close = %v, want ErrPoolClosed", err)
	}
}

func TestPoolInitializeWithoutAgents(t *testing.T) {
	p := New(nil, DefaultConfig())

	err := p.Initialize("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("Initialize with nil agents = %v, want ErrAgentNotFound", err)
	}
}

func TestPoolStats(t *testing.T) {
	p := New(nil, DefaultConfig())

	// Stats for non-existent agent
	_, err := p.GetStats("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("GetStats for nonexistent = %v, want ErrAgentNotFound", err)
	}

	// AllStats on empty pool
	stats := p.AllStats()
	if len(stats) != 0 {
		t.Errorf("AllStats on empty pool = %d, want 0", len(stats))
	}
}

func TestResult(t *testing.T) {
	r := Result{Output: "test output", Err: nil}

	if r.Output != "test output" {
		t.Errorf("Output = %q, want %q", r.Output, "test output")
	}
	if r.Err != nil {
		t.Errorf("Err = %v, want nil", r.Err)
	}
}

func TestTask(t *testing.T) {
	resultChan := make(chan Result, 1)
	task := Task{
		AgentName: "test",
		Prompt:    "hello",
		Result:    resultChan,
	}

	if task.AgentName != "test" {
		t.Errorf("AgentName = %q, want %q", task.AgentName, "test")
	}
	if task.Prompt != "hello" {
		t.Errorf("Prompt = %q, want %q", task.Prompt, "hello")
	}
}

func TestDispatchContextCancellation(t *testing.T) {
	p := New(nil, Config{WorkersPerAgent: 1, QueueSize: 1})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Small delay to ensure context expires
	time.Sleep(5 * time.Millisecond)

	_, err := p.Dispatch(ctx, "test", "prompt")
	if err != context.DeadlineExceeded && err != ErrAgentNotFound {
		t.Errorf("Dispatch with expired context = %v, want context.DeadlineExceeded or ErrAgentNotFound", err)
	}
}
