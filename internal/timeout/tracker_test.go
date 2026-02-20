package timeout

import (
	"sync"
	"testing"
	"time"
)

func TestTrackerRecordActivity(t *testing.T) {
	tracker := NewTracker(time.Minute, nil)

	ctx := AgentContext{
		IssueNumber: 367,
		TaskSummary: "テスト",
		Progress:    "50%",
		NextAction:  "完了",
	}

	tracker.RecordActivity("yuki-1", ctx)

	activity, exists := tracker.GetActivity("yuki-1")
	if !exists {
		t.Fatal("GetActivity() should return true for recorded agent")
	}
	if activity.Context.IssueNumber != 367 {
		t.Errorf("IssueNumber = %d, want 367", activity.Context.IssueNumber)
	}
	if activity.Context.TaskSummary != "テスト" {
		t.Errorf("TaskSummary = %q, want %q", activity.Context.TaskSummary, "テスト")
	}
}

func TestTrackerRecordHeartbeat(t *testing.T) {
	tracker := NewTracker(time.Minute, nil)

	ctx := AgentContext{IssueNumber: 100}
	tracker.RecordActivity("agent-1", ctx)

	// Get initial timestamp
	activity1, _ := tracker.GetActivity("agent-1")
	time1 := activity1.LastActivity

	// Wait a bit and record heartbeat
	time.Sleep(10 * time.Millisecond)
	tracker.RecordHeartbeat("agent-1")

	activity2, _ := tracker.GetActivity("agent-1")
	if !activity2.LastActivity.After(time1) {
		t.Error("RecordHeartbeat() should update LastActivity")
	}
	// Context should remain unchanged
	if activity2.Context.IssueNumber != 100 {
		t.Error("RecordHeartbeat() should not change context")
	}
}

func TestTrackerUpdateProgress(t *testing.T) {
	tracker := NewTracker(time.Minute, nil)

	ctx := AgentContext{
		IssueNumber: 200,
		Progress:    "10%",
	}
	tracker.RecordActivity("agent-1", ctx)

	// Get initial timestamp
	activity1, _ := tracker.GetActivity("agent-1")
	time1 := activity1.LastActivity

	// Update progress should NOT reset timer
	tracker.UpdateProgress("agent-1", "50%")

	activity2, _ := tracker.GetActivity("agent-1")
	if activity2.Context.Progress != "50%" {
		t.Errorf("Progress = %q, want %q", activity2.Context.Progress, "50%")
	}
	if !activity2.LastActivity.Equal(time1) {
		t.Error("UpdateProgress() should not change LastActivity")
	}
}

func TestTrackerCheckTimeouts(t *testing.T) {
	var callbackCalled bool
	var callbackReport Report
	var mu sync.Mutex

	callback := func(report Report) {
		mu.Lock()
		callbackCalled = true
		callbackReport = report
		mu.Unlock()
	}

	// Use very short timeout for testing
	tracker := NewTracker(50*time.Millisecond, callback)

	ctx := AgentContext{
		IssueNumber: 367,
		TaskSummary: "タイムアウトテスト",
	}
	tracker.RecordActivity("timeout-agent", ctx)

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	timedOut := tracker.CheckTimeouts()

	if len(timedOut) != 1 {
		t.Fatalf("CheckTimeouts() returned %d agents, want 1", len(timedOut))
	}
	if timedOut[0] != "timeout-agent" {
		t.Errorf("Timed out agent = %q, want %q", timedOut[0], "timeout-agent")
	}

	mu.Lock()
	if !callbackCalled {
		t.Error("Callback should have been called")
	}
	if callbackReport.AgentName != "timeout-agent" {
		t.Errorf("Report.AgentName = %q, want %q", callbackReport.AgentName, "timeout-agent")
	}
	if callbackReport.IssueNumber != 367 {
		t.Errorf("Report.IssueNumber = %d, want 367", callbackReport.IssueNumber)
	}
	mu.Unlock()
}

func TestTrackerNoTimeoutForActiveAgent(t *testing.T) {
	callbackCalled := false
	callback := func(report Report) {
		callbackCalled = true
	}

	tracker := NewTracker(time.Minute, callback)
	tracker.RecordActivity("active-agent", AgentContext{})

	timedOut := tracker.CheckTimeouts()

	if len(timedOut) != 0 {
		t.Error("Active agent should not timeout")
	}
	if callbackCalled {
		t.Error("Callback should not be called for active agent")
	}
}

func TestTrackerRemoveAgent(t *testing.T) {
	tracker := NewTracker(time.Minute, nil)
	tracker.RecordActivity("agent-1", AgentContext{})

	tracker.RemoveAgent("agent-1")

	_, exists := tracker.GetActivity("agent-1")
	if exists {
		t.Error("RemoveAgent() should remove the agent")
	}
}

func TestTrackerGetAllActivities(t *testing.T) {
	tracker := NewTracker(time.Minute, nil)
	tracker.RecordActivity("agent-1", AgentContext{IssueNumber: 1})
	tracker.RecordActivity("agent-2", AgentContext{IssueNumber: 2})

	all := tracker.GetAllActivities()

	if len(all) != 2 {
		t.Fatalf("GetAllActivities() returned %d, want 2", len(all))
	}
	if all["agent-1"].Context.IssueNumber != 1 {
		t.Error("Agent-1 should have IssueNumber 1")
	}
	if all["agent-2"].Context.IssueNumber != 2 {
		t.Error("Agent-2 should have IssueNumber 2")
	}
}

func TestTrackerGetTimedOutAgents(t *testing.T) {
	tracker := NewTracker(50*time.Millisecond, nil)

	tracker.RecordActivity("old-agent", AgentContext{})
	time.Sleep(100 * time.Millisecond)
	tracker.RecordActivity("new-agent", AgentContext{})

	timedOut := tracker.GetTimedOutAgents()

	if len(timedOut) != 1 {
		t.Fatalf("GetTimedOutAgents() returned %d agents, want 1", len(timedOut))
	}
	if timedOut[0] != "old-agent" {
		t.Errorf("Timed out agent = %q, want %q", timedOut[0], "old-agent")
	}
}

func TestTrackerDefaultTimeout(t *testing.T) {
	// Test that zero timeout uses default
	tracker := NewTracker(0, nil)

	// This is a bit of a hack to test internal state
	// We can verify by checking timeout behavior
	tracker.RecordActivity("agent", AgentContext{})

	// With default timeout (3 minutes), agent should not timeout immediately
	timedOut := tracker.GetTimedOutAgents()
	if len(timedOut) != 0 {
		t.Error("Agent should not timeout with default timeout")
	}
}
