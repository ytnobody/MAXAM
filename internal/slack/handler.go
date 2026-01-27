package slack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/maxam/internal/agent"
	"github.com/anthropics/maxam/internal/comms"
	"github.com/anthropics/maxam/internal/logger"
)

// Handler processes Slack messages with the AI team
type Handler struct {
	client  *Client
	agents  *agent.Agents
	router  *comms.Router
	logMgr  *logger.Manager
}

// NewHandler creates a new Slack message handler
func NewHandler(client *Client, agents *agent.Agents, router *comms.Router, logMgr *logger.Manager) *Handler {
	return &Handler{
		client:  client,
		agents:  agents,
		router:  router,
		logMgr:  logMgr,
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

	// Add thinking reaction
	h.client.AddReaction(channel, msgs[len(msgs)-1].Timestamp, "thinking_face")

	// Log incoming messages
	h.router.Send("slack", "mei", &comms.Message{
		Subject: "Customer inquiry",
		Body:    customerInput,
		Action:  "Analyze and respond",
	})

	// Let Mei handle the conversation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	mei := h.agents.Mei()
	prompt := fmt.Sprintf(`あなたはMAXAMチームのPM、Mei Chenです。
顧客からの問い合わせに対応してください。

%s

以下の形式で回答してください:
1. 顧客への返答（日本語で丁寧に）
2. 必要であれば、チームへの指示（Yukiへの実装依頼など）

顧客への返答は「## 返答」セクションに書いてください。
チームへの指示がある場合は「## チーム指示」セクションに書いてください。`, customerInput)

	start := time.Now()
	result, err := mei.Run(ctx, prompt)
	elapsed := time.Since(start)

	// Log Mei's work
	if log, err := h.logMgr.Get("mei"); err == nil {
		log.LogSimple(customerInput, result, elapsed)
	}

	// Remove thinking reaction, add done
	h.client.AddReaction(channel, msgs[len(msgs)-1].Timestamp, "white_check_mark")

	if err != nil {
		h.client.PostMessageAsAgent(channel, "Mei Chen",
			"申し訳ありません、処理中にエラーが発生しました。少々お待ちください。",
			threadTS)
		return
	}

	// Parse and send response
	response := parseResponse(result)
	if response.CustomerReply != "" {
		h.client.PostMessageAsAgent(channel, "Mei Chen", response.CustomerReply, threadTS)
	}

	// Handle team instructions
	if response.TeamInstruction != "" {
		h.handleTeamInstruction(response.TeamInstruction, channel, threadTS)
	}
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

		// Send to Yuki via comms
		h.router.Send("mei", "yuki", &comms.Message{
			Subject: "Implementation task from customer request",
			Body:    instruction,
			Action:  "Please implement",
		})

		// Notify in Slack
		h.client.PostMessageAsAgent(channel, "Mei Chen",
			"Yukiに実装を依頼しました。完了次第お知らせします。",
			threadTS)
	}
}
