package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/logger"
)

// ChatSession manages an interactive conversation with an agent
type ChatSession struct {
	agents  *agent.Agents
	logMgr  *logger.Manager
	workDir string
	history []chatMessage
}

type chatMessage struct {
	role    string // "user" or agent name
	content string
}

func runChat() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: maxam chat <agent>")
		fmt.Fprintln(os.Stderr, "Agents: mei, yuki, rin, shiori, priya, amara, team")
		os.Exit(1)
	}

	agentName := strings.ToLower(os.Args[2])
	workDir := getWorkDir()

	session := &ChatSession{
		agents:  agent.NewAgents(workDir),
		logMgr:  logger.NewManager(logger.GetDefaultLogDir(), workDir),
		workDir: workDir,
		history: make([]chatMessage, 0),
	}
	defer session.logMgr.Close()

	if agentName == "team" {
		session.runTeamChat()
	} else {
		session.runAgentChat(agentName)
	}
}

func (s *ChatSession) runAgentChat(agentName string) {
	runner, ok := s.agents.Get(agentName)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown agent: %s\n", agentName)
		os.Exit(1)
	}

	fullName := getFullName(agentName)
	fmt.Printf("Chat with %s\n", fullName)
	fmt.Println("Type 'exit' to quit, 'clear' to reset conversation")
	fmt.Println(strings.Repeat("-", 50))

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Println("Bye!")
			break
		}
		if input == "clear" {
			s.history = make([]chatMessage, 0)
			fmt.Println("(conversation cleared)")
			continue
		}

		s.history = append(s.history, chatMessage{role: "user", content: input})

		// Build prompt with history
		prompt := s.buildPrompt(agentName, input)

		fmt.Printf("\n%s: ", fullName)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		result, err := runner.Run(ctx, prompt)
		cancel()

		if err != nil {
			fmt.Printf("(error: %v)\n", err)
			continue
		}

		result = strings.TrimSpace(result)
		fmt.Println(result)

		s.history = append(s.history, chatMessage{role: agentName, content: result})

		// Log
		if log, err := s.logMgr.Get(agentName); err == nil {
			log.LogSimple(input, result, 0)
		}
	}
}

func (s *ChatSession) runTeamChat() {
	fmt.Println("Chat with MAXAM Team")
	fmt.Println("Mei will coordinate. Mention others: @yuki, @rin, @shiori, @priya, @amara, @alex, @hana")
	fmt.Println("Type 'exit' to quit")
	fmt.Println(strings.Repeat("-", 50))

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Println("Bye!")
			break
		}

		s.history = append(s.history, chatMessage{role: "user", content: input})

		// Detect all mentioned agents (returns array)
		mentionedAgents := detectMentions(input)

		// Call each mentioned agent sequentially
		for _, agentName := range mentionedAgents {
			runner, ok := s.agents.Get(agentName)
			if !ok {
				fmt.Printf("\n%s: (agent not configured)\n", getFullName(agentName))
				continue
			}
			fullName := getFullName(agentName)

			prompt := s.buildPrompt(agentName, input)

			fmt.Printf("\n%s: ", fullName)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			result, err := runner.Run(ctx, prompt)
			cancel()

			if err != nil {
				fmt.Printf("(error: %v)\n", err)
				continue
			}

			result = strings.TrimSpace(result)
			fmt.Println(result)

			s.history = append(s.history, chatMessage{role: agentName, content: result})
		}
	}
}

func (s *ChatSession) buildPrompt(agentName, currentInput string) string {
	var sb strings.Builder

	sb.WriteString("これは対話セッションです。ユーザーと自然に会話してください。\n\n")
	sb.WriteString("重要なルール:\n")
	sb.WriteString("- 情報が不足している場合は、必ず質問してください\n")
	sb.WriteString("- 曖昧な指示には確認を取ってください\n")
	sb.WriteString("- 実行前に計画を説明し、OKをもらってから進めてください\n")
	sb.WriteString("- 短く自然な会話調で返答してください\n\n")

	if len(s.history) > 1 {
		sb.WriteString("## これまでの会話\n\n")
		// Last 10 messages for context
		start := 0
		if len(s.history) > 10 {
			start = len(s.history) - 10
		}
		for i := start; i < len(s.history)-1; i++ {
			msg := s.history[i]
			if msg.role == "user" {
				sb.WriteString(fmt.Sprintf("ユーザー: %s\n\n", msg.content))
			} else {
				sb.WriteString(fmt.Sprintf("%s: %s\n\n", getFullName(msg.role), msg.content))
			}
		}
	}

	sb.WriteString("## 今回のメッセージ\n\n")
	sb.WriteString(fmt.Sprintf("ユーザー: %s\n\n", currentInput))
	sb.WriteString("あなたの返答:")

	return sb.String()
}

func detectMentions(text string) []string {
	lower := strings.ToLower(text)
	var agents []string

	// Check each agent - order doesn't matter for multiple mentions
	if strings.Contains(lower, "@mei") || strings.Contains(lower, "めい") {
		agents = append(agents, "mei")
	}
	if strings.Contains(lower, "@yuki") || strings.Contains(lower, "ゆき") {
		agents = append(agents, "yuki")
	}
	if strings.Contains(lower, "@rin") || strings.Contains(lower, "りん") {
		agents = append(agents, "rin")
	}
	if strings.Contains(lower, "@shiori") || strings.Contains(lower, "しおり") {
		agents = append(agents, "shiori")
	}
	if strings.Contains(lower, "@priya") || strings.Contains(lower, "プリヤ") {
		agents = append(agents, "priya")
	}
	if strings.Contains(lower, "@amara") || strings.Contains(lower, "アマラ") {
		agents = append(agents, "amara")
	}
	if strings.Contains(lower, "@alex") || strings.Contains(lower, "アレックス") {
		agents = append(agents, "alex")
	}
	if strings.Contains(lower, "@hana") || strings.Contains(lower, "ハナ") {
		agents = append(agents, "hana")
	}

	// Default to Mei if no mentions
	if len(agents) == 0 {
		return []string{"mei"}
	}
	return agents
}

func getFullName(agentName string) string {
	switch agentName {
	case "yuki":
		return "Yuki"
	case "rin":
		return "Rin"
	case "shiori":
		return "Shiori"
	case "priya":
		return "Priya"
	case "amara":
		return "Amara"
	case "mei":
		return "Mei"
	case "alex":
		return "Alex"
	case "hana":
		return "Hana"
	default:
		return agentName
	}
}
