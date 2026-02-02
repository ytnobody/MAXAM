package approval

import (
	"testing"
	"time"
)

func TestRequest(t *testing.T) {
	m := NewManager()

	approval := m.Request("pr-123", "PR #123 マージ確認", "Priya")

	if approval.ID != "pr-123" {
		t.Errorf("expected ID 'pr-123', got %s", approval.ID)
	}
	if approval.Context != "PR #123 マージ確認" {
		t.Errorf("unexpected context: %s", approval.Context)
	}
	if approval.Requester != "Priya" {
		t.Errorf("expected requester 'Priya', got %s", approval.Requester)
	}
	if approval.Status != StatusPending {
		t.Errorf("expected status 'pending', got %s", approval.Status)
	}
}

func TestCancel(t *testing.T) {
	m := NewManager()

	m.Request("pr-123", "PR #123 マージ確認", "Priya")

	// Cancel existing
	if !m.Cancel("pr-123") {
		t.Error("expected Cancel to return true")
	}

	// Verify removed
	if _, ok := m.Get("pr-123"); ok {
		t.Error("expected approval to be removed after cancel")
	}

	// Cancel non-existing
	if m.Cancel("pr-999") {
		t.Error("expected Cancel to return false for non-existing")
	}
}

func TestApprove(t *testing.T) {
	m := NewManager()

	m.Request("pr-123", "PR #123 マージ確認", "Priya")

	if !m.Approve("pr-123") {
		t.Error("expected Approve to return true")
	}

	if _, ok := m.Get("pr-123"); ok {
		t.Error("expected approval to be removed after approve")
	}

	if m.Approve("pr-999") {
		t.Error("expected Approve to return false for non-existing")
	}
}

func TestCheckTimeout(t *testing.T) {
	m := NewManager()
	m.SetTimeout(50 * time.Millisecond)

	m.Request("pr-123", "PR #123 マージ確認", "Priya")

	// Not timed out yet
	if m.CheckTimeout("pr-123") {
		t.Error("expected not timed out immediately")
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	if !m.CheckTimeout("pr-123") {
		t.Error("expected timed out after waiting")
	}

	// Non-existing
	if m.CheckTimeout("pr-999") {
		t.Error("expected false for non-existing")
	}
}

func TestCheckAll(t *testing.T) {
	m := NewManager()
	m.SetTimeout(50 * time.Millisecond)

	var expiredCalled int
	m.SetOnExpire(func(a *PendingApproval) {
		expiredCalled++
	})

	m.Request("pr-1", "PR #1", "Priya")
	m.Request("pr-2", "PR #2", "Yuki")

	// Not timed out yet
	timedOut := m.CheckAll()
	if len(timedOut) != 0 {
		t.Errorf("expected 0 timed out, got %d", len(timedOut))
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	timedOut = m.CheckAll()
	if len(timedOut) != 2 {
		t.Errorf("expected 2 timed out, got %d", len(timedOut))
	}
	if expiredCalled != 2 {
		t.Errorf("expected onExpire called 2 times, got %d", expiredCalled)
	}

	// Verify all removed
	if len(m.Pending()) != 0 {
		t.Error("expected all pending removed")
	}
}

func TestRemainingTime(t *testing.T) {
	m := NewManager()
	m.SetTimeout(100 * time.Millisecond)

	m.Request("pr-123", "PR #123", "Priya")

	remaining := m.RemainingTime("pr-123")
	if remaining <= 0 || remaining > 100*time.Millisecond {
		t.Errorf("unexpected remaining time: %v", remaining)
	}

	// Wait a bit
	time.Sleep(30 * time.Millisecond)

	remaining2 := m.RemainingTime("pr-123")
	if remaining2 >= remaining {
		t.Errorf("expected remaining time to decrease: %v >= %v", remaining2, remaining)
	}

	// Non-existing
	if m.RemainingTime("pr-999") != 0 {
		t.Error("expected 0 for non-existing")
	}
}

func TestGenerateTimeoutMessage(t *testing.T) {
	approval := &PendingApproval{
		ID:        "pr-123",
		Context:   "PR #123 マージ確認",
		Requester: "Priya",
	}

	msg := GenerateTimeoutMessage(approval, "Mei")

	expected := "@Mei オーナー確認待ちがタイムアウトしました。\n" +
		"- ID: pr-123\n" +
		"- 依頼者: Priya\n" +
		"- 内容: PR #123 マージ確認\n" +
		"フォローアップが必要です。"

	if msg != expected {
		t.Errorf("unexpected message:\ngot: %s\nwant: %s", msg, expected)
	}
}
