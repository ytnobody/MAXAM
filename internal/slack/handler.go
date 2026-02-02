package slack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/config"
	"github.com/ytnobody/MAXAM/internal/logger"
	"github.com/ytnobody/MAXAM/internal/member"
)

// Handler processes Slack messages with the AI team
type Handler struct {
	client       *Client
	agents       *agent.Agents
	members      *member.Members
	logMgr       *logger.Manager
	defaultAgent string
}

// NewHandler creates a new Slack message handler
func NewHandler(client *Client, agents *agent.Agents, logMgr *logger.Manager, workDir string) *Handler {
	members := member.NewMembers(workDir)
	cfg, err := config.LoadWithProject(workDir)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	return &Handler{
		client:       client,
		agents:       agents,
		members:      members,
		logMgr:       logMgr,
		defaultAgent: cfg.DefaultAgent,
	}
}

// HandleMessages processes a batch of messages from a customer
func (h *Handler) HandleMessages(msgs []*IncomingMessage) {
	if len(msgs) == 0 {
		return
	}

	// Combine messages from same channel
	byChannel := make(map[string][]*IncomingMessage)
	for _, msg := range msgs {
		byChannel[msg.Channel] = append(byChannel[msg.Channel], msg)
	}

	for channel, channelMsgs := range byChannel {
		h.handleChannelMessages(channel, channelMsgs)
	}
}

func (h *Handler) handleChannelMessages(channel string, msgs []*IncomingMessage) {
	// Get thread TS (use first message's thread or timestamp)
	threadTS := msgs[0].ThreadTS
	if threadTS == "" {
		threadTS = msgs[0].Timestamp
	}

	// Combine message text
	var combined strings.Builder
	combined.WriteString("## Customer Messages\n\n")
	for _, msg := range msgs {
		combined.WriteString(fmt.Sprintf("**%s**: %s\n\n", msg.UserName, msg.Text))
	}
	customerInput := combined.String()

	// Check for agent mentions
	mentionedAgent := h.detectAgentMention(msgs)

	// Add thinking reaction
	h.client.AddReaction(channel, msgs[len(msgs)-1].Timestamp, "thinking_face")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var result string
	var agentName string
	var err error
	start := time.Now()

	if mentionedAgent != "" {
		// Direct message to specific agent
		result, agentName, err = h.handleDirectMention(ctx, mentionedAgent, customerInput)
	} else {
		// Default: Mei handles the conversation
		result, agentName, err = h.handleMeiDefault(ctx, customerInput)
	}

	elapsed := time.Since(start)

	// Log work
	if log, err := h.logMgr.Get(strings.ToLower(strings.Split(agentName, " ")[0])); err == nil {
		log.LogSimple(customerInput, result, elapsed)
	}

	// Remove thinking reaction, add done
	h.client.AddReaction(channel, msgs[len(msgs)-1].Timestamp, "white_check_mark")

	if err != nil {
		h.client.PostMessageAsAgent(channel, agentName,
			"申し訳ありません、処理中にエラーが発生しました。",
			threadTS)
		return
	}

	// Send response
	if mentionedAgent != "" {
		// Direct response from mentioned agent
		h.client.PostMessageAsAgent(channel, agentName, result, threadTS)
	} else {
		// Mei's structured response
		response := parseResponse(result)
		if response.CustomerReply != "" {
			h.client.PostMessageAsAgent(channel, "Mei Chen", response.CustomerReply, threadTS)
		}
		if response.TeamInstruction != "" {
			h.handleTeamInstruction(response.TeamInstruction, channel, threadTS)
		}
	}
}

// detectAgentMention checks if any agent is mentioned in the messages
func (h *Handler) detectAgentMention(msgs []*IncomingMessage) string {
	for _, msg := range msgs {
		text := strings.ToLower(msg.Text)

		// Check all configured agents for @mentions
		for _, m := range h.members.All() {
			name := strings.ToLower(m.Name)
			if strings.Contains(text, "@"+name) {
				return name
			}
		}
	}
	return ""
}

// handleDirectMention handles a message directed at a specific agent
func (h *Handler) handleDirectMention(ctx context.Context, agentName, input string) (string, string, error) {
	runner, ok := h.agents.Get(agentName)
	if !ok {
		// Fallback to default agent
		fallback := h.defaultAgent
		if fallback == "" {
			fallback = "mei" // last resort fallback
		}
		runner, ok = h.agents.Get(fallback)
		if !ok {
			return "", "", fmt.Errorf("agent %s not found", agentName)
		}
		agentName = fallback
	}

	fullName := h.members.GetFullName(agentName)
	result, err := runner.Run(ctx, input)
	return result, fullName, err
}

// handleMeiDefault handles messages through Mei (default behavior)
func (h *Handler) handleMeiDefault(ctx context.Context, customerInput string) (string, string, error) {
	// Use default agent (fallback to "mei" if not configured)
	defaultAgentName := h.defaultAgent
	if defaultAgentName == "" {
		defaultAgentName = "mei"
	}

	defaultAgent, ok := h.agents.Get(defaultAgentName)
	if !ok {
		return "", "", fmt.Errorf("default agent %s not found", defaultAgentName)
	}

	fullName := h.members.GetFullName(defaultAgentName)
	prompt := fmt.Sprintf(`あなたはMAXAMチームのPM、%sです。
顧客からの問い合わせに対応してください。

%s

以下の形式で回答してください:
1. 顧客への返答（日本語で丁寧に）
2. 必要であれば、チームへの指示

顧客への返答は「## 返答」セクションに書いてください。
チームへの指示がある場合は「## チーム指示」セクションに書いてください。`, fullName, customerInput)

	result, err := defaultAgent.Run(ctx, prompt)
	return result, fullName, err
}

type parsedResponse struct {
	CustomerReply   string
	TeamInstruction string
}

func parseResponse(result string) parsedResponse {
	resp := parsedResponse{}

	// Simple parsing for sections
	lines := strings.Split(result, "\n")
	var currentSection string
	var sectionContent strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "## 返答") || strings.HasPrefix(line, "## Response") {
			if currentSection != "" {
				saveSection(&resp, currentSection, sectionContent.String())
			}
			currentSection = "reply"
			sectionContent.Reset()
		} else if strings.HasPrefix(line, "## チーム") || strings.HasPrefix(line, "## Team") {
			if currentSection != "" {
				saveSection(&resp, currentSection, sectionContent.String())
			}
			currentSection = "team"
			sectionContent.Reset()
		} else if currentSection != "" {
			sectionContent.WriteString(line)
			sectionContent.WriteString("\n")
		}
	}

	// Save last section
	if currentSection != "" {
		saveSection(&resp, currentSection, sectionContent.String())
	}

	// If no sections found, use entire result as reply
	if resp.CustomerReply == "" && resp.TeamInstruction == "" {
		resp.CustomerReply = strings.TrimSpace(result)
	}

	return resp
}

func saveSection(resp *parsedResponse, section, content string) {
	content = strings.TrimSpace(content)
	switch section {
	case "reply":
		resp.CustomerReply = content
	case "team":
		resp.TeamInstruction = content
	}
}

func (h *Handler) handleTeamInstruction(instruction string, channel, threadTS string) {
	// Check if it's for Yuki (implementation)
	instructionLower := strings.ToLower(instruction)

	if strings.Contains(instructionLower, "yuki") ||
		strings.Contains(instructionLower, "実装") ||
		strings.Contains(instructionLower, "implement") {

		// Notify in Slack
		h.client.PostMessageAsAgent(channel, "Mei Chen",
			"Yukiに実装を依頼しました。完了次第お知らせします。",
			threadTS)
	}
}
