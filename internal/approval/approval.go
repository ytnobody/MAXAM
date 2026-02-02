// Package approval manages pending approval requests with timeout
package approval

import (
	"sync"
	"time"
)

// DefaultTimeout is the default timeout for approval requests
const DefaultTimeout = 3 * time.Minute

// Status represents the status of an approval request
type Status string

const (
	StatusPending   Status = "pending"
	StatusApproved  Status = "approved"
	StatusCancelled Status = "cancelled"
	StatusTimedOut  Status = "timedout"
)

// PendingApproval represents a pending approval request
type PendingApproval struct {
	ID          string
	RequestedAt time.Time
	Context     string // What is being requested
	Requester   string // Who requested it
	Status      Status
}

// Manager manages pending approval requests
type Manager struct {
	mu       sync.RWMutex
	pending  map[string]*PendingApproval
	timeout  time.Duration
	onExpire func(approval *PendingApproval) // Callback when approval expires
}

// NewManager creates a new approval manager
func NewManager() *Manager {
	return &Manager{
		pending: make(map[string]*PendingApproval),
		timeout: DefaultTimeout,
	}
}

// SetTimeout sets the timeout duration
func (m *Manager) SetTimeout(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeout = d
}

// SetOnExpire sets the callback for when approvals expire
func (m *Manager) SetOnExpire(fn func(*PendingApproval)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onExpire = fn
}

// Request creates a new pending approval request
func (m *Manager) Request(id, context, requester string) *PendingApproval {
	m.mu.Lock()
	defer m.mu.Unlock()

	approval := &PendingApproval{
		ID:          id,
		RequestedAt: time.Now(),
		Context:     context,
		Requester:   requester,
		Status:      StatusPending,
	}

	m.pending[id] = approval
	return approval
}

// Cancel cancels a pending approval (owner responded)
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	approval, ok := m.pending[id]
	if !ok {
		return false
	}

	approval.Status = StatusCancelled
	delete(m.pending, id)
	return true
}

// Approve marks an approval as approved
func (m *Manager) Approve(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	approval, ok := m.pending[id]
	if !ok {
		return false
	}

	approval.Status = StatusApproved
	delete(m.pending, id)
	return true
}

// Get returns a pending approval by ID
func (m *Manager) Get(id string) (*PendingApproval, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	approval, ok := m.pending[id]
	if !ok {
		return nil, false
	}
	return approval, true
}

// CheckTimeout checks if an approval has timed out
func (m *Manager) CheckTimeout(id string) bool {
	m.mu.RLock()
	approval, ok := m.pending[id]
	timeout := m.timeout
	m.mu.RUnlock()

	if !ok {
		return false
	}

	return time.Since(approval.RequestedAt) >= timeout
}

// CheckAll checks all pending approvals for timeout and handles them
// Returns list of timed out approvals
func (m *Manager) CheckAll() []*PendingApproval {
	m.mu.Lock()
	defer m.mu.Unlock()

	var timedOut []*PendingApproval

	for id, approval := range m.pending {
		if time.Since(approval.RequestedAt) >= m.timeout {
			approval.Status = StatusTimedOut
			timedOut = append(timedOut, approval)
			delete(m.pending, id)

			if m.onExpire != nil {
				m.onExpire(approval)
			}
		}
	}

	return timedOut
}

// Pending returns all pending approvals
func (m *Manager) Pending() []*PendingApproval {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*PendingApproval
	for _, approval := range m.pending {
		result = append(result, approval)
	}
	return result
}

// RemainingTime returns the remaining time for a pending approval
func (m *Manager) RemainingTime(id string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	approval, ok := m.pending[id]
	if !ok {
		return 0
	}

	elapsed := time.Since(approval.RequestedAt)
	remaining := m.timeout - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GenerateTimeoutMessage generates a notification message for PM
func GenerateTimeoutMessage(approval *PendingApproval, pmName string) string {
	return "@" + pmName + " オーナー確認待ちがタイムアウトしました。\n" +
		"- ID: " + approval.ID + "\n" +
		"- 依頼者: " + approval.Requester + "\n" +
		"- 内容: " + approval.Context + "\n" +
		"フォローアップが必要です。"
}
