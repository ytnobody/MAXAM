// Package router provides an agent router that determines which agent should respond
package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Router routes messages to appropriate agents using LLM
type Router struct {
	agents []AgentInfo
}

// AgentInfo holds basic agent information for routing
type AgentInfo struct {
	Name string
	Role string
}

// RouteResult contains the routing decision
type RouteResult struct {
	Agents []string `json:"agents"`
}

// New creates a new Router with agent information
func New(agents []AgentInfo) *Router {
	return &Router{
		agents: agents,
	}
}

// Message represents a chat message for context
type Message struct {
	Role    string
	Content string
}

// Route determines which agents should respond to the message
// It uses Claude Haiku for fast, cost-effective routing decisions
func (r *Router) Route(ctx context.Context, message string, history []Message) ([]string, error) {
	prompt := r.buildPrompt(message, history)

	// Use haiku for cost-effective routing
	args := []string{
		"--print",
		"--model", "haiku",
		"--max-turns", "1",
		prompt,
	}

	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "claude", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return []string{"mei"}, nil // fallback to Mei on timeout
		}
		return []string{"mei"}, nil // fallback to Mei on error
	}

	result := strings.TrimSpace(stdout.String())
	agents := r.parseResult(result)

	if len(agents) == 0 {
		return []string{"mei"}, nil // fallback to Mei
	}

	return agents, nil
}

func (r *Router) buildPrompt(message string, history []Message) string {
	var sb strings.Builder

	sb.WriteString(`You are a message router. Determine which team member(s) should respond to the message.

## Team Members
`)

	for _, agent := range r.agents {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", agent.Name, agent.Role))
	}

	sb.WriteString(`
## Rules
1. Return ONLY a JSON object with "agents" array containing the agent names (lowercase)
2. Choose 1-3 most appropriate agents based on the message content
3. Consider the conversation context to understand ongoing discussions
4. If unsure, include "mei" as she handles general coordination

## Examples
- "実装お願い" → {"agents": ["yuki"]}
- "レビューお願い" → {"agents": ["priya"]}
- "YukiとPriyaに確認" → {"agents": ["yuki", "priya"]}
- "こんにちは" → {"agents": ["mei"]}
- "分析して" → {"agents": ["amara"]}
- "UIを直して" → {"agents": ["rin"]}
- "テスト書いて" → {"agents": ["shiori"]}

`)

	// Add recent conversation history (max 4 messages)
	if len(history) > 0 {
		sb.WriteString("## Recent Conversation\n")
		start := 0
		if len(history) > 4 {
			start = len(history) - 4
		}
		for i := start; i < len(history); i++ {
			msg := history[i]
			sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, truncate(msg.Content, 200)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Current Message\n")
	sb.WriteString(message)
	sb.WriteString("\n\n## Your Response (JSON only)\n")

	return sb.String()
}

func (r *Router) parseResult(result string) []string {
	// Try to extract JSON from the result
	// The LLM might include extra text, so look for JSON pattern

	// First, try direct parse
	var routeResult RouteResult
	if err := json.Unmarshal([]byte(result), &routeResult); err == nil {
		return r.validateAgents(routeResult.Agents)
	}

	// Try to find JSON in the response
	start := strings.Index(result, "{")
	end := strings.LastIndex(result, "}")
	if start >= 0 && end > start {
		jsonStr := result[start : end+1]
		if err := json.Unmarshal([]byte(jsonStr), &routeResult); err == nil {
			return r.validateAgents(routeResult.Agents)
		}
	}

	// Fallback: look for agent names in the response
	lower := strings.ToLower(result)
	var agents []string
	validNames := r.getValidNames()

	for _, name := range validNames {
		if strings.Contains(lower, name) {
			agents = append(agents, name)
		}
	}

	return agents
}

func (r *Router) validateAgents(agents []string) []string {
	validNames := r.getValidNames()
	validSet := make(map[string]bool)
	for _, name := range validNames {
		validSet[name] = true
	}

	var result []string
	seen := make(map[string]bool)

	for _, agent := range agents {
		name := strings.ToLower(strings.TrimSpace(agent))
		if validSet[name] && !seen[name] {
			result = append(result, name)
			seen[name] = true
		}
	}

	return result
}

func (r *Router) getValidNames() []string {
	names := make([]string, len(r.agents))
	for i, agent := range r.agents {
		names[i] = strings.ToLower(agent.Name)
	}
	return names
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
