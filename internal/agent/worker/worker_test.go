package worker

import (
	"testing"
	"time"
)

func TestAgentState(t *testing.T) {
	t.Run("initial state is available", func(t *testing.T) {
		state := NewAgentState()
		if !state.IsAvailable() {
			t.Error("expected initial state to be available")
		}
		if state.IsWorking() {
			t.Error("expected initial state to not be working")
		}
	})

	t.Run("start task transitions to working", func(t *testing.T) {
		state := NewAgentState()
		state.StartTask("implementing feature")

		if !state.IsWorking() {
			t.Error("expected state to be working after StartTask")
		}
		if state.IsAvailable() {
			t.Error("expected state to not be available after StartTask")
		}
		if state.GetCurrentTask() != "implementing feature" {
			t.Errorf("expected task to be 'implementing feature', got '%s'", state.GetCurrentTask())
		}
	})

	t.Run("complete task transitions to available", func(t *testing.T) {
		state := NewAgentState()
		state.StartTask("implementing feature")
		state.CompleteTask()

		if !state.IsAvailable() {
			t.Error("expected state to be available after CompleteTask")
		}
		if state.GetCurrentTask() != "" {
			t.Errorf("expected task to be empty after CompleteTask, got '%s'", state.GetCurrentTask())
		}
	})

	t.Run("get status while working", func(t *testing.T) {
		state := NewAgentState()
		state.StartTask("PRレビュー")
		time.Sleep(10 * time.Millisecond)

		status := state.GetStatus()
		if status == "" {
			t.Error("expected non-empty status")
		}
		// Should contain task description
		if len(status) < 5 {
			t.Errorf("expected status to contain task description, got '%s'", status)
		}
	})

	t.Run("get status while available", func(t *testing.T) {
		state := NewAgentState()
		status := state.GetStatus()
		if status != "待機中" {
			t.Errorf("expected status '待機中', got '%s'", status)
		}
	})
}

func TestWorkerChatWhileWorking(t *testing.T) {
	t.Run("returns busy message when working", func(t *testing.T) {
		// Create a worker with nil runner (won't be used for busy response)
		w := NewWorker("yuki", nil)
		w.Start()
		defer w.Stop()

		// Set state to working
		w.state.StartTask("PR #123 レビュー中")

		// Send chat request
		responseChan := make(chan ChatResponse, 1)
		w.SendChat("@yuki 手伝って", responseChan)

		// Should get immediate busy response
		select {
		case resp := <-responseChan:
			if resp.Err != nil {
				t.Errorf("unexpected error: %v", resp.Err)
			}
			if resp.Content == "" {
				t.Error("expected non-empty response")
			}
			// Should contain task description
			if len(resp.Content) < 10 {
				t.Errorf("expected response to contain task info, got '%s'", resp.Content)
			}
		case <-time.After(1 * time.Second):
			t.Error("timeout waiting for response")
		}
	})
}

func TestAgentState_StopResume(t *testing.T) {
	t.Run("request stop while available transitions to stopped", func(t *testing.T) {
		state := NewAgentState()
		state.RequestStop()

		if !state.IsStopped() {
			t.Error("expected state to be stopped after RequestStop while available")
		}
		if state.IsAvailable() {
			t.Error("expected state to not be available after RequestStop")
		}
	})

	t.Run("request stop while working sets stop flag", func(t *testing.T) {
		state := NewAgentState()
		state.StartTask("doing work")
		state.RequestStop()

		if !state.IsWorking() {
			t.Error("expected state to still be working after RequestStop")
		}
		if !state.IsStopRequested() {
			t.Error("expected stop to be requested")
		}
	})

	t.Run("complete task after stop request transitions to stopped", func(t *testing.T) {
		state := NewAgentState()
		state.StartTask("doing work")
		state.RequestStop()
		state.CompleteTask()

		if !state.IsStopped() {
			t.Error("expected state to be stopped after CompleteTask with stop requested")
		}
		if state.IsStopRequested() {
			t.Error("expected stop request to be cleared after transition")
		}
	})

	t.Run("resume from stopped transitions to available", func(t *testing.T) {
		state := NewAgentState()
		state.RequestStop()

		if !state.IsStopped() {
			t.Error("precondition failed: expected stopped state")
		}

		state.Resume()

		if !state.IsAvailable() {
			t.Error("expected state to be available after Resume")
		}
		if state.IsStopped() {
			t.Error("expected state to not be stopped after Resume")
		}
	})

	t.Run("resume from available is no-op", func(t *testing.T) {
		state := NewAgentState()
		state.Resume()

		if !state.IsAvailable() {
			t.Error("expected state to still be available after Resume")
		}
	})

	t.Run("get status shows stopped", func(t *testing.T) {
		state := NewAgentState()
		state.RequestStop()

		status := state.GetStatus()
		if status != "停止中" {
			t.Errorf("expected status '停止中', got '%s'", status)
		}
	})

	t.Run("get status shows stop pending while working", func(t *testing.T) {
		state := NewAgentState()
		state.StartTask("working")
		state.RequestStop()

		status := state.GetStatus()
		if status == "" {
			t.Error("expected non-empty status")
		}
		// Should contain stop pending indicator
		if len(status) < 5 {
			t.Errorf("expected status to contain info, got '%s'", status)
		}
	})
}

func TestWorkerChatWhileStopped(t *testing.T) {
	t.Run("returns stopped message when stopped", func(t *testing.T) {
		w := NewWorker("yuki", nil)
		w.Start()
		defer w.Stop()

		// Stop the worker
		w.Pause()

		// Send chat request
		responseChan := make(chan ChatResponse, 1)
		w.SendChat("@yuki 手伝って", responseChan)

		// Should get stopped response
		select {
		case resp := <-responseChan:
			if resp.Err != nil {
				t.Errorf("unexpected error: %v", resp.Err)
			}
			if resp.Content == "" {
				t.Error("expected non-empty response")
			}
		case <-time.After(1 * time.Second):
			t.Error("timeout waiting for response")
		}
	})
}

func TestPool(t *testing.T) {
	t.Run("add and get worker", func(t *testing.T) {
		pool := NewPool()

		// Create a mock worker (nil runner is fine for this test)
		w := NewWorker("test", nil)
		pool.Add(w)

		got, ok := pool.Get("test")
		if !ok {
			t.Error("expected to find worker 'test'")
		}
		if got.Name() != "test" {
			t.Errorf("expected worker name 'test', got '%s'", got.Name())
		}
	})

	t.Run("get non-existent worker", func(t *testing.T) {
		pool := NewPool()

		_, ok := pool.Get("nonexistent")
		if ok {
			t.Error("expected not to find worker 'nonexistent'")
		}
	})

	t.Run("all returns all worker names", func(t *testing.T) {
		pool := NewPool()
		pool.Add(NewWorker("mei", nil))
		pool.Add(NewWorker("yuki", nil))

		names := pool.All()
		if len(names) != 2 {
			t.Errorf("expected 2 workers, got %d", len(names))
		}
	})

	t.Run("get status", func(t *testing.T) {
		pool := NewPool()
		w1 := NewWorker("mei", nil)
		w2 := NewWorker("yuki", nil)
		pool.Add(w1)
		pool.Add(w2)

		// Set one to working
		w1.state.StartTask("レビュー")

		status := pool.GetStatus()
		if len(status) != 2 {
			t.Errorf("expected 2 status entries, got %d", len(status))
		}
		if status["yuki"] != "待機中" {
			t.Errorf("expected yuki status '待機中', got '%s'", status["yuki"])
		}
		// mei should be working
		if status["mei"] == "待機中" {
			t.Errorf("expected mei status to not be '待機中', got '%s'", status["mei"])
		}
	})

	t.Run("pause and resume individual worker", func(t *testing.T) {
		pool := NewPool()
		w := NewWorker("yuki", nil)
		pool.Add(w)

		// Pause
		ok := pool.Pause("yuki")
		if !ok {
			t.Error("expected Pause to return true")
		}
		if !w.IsStopped() {
			t.Error("expected worker to be stopped after Pause")
		}

		// Resume
		ok = pool.Resume("yuki")
		if !ok {
			t.Error("expected Resume to return true")
		}
		if w.IsStopped() {
			t.Error("expected worker to not be stopped after Resume")
		}
	})

	t.Run("pause and resume non-existent worker returns false", func(t *testing.T) {
		pool := NewPool()

		if pool.Pause("nonexistent") {
			t.Error("expected Pause to return false for non-existent worker")
		}
		if pool.Resume("nonexistent") {
			t.Error("expected Resume to return false for non-existent worker")
		}
	})

	t.Run("pause all and resume all", func(t *testing.T) {
		pool := NewPool()
		w1 := NewWorker("mei", nil)
		w2 := NewWorker("yuki", nil)
		pool.Add(w1)
		pool.Add(w2)

		// Pause all
		pool.PauseAll()
		if !w1.IsStopped() || !w2.IsStopped() {
			t.Error("expected all workers to be stopped after PauseAll")
		}

		// Resume all
		pool.ResumeAll()
		if w1.IsStopped() || w2.IsStopped() {
			t.Error("expected all workers to not be stopped after ResumeAll")
		}
	})

	t.Run("get stopped workers", func(t *testing.T) {
		pool := NewPool()
		w1 := NewWorker("mei", nil)
		w2 := NewWorker("yuki", nil)
		w3 := NewWorker("rin", nil)
		pool.Add(w1)
		pool.Add(w2)
		pool.Add(w3)

		// Stop mei and rin
		w1.Pause()
		w3.Pause()

		stopped := pool.GetStoppedWorkers()
		if len(stopped) != 2 {
			t.Errorf("expected 2 stopped workers, got %d", len(stopped))
		}

		// Check that both mei and rin are in the list
		foundMei, foundRin := false, false
		for _, name := range stopped {
			if name == "mei" {
				foundMei = true
			}
			if name == "rin" {
				foundRin = true
			}
		}
		if !foundMei || !foundRin {
			t.Errorf("expected mei and rin in stopped list, got %v", stopped)
		}
	})
}
