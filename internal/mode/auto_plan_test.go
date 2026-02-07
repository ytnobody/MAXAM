package mode

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/ytnobody/MAXAM/internal/approval"
	"github.com/ytnobody/MAXAM/internal/auto"
	"github.com/ytnobody/MAXAM/internal/config"
)

// mockClient implements both IssueCommentFetcher and IssueCommenter
type mockClient struct {
	mu           sync.Mutex
	comments     map[int][]*github.IssueComment
	postedBodies []string
	postedIssues []int
}

func newMockClient() *mockClient {
	return &mockClient{
		comments:     make(map[int][]*github.IssueComment),
		postedBodies: []string{},
		postedIssues: []int{},
	}
}

func (m *mockClient) ListIssueCommentsSince(ctx context.Context, number int, since time.Time) ([]*github.IssueComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	allComments := m.comments[number]
	var filtered []*github.IssueComment
	for _, c := range allComments {
		if c.CreatedAt != nil && !c.CreatedAt.Time.Before(since) {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

func (m *mockClient) CommentIssue(ctx context.Context, number int, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.postedBodies = append(m.postedBodies, body)
	m.postedIssues = append(m.postedIssues, number)
	return nil
}

func (m *mockClient) SetComments(issueNumber int, comments []*github.IssueComment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.comments[issueNumber] = comments
}

func (m *mockClient) GetPostedComments() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.postedBodies...)
}

func ptrStr(s string) *string {
	return &s
}

func TestAutoPlanHandler_StartPlanWorkflow(t *testing.T) {
	client := newMockClient()
	modeManager := NewManager(config.ModeInteractive)
	watcher := approval.NewWatcher(client)
	poster := approval.NewQuestionPoster(client)

	handler := NewAutoPlanHandler(modeManager, watcher, poster)

	// Start workflow
	err := handler.StartPlanWorkflow(context.Background(), 123, "Test plan summary", "TestAgent")
	if err != nil {
		t.Fatalf("StartPlanWorkflow() error = %v", err)
	}

	// Check state
	if handler.State() != StateWaitingApproval {
		t.Errorf("State() = %v, want %v", handler.State(), StateWaitingApproval)
	}

	// Check mode
	if !modeManager.IsPlan() {
		t.Error("Expected mode to be Plan")
	}

	// Check that comment was posted
	comments := client.GetPostedComments()
	if len(comments) != 1 {
		t.Fatalf("Expected 1 posted comment, got %d", len(comments))
	}

	// Check issue number
	if handler.PlanIssueNumber() != 123 {
		t.Errorf("PlanIssueNumber() = %d, want 123", handler.PlanIssueNumber())
	}
}

func TestAutoPlanHandler_CheckApproval(t *testing.T) {
	client := newMockClient()
	modeManager := NewManager(config.ModeInteractive)
	watcher := approval.NewWatcher(client)
	poster := approval.NewQuestionPoster(client)

	handler := NewAutoPlanHandler(modeManager, watcher, poster)

	// Start workflow
	err := handler.StartPlanWorkflow(context.Background(), 123, "Test plan", "TestAgent")
	if err != nil {
		t.Fatalf("StartPlanWorkflow() error = %v", err)
	}

	// No approval yet
	approved, _, err := handler.CheckApproval(context.Background())
	if err != nil {
		t.Fatalf("CheckApproval() error = %v", err)
	}
	if approved {
		t.Error("Expected not approved yet")
	}

	// Add approval comment
	client.SetComments(123, []*github.IssueComment{
		{
			Body:      ptrStr("/approve"),
			CreatedAt: &github.Timestamp{Time: time.Now().Add(1 * time.Second)},
			User:      &github.User{Login: ptrStr("owner")},
		},
	})

	// Now check again
	approved, approvedBy, err := handler.CheckApproval(context.Background())
	if err != nil {
		t.Fatalf("CheckApproval() error = %v", err)
	}
	if !approved {
		t.Error("Expected approved")
	}
	if approvedBy != "owner" {
		t.Errorf("approvedBy = %q, want %q", approvedBy, "owner")
	}

	// State should be executing
	if handler.State() != StateExecuting {
		t.Errorf("State() = %v, want %v", handler.State(), StateExecuting)
	}

	// Mode should be auto
	if !modeManager.IsAuto() {
		t.Error("Expected mode to be Auto")
	}
}

func TestAutoPlanHandler_Stop(t *testing.T) {
	client := newMockClient()
	modeManager := NewManager(config.ModeInteractive)
	watcher := approval.NewWatcher(client)
	poster := approval.NewQuestionPoster(client)

	handler := NewAutoPlanHandler(modeManager, watcher, poster)

	// Start workflow
	_ = handler.StartPlanWorkflow(context.Background(), 123, "Test plan", "TestAgent")

	// Stop
	handler.Stop()

	// Check state
	if handler.State() != StateIdle {
		t.Errorf("State() = %v, want %v", handler.State(), StateIdle)
	}

	// Check mode
	if !modeManager.IsInteractive() {
		t.Error("Expected mode to be Interactive")
	}

	// Check issue number
	if handler.PlanIssueNumber() != 0 {
		t.Errorf("PlanIssueNumber() = %d, want 0", handler.PlanIssueNumber())
	}
}

func TestAutoPlanHandler_AskOwnerQuestion(t *testing.T) {
	client := newMockClient()
	modeManager := NewManager(config.ModeInteractive)
	watcher := approval.NewWatcher(client)
	poster := approval.NewQuestionPoster(client)

	handler := NewAutoPlanHandler(modeManager, watcher, poster)

	// Without active plan, should fail
	err := handler.AskOwnerQuestion(context.Background(), "Test question?", "TestAgent")
	if err == nil {
		t.Error("Expected error when no active plan")
	}

	// Start workflow
	_ = handler.StartPlanWorkflow(context.Background(), 123, "Test plan", "TestAgent")

	// Now should work
	err = handler.AskOwnerQuestion(context.Background(), "Test question?", "TestAgent")
	if err != nil {
		t.Fatalf("AskOwnerQuestion() error = %v", err)
	}

	// Check that 2 comments were posted (plan + question)
	comments := client.GetPostedComments()
	if len(comments) != 2 {
		t.Errorf("Expected 2 posted comments, got %d", len(comments))
	}
}

func TestAutoPlanHandler_OnApprovedCallback(t *testing.T) {
	client := newMockClient()
	modeManager := NewManager(config.ModeInteractive)
	watcher := approval.NewWatcher(client)
	poster := approval.NewQuestionPoster(client)

	handler := NewAutoPlanHandler(modeManager, watcher, poster)

	// Set callback
	var callbackIssue int
	var callbackApprover string
	handler.SetOnApproved(func(issueNumber int, approvedBy string) {
		callbackIssue = issueNumber
		callbackApprover = approvedBy
	})

	// Start workflow
	_ = handler.StartPlanWorkflow(context.Background(), 123, "Test plan", "TestAgent")

	// Add approval
	client.SetComments(123, []*github.IssueComment{
		{
			Body:      ptrStr("LGTM"),
			CreatedAt: &github.Timestamp{Time: time.Now().Add(1 * time.Second)},
			User:      &github.User{Login: ptrStr("approver")},
		},
	})

	// Check approval
	_, _, _ = handler.CheckApproval(context.Background())

	// Callback should have been called
	if callbackIssue != 123 {
		t.Errorf("Callback issue = %d, want 123", callbackIssue)
	}
	if callbackApprover != "approver" {
		t.Errorf("Callback approver = %q, want %q", callbackApprover, "approver")
	}
}

func TestAutoPlanHandler_DoubleStart(t *testing.T) {
	client := newMockClient()
	modeManager := NewManager(config.ModeInteractive)
	watcher := approval.NewWatcher(client)
	poster := approval.NewQuestionPoster(client)

	handler := NewAutoPlanHandler(modeManager, watcher, poster)

	// First start
	err := handler.StartPlanWorkflow(context.Background(), 123, "Plan 1", "Agent1")
	if err != nil {
		t.Fatalf("First StartPlanWorkflow() error = %v", err)
	}

	// Second start should fail
	err = handler.StartPlanWorkflow(context.Background(), 456, "Plan 2", "Agent2")
	if err == nil {
		t.Error("Expected error on second StartPlanWorkflow()")
	}
}

// mockExecutor is a mock TaskExecutor for testing
type mockExecutor struct {
	executed            atomic.Bool
	executedIssueNumber int
	mu                  sync.Mutex
}

func (m *mockExecutor) Execute(ctx context.Context, issueNumber int, title string) (prNumber int, err error) {
	m.executed.Store(true)
	m.mu.Lock()
	m.executedIssueNumber = issueNumber
	m.mu.Unlock()
	return 0, nil
}

func TestAutoPlanHandler_RunnerIntegration(t *testing.T) {
	client := newMockClient()
	modeManager := NewManager(config.ModeInteractive)
	watcher := approval.NewWatcher(client)
	poster := approval.NewQuestionPoster(client)

	// Create runner with custom executor
	executor := &mockExecutor{}
	runner := auto.NewRunner(executor, auto.DefaultConfig())

	handler := NewAutoPlanHandlerWithRunner(modeManager, watcher, poster, runner)

	// Start workflow
	err := handler.StartPlanWorkflow(context.Background(), 42, "Test plan", "TestAgent")
	if err != nil {
		t.Fatalf("StartPlanWorkflow() error = %v", err)
	}

	// Add approval comment
	client.SetComments(42, []*github.IssueComment{
		{
			Body:      ptrStr("/approve"),
			CreatedAt: &github.Timestamp{Time: time.Now().Add(1 * time.Second)},
			User:      &github.User{Login: ptrStr("owner")},
		},
	})

	// Check approval (triggers runner)
	approved, _, err := handler.CheckApproval(context.Background())
	if err != nil {
		t.Fatalf("CheckApproval() error = %v", err)
	}
	if !approved {
		t.Error("Expected approved")
	}

	// Wait for runner to execute (async)
	time.Sleep(100 * time.Millisecond)

	// Verify runner was called
	if !executor.executed.Load() {
		t.Error("Expected runner to be executed")
	}
	executor.mu.Lock()
	executedIssueNumber := executor.executedIssueNumber
	executor.mu.Unlock()
	if executedIssueNumber != 42 {
		t.Errorf("Executed issue number = %d, want 42", executedIssueNumber)
	}
}

func TestAutoPlanHandler_RunnerAccessor(t *testing.T) {
	client := newMockClient()
	modeManager := NewManager(config.ModeInteractive)
	watcher := approval.NewWatcher(client)
	poster := approval.NewQuestionPoster(client)

	handler := NewAutoPlanHandler(modeManager, watcher, poster)

	// Runner should not be nil
	if handler.Runner() == nil {
		t.Error("Expected runner to be non-nil")
	}

	// Runner should be in idle state
	if handler.Runner().State() != auto.RunnerStateIdle {
		t.Errorf("Runner state = %v, want %v", handler.Runner().State(), auto.RunnerStateIdle)
	}
}
