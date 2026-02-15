package healthmgr

import "time"

// Health represents the health status of an agent.
type Health struct {
	AgentName         string
	Status            HealthStatus
	LastCheckedAt     time.Time
	LastResponseAt    time.Time
	FailCount         int
	LastError         string
	TaskDurationSince time.Time
}

// HealthStatus represents the health state of an agent.
type HealthStatus string

const (
	HealthStatusHealthy      HealthStatus = "healthy"
	HealthStatusUnresponsive HealthStatus = "unresponsive"
	HealthStatusDead         HealthStatus = "dead"
)

// RecoveryResult represents the outcome of a recovery attempt.
type RecoveryResult struct {
	AgentName        string
	Success          bool
	Attempt          int
	LastError        string
	TimeSinceFailure time.Duration
}
