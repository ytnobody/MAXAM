package auto

import (
	"context"
	"log"
	"time"
)

// StubExecutor is a stub implementation of TaskExecutor for testing
// It simulates task execution by logging and returning a mock PR number
type StubExecutor struct {
	// SimulatedDuration is how long each task "takes" to execute
	SimulatedDuration time.Duration
	// NextPRNumber is the next PR number to return (incremented after each call)
	NextPRNumber int
	// FailIssues is a list of issue numbers that should fail
	FailIssues map[int]error
	// Logger for output
	Logger *log.Logger
}

// NewStubExecutor creates a new stub executor
func NewStubExecutor() *StubExecutor {
	return &StubExecutor{
		SimulatedDuration: 100 * time.Millisecond,
		NextPRNumber:      100,
		FailIssues:        make(map[int]error),
		Logger:            log.Default(),
	}
}

// Execute simulates task execution
func (e *StubExecutor) Execute(ctx context.Context, issueNumber int, title string) (int, error) {
	e.Logger.Printf("[stub] Executing task #%d: %s", issueNumber, title)

	// Check if this issue should fail
	if err, ok := e.FailIssues[issueNumber]; ok {
		e.Logger.Printf("[stub] Task #%d failed (simulated): %v", issueNumber, err)
		return 0, err
	}

	// Simulate work
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(e.SimulatedDuration):
	}

	prNumber := e.NextPRNumber
	e.NextPRNumber++

	e.Logger.Printf("[stub] Task #%d completed -> PR #%d", issueNumber, prNumber)
	return prNumber, nil
}

// SetFailure configures an issue to fail with the given error
func (e *StubExecutor) SetFailure(issueNumber int, err error) {
	e.FailIssues[issueNumber] = err
}

// ClearFailures removes all configured failures
func (e *StubExecutor) ClearFailures() {
	e.FailIssues = make(map[int]error)
}

// Ensure StubExecutor implements TaskExecutor
var _ TaskExecutor = (*StubExecutor)(nil)
