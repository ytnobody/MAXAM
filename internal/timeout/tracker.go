// Package timeout provides agent timeout detection and reporting.
package timeout

import (
	"sync"
	"time"
)

// DefaultTimeout is the default timeout duration (3 minutes).
const DefaultTimeout = 3 * time.Minute

// AgentContext holds the current context of an agent's work.
type AgentContext struct {
	IssueNumber int
	TaskSummary string
	Progress    string
	NextAction  string
}

// Activity records an agent's activity.
type Activity struct {
	AgentName    string
	LastActivity time.Time
	Context      AgentContext
}

// TimeoutCallback is called when an agent times out.
type TimeoutCallback func(report Report)

// Tracker tracks agent activity and detects timeouts.
type Tracker struct {
	mu         sync.RWMutex
	activities map[string]*Activity
	timeout    time.Duration
	onTimeout  TimeoutCallback
}

// NewTracker creates a new activity tracker.
func NewTracker(timeout time.Duration, onTimeout TimeoutCallback) *Tracker {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Tracker{
		activities: make(map[string]*Activity),
		timeout:    timeout,
		onTimeout:  onTimeout,
	}
}

// RecordActivity records an agent's activity.
func (t *Tracker) RecordActivity(agentName string, ctx AgentContext) {
	t.mu.Lock()
	defer t.mu.Unlock()

	activity, exists := t.activities[agentName]
	if !exists {
		activity = &Activity{AgentName: agentName}
		t.activities[agentName] = activity
	}

	activity.LastActivity = time.Now()
	activity.Context = ctx
}

// UpdateProgress updates only the progress field without resetting the activity timer.
// Use this for incremental progress updates that shouldn't reset the timeout.
func (t *Tracker) UpdateProgress(agentName string, progress string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if activity, exists := t.activities[agentName]; exists {
		activity.Context.Progress = progress
	}
}

// RecordHeartbeat records activity without changing context.
func (t *Tracker) RecordHeartbeat(agentName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if activity, exists := t.activities[agentName]; exists {
		activity.LastActivity = time.Now()
	}
}

// GetActivity returns the current activity for an agent.
func (t *Tracker) GetActivity(agentName string) (Activity, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	activity, exists := t.activities[agentName]
	if !exists {
		return Activity{}, false
	}
	return *activity, true
}

// CheckTimeouts checks all agents for timeouts and calls the callback.
// Returns the list of agents that timed out.
func (t *Tracker) CheckTimeouts() []string {
	t.mu.RLock()
	now := time.Now()
	var timedOut []string

	for name, activity := range t.activities {
		if now.Sub(activity.LastActivity) > t.timeout {
			timedOut = append(timedOut, name)
		}
	}
	t.mu.RUnlock()

	// Call callbacks outside lock
	for _, name := range timedOut {
		t.mu.RLock()
		activity := t.activities[name]
		t.mu.RUnlock()

		if activity != nil && t.onTimeout != nil {
			report := Report{
				AgentName:    name,
				IssueNumber:  activity.Context.IssueNumber,
				TaskSummary:  activity.Context.TaskSummary,
				Progress:     activity.Context.Progress,
				NextAction:   activity.Context.NextAction,
				LastActivity: activity.LastActivity,
				TimeoutAt:    now,
			}
			t.onTimeout(report)
		}
	}

	return timedOut
}

// RemoveAgent removes an agent from tracking.
func (t *Tracker) RemoveAgent(agentName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.activities, agentName)
}

// GetAllActivities returns all tracked activities.
func (t *Tracker) GetAllActivities() map[string]Activity {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]Activity, len(t.activities))
	for name, activity := range t.activities {
		result[name] = *activity
	}
	return result
}

// GetTimedOutAgents returns agents that have exceeded the timeout threshold.
func (t *Tracker) GetTimedOutAgents() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()
	var timedOut []string

	for name, activity := range t.activities {
		if now.Sub(activity.LastActivity) > t.timeout {
			timedOut = append(timedOut, name)
		}
	}

	return timedOut
}
