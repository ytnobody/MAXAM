package control

import (
	"fmt"
	"strings"

	"github.com/ytnobody/MAXAM/internal/agent/worker"
)

// Handler handles control commands
type Handler struct {
	pool   *worker.Pool
	parser *Parser
}

// NewHandler creates a new control command handler
func NewHandler(pool *worker.Pool) *Handler {
	return &Handler{
		pool:   pool,
		parser: NewParser(),
	}
}

// Result represents the result of a control command
type Result struct {
	Success bool
	Message string
}

// Handle parses and executes a control command
// Returns nil if the input is not a control command
func (h *Handler) Handle(input string) *Result {
	cmd := h.parser.Parse(input)
	if cmd == nil {
		return nil
	}

	switch cmd.Type {
	case CommandStop:
		return h.handleStop(cmd.Target)
	case CommandResume:
		return h.handleResume(cmd.Target)
	case CommandStopAll:
		return h.handleStopAll()
	case CommandResumeAll:
		return h.handleResumeAll()
	default:
		return &Result{Success: false, Message: "不明なコマンドです"}
	}
}

// handleStop handles a stop command for a specific agent
func (h *Handler) handleStop(target string) *Result {
	// Case-insensitive agent name lookup
	targetLower := strings.ToLower(target)
	for _, name := range h.pool.All() {
		if strings.ToLower(name) == targetLower {
			if h.pool.Pause(name) {
				return &Result{
					Success: true,
					Message: fmt.Sprintf("%s を停止しました（作業中の場合は完了後に停止）", name),
				}
			}
		}
	}
	return &Result{
		Success: false,
		Message: fmt.Sprintf("エージェント '%s' が見つかりません", target),
	}
}

// handleResume handles a resume command for a specific agent
func (h *Handler) handleResume(target string) *Result {
	// Case-insensitive agent name lookup
	targetLower := strings.ToLower(target)
	for _, name := range h.pool.All() {
		if strings.ToLower(name) == targetLower {
			if h.pool.Resume(name) {
				return &Result{
					Success: true,
					Message: fmt.Sprintf("%s を再開しました", name),
				}
			}
		}
	}
	return &Result{
		Success: false,
		Message: fmt.Sprintf("エージェント '%s' が見つかりません", target),
	}
}

// handleStopAll handles a stop all command
func (h *Handler) handleStopAll() *Result {
	h.pool.PauseAll()
	return &Result{
		Success: true,
		Message: "全エージェントを停止しました（作業中のエージェントは完了後に停止）",
	}
}

// handleResumeAll handles a resume all command
func (h *Handler) handleResumeAll() *Result {
	h.pool.ResumeAll()
	return &Result{
		Success: true,
		Message: "全エージェントを再開しました",
	}
}

// IsControlCommand checks if the input is a control command
func (h *Handler) IsControlCommand(input string) bool {
	return h.parser.IsControlCommand(input)
}
