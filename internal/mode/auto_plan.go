package mode

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ytnobody/MAXAM/internal/approval"
	"github.com/ytnobody/MAXAM/internal/auto"
	"github.com/ytnobody/MAXAM/internal/config"
)

// AutoPlanHandler manages the automatic plan mode workflow
// It watches for approval on plan issues and transitions to auto mode when approved
type AutoPlanHandler struct {
	mu sync.RWMutex

	modeManager     *Manager
	watcher         *approval.Watcher
	feedbackWatcher *approval.FeedbackWatcher
	poster          *approval.QuestionPoster
	runner          *auto.Runner

	// current state
	planIssueNumber int
	planSummary     string
	state           AutoPlanState

	// callbacks
	onApproved   func(issueNumber int, approvedBy string)
	onFeedback   func(issueNumber int, feedback *approval.FeedbackComment)
	onTransition func(from, to config.Mode)
}

// AutoPlanState represents the state of the auto plan workflow
type AutoPlanState string

const (
	// StateIdle means no active plan workflow
	StateIdle AutoPlanState = "idle"
	// StatePlanCreated means a plan has been created and posted
	StatePlanCreated AutoPlanState = "plan_created"
	// StateWaitingApproval means waiting for owner approval
	StateWaitingApproval AutoPlanState = "waiting_approval"
	// StateApproved means the plan has been approved
	StateApproved AutoPlanState = "approved"
	// StateExecuting means the plan is being executed
	StateExecuting AutoPlanState = "executing"
)

// AutoPlanConfig holds configuration for AutoPlanHandler
type AutoPlanConfig struct {
	PollInterval time.Duration
}

// DefaultAutoPlanConfig returns the default configuration
func DefaultAutoPlanConfig() AutoPlanConfig {
	return AutoPlanConfig{
		PollInterval: 30 * time.Second,
	}
}

// noopExecutor is a no-op TaskExecutor for default handler
type noopExecutor struct{}

func (n *noopExecutor) Execute(ctx context.Context, issueNumber int, title string) (prNumber int, err error) {
	// No-op: actual execution is handled by the caller
	return 0, nil
}

// NewAutoPlanHandler creates a new AutoPlanHandler
func NewAutoPlanHandler(
	modeManager *Manager,
	watcher *approval.Watcher,
	poster *approval.QuestionPoster,
) *AutoPlanHandler {
	// Create feedback watcher using the same client as approval watcher
	var feedbackWatcher *approval.FeedbackWatcher
	if watcher != nil {
		feedbackWatcher = approval.NewFeedbackWatcher(watcher.Client())
	}

	return &AutoPlanHandler{
		modeManager:     modeManager,
		watcher:         watcher,
		feedbackWatcher: feedbackWatcher,
		poster:          poster,
		runner:          auto.NewRunner(&noopExecutor{}, auto.DefaultConfig()),
		state:           StateIdle,
	}
}

// NewAutoPlanHandlerWithRunner creates a new AutoPlanHandler with a custom runner
func NewAutoPlanHandlerWithRunner(
	modeManager *Manager,
	watcher *approval.Watcher,
	poster *approval.QuestionPoster,
	runner *auto.Runner,
) *AutoPlanHandler {
	return &AutoPlanHandler{
		modeManager: modeManager,
		watcher:     watcher,
		poster:      poster,
		runner:      runner,
		state:       StateIdle,
	}
}

// SetOnApproved sets the callback for when a plan is approved
func (h *AutoPlanHandler) SetOnApproved(fn func(issueNumber int, approvedBy string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onApproved = fn
}

// SetOnTransition sets the callback for mode transitions
func (h *AutoPlanHandler) SetOnTransition(fn func(from, to config.Mode)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onTransition = fn
}

// SetOnFeedback sets the callback for when feedback is received
func (h *AutoPlanHandler) SetOnFeedback(fn func(issueNumber int, feedback *approval.FeedbackComment)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onFeedback = fn
}

// State returns the current state
func (h *AutoPlanHandler) State() AutoPlanState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.state
}

// PlanIssueNumber returns the current plan issue number
func (h *AutoPlanHandler) PlanIssueNumber() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.planIssueNumber
}

// Runner returns the auto runner
func (h *AutoPlanHandler) Runner() *auto.Runner {
	return h.runner
}

// StartPlanWorkflow starts the automatic plan workflow
// 1. Posts the plan to the issue
// 2. Starts watching for approval
// 3. When approved, transitions to auto mode
func (h *AutoPlanHandler) StartPlanWorkflow(ctx context.Context, issueNumber int, planSummary, agentName string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state != StateIdle {
		return fmt.Errorf("plan workflow already active (state: %s)", h.state)
	}

	// Enter plan mode
	if err := h.modeManager.EnterPlanMode(); err != nil {
		return fmt.Errorf("enter plan mode: %w", err)
	}

	// Post plan to issue
	if err := h.poster.PostPlanApprovalRequest(ctx, issueNumber, planSummary, agentName); err != nil {
		h.modeManager.Stop() // rollback
		return fmt.Errorf("post plan: %w", err)
	}

	// Start watching for approval
	h.watcher.Watch(issueNumber)

	// Start watching for feedback
	if h.feedbackWatcher != nil {
		h.feedbackWatcher.Watch(issueNumber)
	}

	// Update state
	h.planIssueNumber = issueNumber
	h.planSummary = planSummary
	h.state = StateWaitingApproval

	return nil
}

// CheckApproval checks for approval on the current plan
// Returns true if approved, false otherwise
func (h *AutoPlanHandler) CheckApproval(ctx context.Context) (bool, string, error) {
	h.mu.RLock()
	state := h.state
	issueNumber := h.planIssueNumber
	h.mu.RUnlock()

	if state != StateWaitingApproval {
		return false, "", nil
	}

	events, err := h.watcher.CheckOnce(ctx)
	if err != nil {
		return false, "", err
	}

	for _, event := range events {
		if event.IssueNumber == issueNumber && event.Result.Approved {
			// Handle approval
			h.handleApproval(event.Result.ApprovedBy)
			return true, event.Result.ApprovedBy, nil
		}
	}

	return false, "", nil
}

// handleApproval handles the approval event
func (h *AutoPlanHandler) handleApproval(approvedBy string) {
	h.mu.Lock()

	h.state = StateApproved

	// Mark plan as approved
	h.modeManager.SetPlanApproved(true)

	// Transition to auto mode
	planCtx := h.modeManager.GetPlanContext()
	if planCtx != nil {
		if err := h.modeManager.EnterAutoMode(planCtx); err == nil {
			h.state = StateExecuting
			if h.onTransition != nil {
				h.onTransition(config.ModePlan, config.ModeAuto)
			}
		}
	}

	// Add task to runner
	h.runner.AddTask(h.planIssueNumber, h.planSummary)

	onApproved := h.onApproved
	issueNumber := h.planIssueNumber

	h.mu.Unlock()

	// Start auto runner in background
	go func() {
		if err := h.runner.Run(context.Background()); err != nil {
			log.Printf("Auto runner failed for issue #%d: %v", issueNumber, err)
		}
	}()

	if onApproved != nil {
		onApproved(issueNumber, approvedBy)
	}
}

// Stop stops the plan workflow and returns to interactive mode
func (h *AutoPlanHandler) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.planIssueNumber != 0 {
		h.watcher.Unwatch(h.planIssueNumber)
		if h.feedbackWatcher != nil {
			h.feedbackWatcher.Unwatch(h.planIssueNumber)
		}
	}

	h.modeManager.Stop()
	h.planIssueNumber = 0
	h.planSummary = ""
	h.state = StateIdle
}

// StartPolling starts the polling loop for approval
// Returns a channel that receives the approval result
func (h *AutoPlanHandler) StartPolling(ctx context.Context) <-chan ApprovalNotification {
	notifyCh := make(chan ApprovalNotification, 1)

	go func() {
		defer close(notifyCh)

		eventCh := h.watcher.Start(ctx)
		for event := range eventCh {
			h.mu.RLock()
			issueNumber := h.planIssueNumber
			h.mu.RUnlock()

			if event.IssueNumber == issueNumber && event.Result.Approved {
				h.handleApproval(event.Result.ApprovedBy)

				select {
				case notifyCh <- ApprovalNotification{
					IssueNumber: event.IssueNumber,
					ApprovedBy:  event.Result.ApprovedBy,
				}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return notifyCh
}

// ApprovalNotification represents a notification when a plan is approved
type ApprovalNotification struct {
	IssueNumber int
	ApprovedBy  string
}

// AskOwnerQuestion posts a question to the owner on the current issue
func (h *AutoPlanHandler) AskOwnerQuestion(ctx context.Context, question, agentName string) error {
	h.mu.RLock()
	issueNumber := h.planIssueNumber
	h.mu.RUnlock()

	if issueNumber == 0 {
		return fmt.Errorf("no active plan issue")
	}

	return h.poster.PostClarificationRequest(ctx, issueNumber, question, agentName)
}

// ReportBlocker posts a blocker notification to the owner
func (h *AutoPlanHandler) ReportBlocker(ctx context.Context, blocker, agentName string) error {
	h.mu.RLock()
	issueNumber := h.planIssueNumber
	h.mu.RUnlock()

	if issueNumber == 0 {
		return fmt.Errorf("no active plan issue")
	}

	return h.poster.PostBlockerNotification(ctx, issueNumber, blocker, agentName)
}

// FeedbackNotification represents a notification when feedback is received
type FeedbackNotification struct {
	IssueNumber int
	Feedback    *approval.FeedbackComment
}

// StartFeedbackPolling starts the polling loop for feedback
// Returns a channel that receives feedback events
func (h *AutoPlanHandler) StartFeedbackPolling(ctx context.Context) <-chan FeedbackNotification {
	notifyCh := make(chan FeedbackNotification, 10)

	if h.feedbackWatcher == nil {
		close(notifyCh)
		return notifyCh
	}

	go func() {
		defer close(notifyCh)

		eventCh := h.feedbackWatcher.Start(ctx)
		for event := range eventCh {
			h.mu.RLock()
			issueNumber := h.planIssueNumber
			onFeedback := h.onFeedback
			h.mu.RUnlock()

			if event.IssueNumber != issueNumber {
				continue
			}

			// Handle approval separately - let the approval watcher handle it
			if event.Comment.Type == approval.CommentTypeApproval {
				continue
			}

			// Invoke callback
			if onFeedback != nil {
				onFeedback(event.IssueNumber, event.Comment)
			}

			select {
			case notifyCh <- FeedbackNotification{
				IssueNumber: event.IssueNumber,
				Feedback:    event.Comment,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return notifyCh
}

// CheckFeedback checks for feedback on the current plan once
func (h *AutoPlanHandler) CheckFeedback(ctx context.Context) ([]FeedbackNotification, error) {
	if h.feedbackWatcher == nil {
		return nil, nil
	}

	h.mu.RLock()
	issueNumber := h.planIssueNumber
	h.mu.RUnlock()

	events, err := h.feedbackWatcher.CheckOnce(ctx)
	if err != nil {
		return nil, err
	}

	var notifications []FeedbackNotification
	for _, event := range events {
		if event.IssueNumber != issueNumber {
			continue
		}
		// Skip approval comments
		if event.Comment.Type == approval.CommentTypeApproval {
			continue
		}
		notifications = append(notifications, FeedbackNotification{
			IssueNumber: event.IssueNumber,
			Feedback:    event.Comment,
		})
	}

	return notifications, nil
}

// WatchFeedback starts watching the plan issue for feedback
func (h *AutoPlanHandler) WatchFeedback(issueNumber int) {
	if h.feedbackWatcher != nil {
		h.feedbackWatcher.Watch(issueNumber)
	}
}

// UnwatchFeedback stops watching the plan issue for feedback
func (h *AutoPlanHandler) UnwatchFeedback(issueNumber int) {
	if h.feedbackWatcher != nil {
		h.feedbackWatcher.Unwatch(issueNumber)
	}
}
