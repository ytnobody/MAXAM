// Package router provides an agent router that determines which agent should respond
package router

import (
	"regexp"
	"strings"
)

// Router routes messages to appropriate agents based on mentions
type Router struct {
	agents       []AgentInfo
	defaultAgent string
	mentionRegex *regexp.Regexp
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
func New(agents []AgentInfo, defaultAgent string) *Router {
	// Build regex pattern for @mentions
	// Matches @name (case insensitive)
	return &Router{
		agents:       agents,
		defaultAgent: defaultAgent,
		mentionRegex: regexp.MustCompile(`(?i)@(\w+)`),
	}
}

// Message represents a chat message for context
type Message struct {
	Role    string
	Content string
}

// Route determines which agents should respond to the message
// It extracts @mentions from the message and routes to those agents
// If no valid mentions found, routes to the default agent
func (r *Router) Route(message string) []string {
	mentions := r.extractMentions(message)

	if len(mentions) > 0 {
		return mentions
	}

	// No valid mentions - use default agent
	if r.defaultAgent != "" {
		return []string{r.defaultAgent}
	}

	// Fallback to first configured agent if no default
	if len(r.agents) > 0 {
		return []string{strings.ToLower(r.agents[0].Name)}
	}

	return nil
}

// extractMentions finds all @mentions in the message and returns valid agent names
func (r *Router) extractMentions(message string) []string {
	matches := r.mentionRegex.FindAllStringSubmatch(message, -1)
	if len(matches) == 0 {
		return nil
	}

	validNames := r.getValidNames()
	validSet := make(map[string]bool)
	for _, name := range validNames {
		validSet[name] = true
	}

	var agents []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.ToLower(match[1])
		if validSet[name] && !seen[name] {
			agents = append(agents, name)
			seen[name] = true
		}
	}

	return agents
}

func (r *Router) getValidNames() []string {
	names := make([]string, len(r.agents))
	for i, agent := range r.agents {
		names[i] = strings.ToLower(agent.Name)
	}
	return names
}
