// Package cinotify provides CI notification service
package cinotify

import (
	"context"
	"fmt"
	"sync"
	"time"

	ghlib "github.com/ytnobody/MAXAM/internal/github"
)

// Notifier is the interface for sending CI notifications
type Notifier interface {
	NotifyCI(ctx context.Context, message string) error
}

// ServiceConfig holds configuration for the CI notify service
type ServiceConfig struct {
	// PollInterval is the interval between polling
	PollInterval time.Duration
	// Timeout is the maximum time to wait for CI completion
	Timeout time.Duration
}

// DefaultServiceConfig returns default configuration
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		PollInterval: 30 * time.Second,
		Timeout:      24 * time.Hour,
	}
}

// Service monitors CI status and sends notifications
type Service struct {
	watcher  *ghlib.CIWatcher
	notifier Notifier
	mu       sync.RWMutex
	running  bool
	cancel   context.CancelFunc
}

// NewService creates a new CI notification service
func NewService(client *ghlib.Client, notifier Notifier, config ServiceConfig) *Service {
	watcherConfig := ghlib.CIWatcherConfig{
		PollInterval: config.PollInterval,
		Timeout:      config.Timeout,
	}
	watcher := ghlib.NewCIWatcher(client, watcherConfig)

	return &Service{
		watcher:  watcher,
		notifier: notifier,
	}
}

// Start begins the notification service
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("service already running")
	}

	ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	s.mu.Unlock()

	// Start notification loop
	go s.notificationLoop(ctx)

	return nil
}

// Stop stops the notification service
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
}

// IsRunning returns true if the service is running
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// WatchPR starts watching a PR for CI completion
func (s *Service) WatchPR(ctx context.Context, prNumber int) error {
	return s.watcher.WatchPR(ctx, prNumber)
}

// StopWatching stops watching a PR
func (s *Service) StopWatching(prNumber int) {
	s.watcher.StopWatching(prNumber)
}

// WatchingPRs returns the list of PRs being watched
func (s *Service) WatchingPRs() []int {
	return s.watcher.WatchingPRs()
}

// GetCIStatus gets the current CI status for a PR
func (s *Service) GetCIStatus(ctx context.Context, prNumber int) (*ghlib.CIResult, error) {
	return s.watcher.GetCIStatus(ctx, prNumber)
}

func (s *Service) notificationLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case result := <-s.watcher.NotifyCh():
			s.sendNotification(ctx, result)
		}
	}
}

func (s *Service) sendNotification(ctx context.Context, result ghlib.CIResult) {
	message := formatNotification(result)

	if s.notifier != nil {
		// Best effort notification
		_ = s.notifier.NotifyCI(ctx, message)
	}
}

func formatNotification(result ghlib.CIResult) string {
	var status string
	switch result.Status {
	case ghlib.CIStatusSuccess:
		status = "✅ **CI passed**"
	case ghlib.CIStatusFailure:
		status = "❌ **CI failed**"
	case ghlib.CIStatusCancelled:
		status = "⏹️ **CI cancelled/timeout**"
	default:
		status = "⏳ **CI status unknown**"
	}

	msg := fmt.Sprintf("%s\n\nPR #%d: %s\n%d/%d checks passed\n%s",
		status,
		result.PRNumber,
		result.PRTitle,
		result.Passed,
		result.TotalChecks,
		result.PRURL,
	)

	// Add failed check details
	if result.Status == ghlib.CIStatusFailure {
		msg += "\n\n**Failed checks:**"
		for _, detail := range result.Details {
			if detail.Status == ghlib.CIStatusFailure {
				msg += fmt.Sprintf("\n- %s", detail.Name)
			}
		}
	}

	return msg
}
